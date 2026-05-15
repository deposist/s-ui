package service

import (
	"sync"
	"time"
)

const observabilityMaxSamples = 240

type ObservabilitySample struct {
	DateTime int64                  `json:"dateTime"`
	CPU      float64                `json:"cpu"`
	Memory   map[string]interface{} `json:"memory"`
	Network  map[string]interface{} `json:"network"`
}

type CoreSample struct {
	DateTime int64                  `json:"dateTime"`
	Core     map[string]interface{} `json:"core"`
}

type ObservabilityService struct {
	ServerService
}

var observabilityHistory = struct {
	sync.Mutex
	samples []ObservabilitySample
	core    []CoreSample
}{}

func (s *ObservabilityService) History() []ObservabilitySample {
	sample := ObservabilitySample{
		DateTime: time.Now().Unix(),
		CPU:      s.ServerService.GetCpuPercent(),
		Memory:   s.ServerService.GetMemInfo(),
		Network:  s.ServerService.GetNetInfo(),
	}
	observabilityHistory.Lock()
	defer observabilityHistory.Unlock()
	observabilityHistory.samples = appendBounded(observabilityHistory.samples, sample, observabilityMaxSamples)
	return append([]ObservabilitySample(nil), observabilityHistory.samples...)
}

func (s *ObservabilityService) CoreHistory() []CoreSample {
	sample := CoreSample{
		DateTime: time.Now().Unix(),
		Core:     s.ServerService.GetSingboxInfo(),
	}
	observabilityHistory.Lock()
	defer observabilityHistory.Unlock()
	observabilityHistory.core = appendBounded(observabilityHistory.core, sample, observabilityMaxSamples)
	return append([]CoreSample(nil), observabilityHistory.core...)
}

func appendBounded[T any](items []T, item T, max int) []T {
	items = append(items, item)
	if len(items) <= max {
		return items
	}
	return items[len(items)-max:]
}
