package gateway

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrTimeout = errors.New(`input timeout`)
)

type Gateway interface {
	In(path string) (status uint8, exists bool)
	InTimeout(timeout time.Duration, path string) (status uint8, exists bool, err error)
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

func (g *gateway) In(path string) (status uint8, exists bool) {
	g.mutex.Lock()

	var m *method
	m, exists = g.methodList[path]

	g.mutex.Unlock()

	//路径不存在
	if !exists {
		return StatusYes, exists
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	//降级
	if m.secondLimit == -1 {
		return StatusNo, exists
	}

	//不限速
	if m.secondLimit == 0 {
		return StatusYes, exists
	}

	//漏斗
	m.limiter.Take()

	return StatusYes, exists
}

func (g *gateway) InTimeout(timeout time.Duration, path string) (status uint8, exists bool, err error) {
	g.mutex.Lock()

	var m *method
	m, exists = g.methodList[path]

	g.mutex.Unlock()

	//路径不存在
	if !exists {
		return StatusYes, exists, nil
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	//降级
	if m.secondLimit == -1 {
		return StatusNo, exists, nil
	}

	//不限速
	if m.secondLimit == 0 {
		return StatusYes, exists, nil
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
		return StatusYes, exists, nil
	case <-timeoutCtx.Done():
		return StatusBusy, exists, ErrTimeout
	}
}

func (g *gateway) Out(accessTime time.Time, path string, code int) (dur time.Duration, qps int32, total uint64, err error) {
	g.mutex.Lock()
	m, exists := g.methodList[path]

	//路径不存在
	if !exists {
		g.mutex.Unlock()
		return
	}

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

	g.mutex.Unlock()

	if !exists {
		return 0, 0, 0, nil
	}

	dur = current.Sub(accessTime)

	m.mutex.Lock()

	m.total++

	//状态码自增
	codeCount, _ := m.codeMap[code]
	m.codeMap[code] = codeCount + 1

	//只保留最近100次
	m.latency = append(m.latency, dur)
	if len(m.latency) >= sampleMax {
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

	info := Info{
		Qps:        g.qps,
		Total:      g.total,
		MethodList: make([]MethodInfo, 0, len(g.methodList)),
	}

	for _, m := range g.methodList {
		m.mutex.Lock()

		dst := make(LatencyList, len(m.latency))
		copy(dst, m.latency)

		mi := MethodInfo{
			Name:        m.name,
			Path:        m.path,
			SecondLimit: m.secondLimit,
			Qps:         m.qps,
			Total:       m.total,
			latency:     dst,
		}

		mi.CodeMap = make(map[int]uint64, len(m.codeMap))
		for code, count := range m.codeMap {
			mi.CodeMap[code] = count
		}

		m.mutex.Unlock()

		info.MethodList = append(info.MethodList, mi)
	}

	//释放锁
	g.mutex.RUnlock()

	//分别排序计算
	for index, _ := range info.MethodList {
		info.MethodList[index].format()
	}

	return info
}
