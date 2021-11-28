package gateway

import (
	"sync"
	"time"

	"go.uber.org/ratelimit"
)

const (
	StatusUnknow = 0
	StatusNo     = 1
	StatusYes    = 2
	StatusBusy   = 3
)

type Option struct {
	Name        string `json:"name" yaml:"name"`
	Path        string `json:"path" yaml:"path"`
	SecondLimit int    `json:"second_limit" yaml:"second_limit"`
}

type MethodInfo struct {
	Name        string          `json:"name"`
	Path        string          `json:"path"`
	SecondLimit int             `json:"second_limit"`
	Qps         int32           `json:"qps"`
	Total       uint64          `json:"total"`
	Latency     []time.Duration `json:"latency"`
	CodeMap     map[int]uint64  `json:"code_map"`
}

type method struct {
	mutex sync.Mutex

	name        string
	path        string
	secondLimit int
	status      uint8
	qps         int32
	total       uint64
	lastTotal   uint64
	lastSecond  int64
	limiter     ratelimit.Limiter
	latency     []time.Duration
	codeMap     map[int]uint64
}

func newMethod(option Option) *method {
	m := &method{
		status:      StatusYes,
		name:        option.Name,
		path:        option.Path,
		secondLimit: option.SecondLimit,
		codeMap:     make(map[int]uint64, 8),
	}

	if m.secondLimit > 0 {
		m.limiter = ratelimit.New(m.secondLimit)
	}

	m.latency = make([]time.Duration, 0, 100)
	return m
}

func updateMethod(m *method, option Option) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.name, m.secondLimit = option.Name, option.SecondLimit

	if m.secondLimit > 0 {
		m.limiter = ratelimit.New(m.secondLimit)
	} else {
		m.limiter = nil
	}
}
