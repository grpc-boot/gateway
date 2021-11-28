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

type Context struct {
	mutex sync.RWMutex

	status     uint8
	accessId   string
	path       string
	cacheKey   string
	cacheData  []byte
	accessTime time.Time
	data       map[string]interface{}
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

func (c *Context) AccessId() string {
	return c.accessId
}

func (c *Context) Path() string {
	return c.path
}

func (c *Context) Status() uint8 {
	return c.status
}

func (c *Context) CacheData() []byte {
	return c.cacheData
}

func (c *Context) AccessTime() time.Time {
	return c.accessTime
}

func (c *Context) reset() {
	c.status = StatusUnknow
	c.accessId = ""
	c.path = ""
	c.data = nil
}
