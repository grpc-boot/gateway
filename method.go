package gateway

import (
	"time"

	"go.uber.org/ratelimit"
)

const (
	StatusYes  = `Y`
	StatusNo   = `N`
	StatusBusy = `B`
)

type Option struct {
	Name        string `json:"name" yaml:"name"`
	Path        string `json:"path" yaml:"path"`
	Status      string `json:"status" yaml:"status"`
	SecondLimit int    `json:"second_limit" yaml:"second_limit"`
}

type MethodInfo struct {
	Name    string          `json:"name"`
	Path    string          `json:"path"`
	Qps     int32           `json:"qps"`
	Total   uint64          `json:"total"`
	Latency []time.Duration `json:"latency"`
}

type method struct {
	name        string
	path        string
	secondLimit int
	status      string
	qps         int32
	total       uint64
	lastTotal   uint64
	lastSecond  int64
	limiter     ratelimit.Limiter
	latency     []time.Duration
}

func newMethod(option Option) *method {
	m := &method{
		name:        option.Name,
		path:        option.Path,
		secondLimit: option.SecondLimit,
		status:      option.Status,
	}

	if m.status == "" {
		m.status = StatusYes
	}

	if m.secondLimit > 0 {
		m.limiter = ratelimit.New(m.secondLimit)
	}

	m.latency = make([]time.Duration, 0, 100)
	return m
}

func updateMethod(m *method, option Option) {
	m.name, m.secondLimit = option.Name, option.SecondLimit
	if option.Status == "" {
		m.status = option.Status
	}

	if m.secondLimit > 0 {
		m.limiter = ratelimit.New(m.secondLimit)
	} else {
		m.limiter = nil
	}
}
