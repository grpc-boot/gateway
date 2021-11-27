package gateway

import (
	"sync"
	"time"
)

var (
	ctxPool = sync.Pool{
		New: func() interface{} {
			return &Context{}
		},
	}

	acquireCtx = func() *Context {
		return ctxPool.Get().(*Context)
	}

	releaseCtx = func(ctx *Context) {
		ctx.reset()
		ctxPool.Put(ctx)
	}
)

type Format func() (accessId string, ip string, path string, param []byte)

type Context struct {
	mutex sync.RWMutex

	accessId   string
	ip         string
	path       string
	param      []byte
	accessTime time.Time
	takeTime   time.Time
	data       map[string]interface{}
}

func contextWithFormat(format Format) *Context {
	ctx := acquireCtx()
	ctx.accessId, ctx.ip, ctx.path, ctx.param = format()
	ctx.accessTime = time.Now()
	return ctx
}

func (c *Context) Set(key string, value interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.data == nil {
		c.data = make(map[string]interface{})
	}
	c.data[key] = value
}

func (c *Context) Get(key string) (value interface{}, exists bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	value, exists = c.data[key]
	return
}

func (c *Context) Take(t time.Time) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.takeTime = t
}

func (c *Context) TakeTime() time.Time {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.takeTime
}

func (c *Context) Close() {
	releaseCtx(c)
}

func (c *Context) reset() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.accessId = ""
	c.ip = ""
	c.path = ""
	c.data = nil
}
