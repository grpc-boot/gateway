package gateway

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/grpc-boot/base"
)

var (
	ErrPathNotExists = errors.New(`path not exists`)
	ErrTimeout       = errors.New(`input timeout`)
)

type Format func() (accessId string, path string, cacheKey string)
type Send func(code int, data []byte) (err error)

type Gateway interface {
	In(format Format) (ctx *Context, err error)
	InTimeout(timeout time.Duration, format Format) (ctx *Context, err error)
	Out(ctx *Context, sender Send, code int, data []byte) (dur time.Duration, qps int32, total uint64, err error)
	Info() Info
}

type Info struct {
	Qps        int32        `json:"qps"`
	Total      uint64       `json:"total"`
	MethodList []MethodInfo `json:"method_list"`
}

type gateway struct {
	Gateway

	mutex sync.RWMutex

	cache      base.ShardMap
	methodList map[string]*method
	qps        int32
	total      uint64
	lastTotal  uint64
	lastSecond int64
}

func NewGateway(options ...Option) Gateway {
	g := &gateway{
		methodList: make(map[string]*method, len(options)),
		cache:      base.NewShardMap(),
	}

	for _, option := range options {
		g.methodList[option.Path] = newMethod(option)
	}

	return g
}

func (g *gateway) LoadOptions(options ...Option) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	for _, option := range options {
		if m, exists := g.methodList[option.Path]; exists {
			updateMethod(m, option)
		} else {
			g.methodList[option.Path] = newMethod(option)
		}
	}
}

func (g *gateway) In(format Format) (ctx *Context, err error) {
	g.mutex.Lock()

	var (
		accessId, path, cacheKey = format()
		m, exists                = g.methodList[path]
		current                  = time.Now()
	)
	ctx = acquireCtx()
	ctx.accessId, ctx.path, ctx.cacheKey, ctx.accessTime = accessId, path, cacheKey, current

	g.mutex.Unlock()

	//路径不存在
	if !exists {
		ctx.status = StatusNo
		return ctx, ErrPathNotExists
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	//降级
	if m.secondLimit == -1 {
		m.status, ctx.status = StatusNo, StatusNo

		return ctx, nil
	}

	//读取缓存
	if m.cacheSecond > 0 && cacheKey != "" {
		if c, exist := g.cache.Get(cacheKey); exist {
			item, ok := c.(cache)
			if ok && item.expireAt > current.Unix() {
				ctx.cacheData = item.data
				ctx.status = StatusCache
				m.status = StatusYes

				return ctx, nil
			}
		}
	}

	//不限速
	if m.secondLimit == 0 {
		m.status, ctx.status = StatusYes, StatusYes

		return ctx, nil
	}

	//漏斗
	m.limiter.Take()

	//更新状态
	m.status, ctx.status = StatusYes, StatusYes

	return ctx, nil
}

func (g *gateway) InTimeout(timeout time.Duration, format Format) (ctx *Context, err error) {
	g.mutex.Lock()

	var (
		accessId, path, cacheKey = format()
		m, exists                = g.methodList[path]
		current                  = time.Now()
	)
	ctx = acquireCtx()
	ctx.accessId, ctx.path, ctx.cacheKey, ctx.accessTime = accessId, path, cacheKey, current

	g.mutex.Unlock()

	//路径不存在
	if !exists {
		ctx.status = StatusNo
		return ctx, ErrPathNotExists
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	//降级
	if m.secondLimit == -1 {
		m.status, ctx.status = StatusNo, StatusNo

		return ctx, nil
	}

	//读取缓存
	if m.cacheSecond > 0 && cacheKey != "" {
		if c, exist := g.cache.Get(cacheKey); exist {
			item, ok := c.(cache)
			if ok && item.expireAt > current.Unix() {
				ctx.cacheData = item.data
				ctx.status = StatusCache
				m.status = StatusYes

				return ctx, nil
			}
		}
	}

	//不限速
	if m.secondLimit == 0 {
		m.status, ctx.status = StatusYes, StatusYes

		return ctx, nil
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
		m.status, ctx.status = StatusYes, StatusYes
	case <-timeoutCtx.Done():
		m.status, ctx.status = StatusBusy, StatusBusy

		return ctx, ErrTimeout
	}

	return ctx, nil
}

func (g *gateway) Out(ctx *Context, sender Send, code int, data []byte) (dur time.Duration, qps int32, total uint64, err error) {
	defer func() {
		ctx.reset()
		releaseCtx(ctx)
	}()

	g.mutex.Lock()

	g.total++

	err = sender(code, data)

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

	g.mutex.Unlock()

	if !exists {
		return 0, 0, 0, ErrPathNotExists
	}

	dur = current.Sub(ctx.AccessTime())

	m.mutex.Lock()

	m.total++

	//状态码自增
	codeCount, _ := m.codeMap[code]
	m.codeMap[code] = codeCount + 1

	//存储缓存
	if code == m.successCode &&
		m.cacheSecond > 0 &&
		ctx.cacheKey != "" &&
		ctx.Status() != StatusCache {
		g.cache.Set(ctx.cacheKey, cache{
			data:     data,
			expireAt: currentUnix + m.cacheSecond,
		})
	}

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
			CacheSecond: m.cacheSecond,
			SuccessCode: m.successCode,
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
