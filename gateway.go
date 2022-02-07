package gateway

import (
	"sync"
	"time"

	"go.uber.org/atomic"
)

const (
	StatusNo   = 0
	StatusYes  = 1
	StatusBusy = 2
)

type Gateway interface {
	// In 接收请求
	In(path string) (status uint8, exists bool)
	// Out gateway响应
	Out(accessTime time.Time, path string, code int) (dur time.Duration, qps int32, total uint64, err error)
	// Info 获取gateway信息
	Info() (info Info)
	// Close 释放gateway资源
	Close() (err error)
}

type gateway struct {
	mutex       sync.RWMutex
	methodList  map[string]*method
	qps         int32
	total       uint64
	lastTotal   uint64
	lastSecond  int64
	doneChan    chan uint8
	hasDone     atomic.Bool
	tick        *time.Ticker
	optionsFunc OptionsFunc
}

// NewGateway new gateway intance
func NewGateway(duration time.Duration, optionsFunc OptionsFunc) Gateway {
	var (
		options = optionsFunc()
		g       = &gateway{
			optionsFunc: optionsFunc,
			methodList:  make(map[string]*method, len(options)),
			doneChan:    make(chan uint8, 1),
		}
	)

	for _, option := range options {
		g.methodList[option.Path] = newMethod(option)
	}

	if duration >= time.Nanosecond {
		g.tick = time.NewTicker(duration)
		go g.startSyncOptions()
	}

	return g
}

func (g *gateway) startSyncOptions() {
	if g.tick == nil {
		return
	}

	g.hasDone.Store(false)

	for {
		select {
		case <-g.tick.C:
			options := g.optionsFunc()
			g.loadOptions(options...)
		case <-g.doneChan:
			return
		default:
			time.Sleep(time.Millisecond * 10)
		}
	}
}

func (g *gateway) stopSyncOptions() {
	if !g.hasDone.CAS(false, true) {
		return
	}

	g.doneChan <- 1
}

func (g *gateway) loadOptions(options ...Option) {
	if len(options) < 1 {
		return
	}

	g.mutex.Lock()
	defer g.mutex.Unlock()

	for _, option := range options {
		if _, exists := g.methodList[option.Path]; exists {
			g.methodList[option.Path].update(option)
		} else {
			g.methodList[option.Path] = newMethod(option)
		}
	}
}

// In 接收请求
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

	if !m.limiter.Allow() {
		return StatusBusy, exists
	}

	return StatusYes, exists
}

// Out gateway响应
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
	if len(m.latency) >= sampleCount {
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

// Info 获取gateway信息
func (g *gateway) Info() (info Info) {
	g.mutex.RLock()

	info = Info{
		Qps:        g.qps,
		Total:      g.total,
		MethodList: make([]MethodInfo, 0, len(g.methodList)),
	}

	for _, m := range g.methodList {
		m.mutex.RLock()

		dst := make(LatencyList, len(m.latency), len(m.latency))
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

		m.mutex.RUnlock()

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

// Close 释放gateway资源
func (g *gateway) Close() (err error) {
	g.stopSyncOptions()
	return
}
