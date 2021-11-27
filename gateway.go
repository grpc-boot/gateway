package gateway

import (
	"context"
	"sync"
	"time"
)

type Encode func() []byte

type Gateway interface {
	Input(format Format) (ctx *Context, status string)
	InputWithTimeout(timeout time.Duration, format Format) (ctx *Context, status string)
	Output(ctx *Context, encode Encode) (data []byte, dur time.Duration)
	Info() GatewayInfo
}

type GatewayInfo struct {
	Qps        int32        `json:"qps"`
	Total      uint64       `json:"total"`
	MethodList []MethodInfo `json:"method_list"`
}

type gateway struct {
	Gateway

	mutex      sync.Mutex
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
		if _, exists := g.methodList[option.Path]; exists {
			updateMethod(g.methodList[option.Path], option)
		} else {
			g.methodList[option.Path] = newMethod(option)
		}
	}
}

func (g *gateway) Input(format Format) (ctx *Context, status string) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	ctx = contextWithFormat(format)

	m, exists := g.methodList[ctx.path]
	if !exists {
		return nil, StatusNo
	}

	ctx.Take(m.limiter.Take())

	return ctx, m.status
}

func (g *gateway) InputWithTimeout(timeout time.Duration, format Format) (ctx *Context, status string) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	ctx = contextWithFormat(format)

	m, exists := g.methodList[ctx.path]
	if !exists {
		return nil, StatusNo
	}

	var (
		w                  = make(chan time.Time, 1)
		timeoutCtx, cancel = context.WithTimeout(context.Background(), timeout)
	)
	defer cancel()

	go func() {
		t := m.limiter.Take()
		ctx.Take(t)
		w <- t
	}()

	select {
	case <-w:
		m.status = StatusYes
	case <-timeoutCtx.Done():
		m.status = StatusBusy
	}

	return ctx, m.status
}

func (g *gateway) Output(ctx *Context, encode Encode) (data []byte, dur time.Duration) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	g.total++

	data = encode()

	var (
		current     = time.Now()
		currentUnix = current.Unix()
	)

	if currentUnix-g.lastSecond > 0 {
		g.qps = int32(g.total - g.lastTotal)
		g.lastSecond = current.Unix()
		g.lastTotal = g.total
	}

	m, exists := g.methodList[ctx.path]
	if !exists {
		return data, 0
	}

	m.total++
	dur = current.Sub(ctx.TakeTime())

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

	return data, dur
}

func (g *gateway) Info() GatewayInfo {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	info := GatewayInfo{
		Qps:        g.qps,
		Total:      g.total,
		MethodList: make([]MethodInfo, 0, len(g.methodList)),
	}

	for _, m := range g.methodList {
		info.MethodList = append(info.MethodList, MethodInfo{
			Name:    m.name,
			Path:    m.path,
			Qps:     m.qps,
			Total:   m.total,
			Latency: m.latency[0:],
		})
	}

	return info
}
