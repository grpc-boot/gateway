package gateway

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"go.uber.org/ratelimit"
)

const (
	StatusNo   = 0
	StatusYes  = 1
	StatusBusy = 2
)

const (
	sampleMax = 10000
)

type Option struct {
	Name        string `json:"name" yaml:"name"`
	Path        string `json:"path" yaml:"path"`
	SecondLimit int    `json:"second_limit" yaml:"second_limit"`
}

type MethodInfo struct {
	Name        string         `json:"name"`
	Path        string         `json:"path"`
	SecondLimit int            `json:"second_limit"`
	Qps         int32          `json:"qps"`
	Total       uint64         `json:"total"`
	Avg         string         `json:"avg"`
	Min         string         `json:"min"`
	Max         string         `json:"max"`
	L90         string         `json:"90_line"`
	L95         string         `json:"95_line"`
	CodeMap     map[int]uint64 `json:"code_map"`
	latency     LatencyList
}

func (mi *MethodInfo) format() {
	if len(mi.latency) < 1 {
		return
	}

	sort.Sort(mi.latency)
	mi.Min = fmt.Sprint(mi.latency.Min())
	mi.Max = fmt.Sprint(mi.latency.Max())
	mi.Avg = fmt.Sprint(mi.latency.Avg())
	mi.L90 = fmt.Sprint(mi.latency.L90())
	mi.L95 = fmt.Sprint(mi.latency.L95())
}

type method struct {
	mutex sync.RWMutex

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

	m.latency = make([]time.Duration, 0, sampleMax)
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
