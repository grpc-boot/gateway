package gateway

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrPathNotExists = errors.New(`path not exists`)
	ErrTimeout       = errors.New(`input timeout`)
)

type Gateway interface {
	In(path string) (status uint8, err error)
	InTimeout(timeout time.Duration, path string) (status uint8, err error)
	Out(accessTime time.Time, path string, code int) (dur time.Duration, qps int32, total uint64, err error)
	Info() Info
}

type Info struct {
	Qps        int32        `json:"qps"`
	Total      uint64       `json:"total"`
	MethodList []MethodInfo `json:"method_list"`
}

type gateway struct {
	mutex      sync.RWMutex
	methodList map[string]*method
	qps        int32
	total      uint64
	lastTotal  uint64
	lastSecond int64
}

func NewGateway(options ...Option) Gateway {
	g := &gateway{
		methodList: make(map[string]*method, len(options)),
	}

	for _, option := range options {
		g.methodList[option.Path] = newMethod(option)
	}

	return g
}

func (g *gateway) LoadOptions(options ...Option) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	for _, option := range options {
		if m, exists := g.methodList[option.Path]; exists {
			updateMethod(m, option)
		} else {
			g.methodList[option.Path] = newMethod(option)
		}
	}
}

func (g *gateway) In(path string) (status uint8, err error) {
	g.mutex.Lock()

	var (
		m, exists = g.methodList[path]
	)

	g.mutex.Unlock()

	//路径不存在
	if !exists {
		return StatusNo, ErrPathNotExists
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	//降级
	if m.secondLimit == -1 {
		return StatusNo, nil
	}

	//不限速
	if m.secondLimit == 0 {
		return StatusYes, nil
	}

	//漏斗
	m.limiter.Take()

	return StatusYes, nil
}

func (g *gateway) InTimeout(timeout time.Duration, path string) (status uint8, err error) {
	g.mutex.Lock()

	var (
		m, exists = g.methodList[path]
	)

	g.mutex.Unlock()

	//路径不存在
	if !exists {
		return StatusNo, ErrPathNotExists
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	//降级
	if m.secondLimit == -1 {
		return StatusNo, nil
	}

	//不限速
	if m.secondLimit == 0 {
		return StatusYes, nil
	}

	var (
		w                  = make(chan time.Time, 1)
		timeoutCtx, cancel = context.WithTimeout(context.Background(), timeout)
	)
	defer cancel()

	go func() {
		w <- m.limiter.Take()
	}()

	select {
	case <-w:
		return StatusYes, nil
	case <-timeoutCtx.Done():
		return StatusBusy, ErrTimeout
	}
}

func (g *gateway) Out(accessTime time.Time, path string, code int) (dur time.Duration, qps int32, total uint64, err error) {
	g.mutex.Lock()

	g.total++

	var (
		current     = time.Now()
		currentUnix = current.Unix()
	)

	if currentUnix-g.lastSecond > 0 {
		g.qps = int32(g.total - g.lastTotal)
		g.lastSecond = current.Unix()
		g.lastTotal = g.total
	}
	m, exists := g.methodList[path]

	g.mutex.Unlock()

	if !exists {
		return 0, 0, 0, ErrPathNotExists
	}

	dur = current.Sub(accessTime)

	m.mutex.Lock()

	m.total++

	//状态码自增
	codeCount, _ := m.codeMap[code]
	m.codeMap[code] = codeCount + 1

	//只保留最近100次
	m.latency = append(m.latency, dur)
	if len(m.latency) > 99 {
		m.latency = m.latency[1:]
	}

	if currentUnix-m.lastSecond > 0 {
		m.qps = int32(m.total - m.lastTotal)
		m.lastSecond = current.Unix()
		m.lastTotal = m.total
	}

	qps = m.qps
	total = m.total

	m.mutex.Unlock()

	return dur, qps, total, nil
}

func (g *gateway) Info() Info {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	info := Info{
		Qps:        g.qps,
		Total:      g.total,
		MethodList: make([]MethodInfo, 0, len(g.methodList)),
	}

	for _, m := range g.methodList {
		m.mutex.Lock()

		mi := MethodInfo{
			Name:        m.name,
			Path:        m.path,
			SecondLimit: m.secondLimit,
			Qps:         m.qps,
			Total:       m.total,
			Latency:     m.latency[0:],
		}

		mi.CodeMap = make(map[int]uint64, len(m.codeMap))
		for code, count := range m.codeMap {
			mi.CodeMap[code] = count
		}

		info.MethodList = append(info.MethodList, mi)

		m.mutex.Unlock()
	}

	return info
}
