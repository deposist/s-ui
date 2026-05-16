package service

import (
	"sync"
	"time"

	"github.com/deposist/s-ui-rus-inst/logger"
	"github.com/deposist/s-ui-rus-inst/util/common"
)

type ObservabilityBucket string

const (
	ObservabilityBucket2s  ObservabilityBucket = "2s"
	ObservabilityBucket30s ObservabilityBucket = "30s"
	ObservabilityBucket1m  ObservabilityBucket = "1m"
	ObservabilityBucket5m  ObservabilityBucket = "5m"

	observabilityDefaultMemoryCapMB   = 32
	observabilitySampleEstimateBytes  = 2048
	observabilityCoreSampleBytes      = 1024
	observabilityWarnMemoryMinSeconds = 60
)

var observabilityDefaultBucketCaps = map[ObservabilityBucket]int{
	ObservabilityBucket2s:  300,
	ObservabilityBucket30s: 240,
	ObservabilityBucket1m:  240,
	ObservabilityBucket5m:  144,
}

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
	SettingService
}

type observabilityStore struct {
	sync.Mutex
	samples              map[ObservabilityBucket]*ringBuffer[ObservabilitySample]
	core                 map[ObservabilityBucket]*ringBuffer[CoreSample]
	lastMemoryWarnCapMB  int
	lastMemoryWarnUnix   int64
	lastAppliedMemoryCap int
}

var observabilityHistory = newObservabilityStore()

func newObservabilityStore() *observabilityStore {
	caps := copyObservabilityCaps(observabilityDefaultBucketCaps)
	return &observabilityStore{
		samples:              newObservabilityRings[ObservabilitySample](caps),
		core:                 newObservabilityRings[CoreSample](caps),
		lastAppliedMemoryCap: observabilityDefaultMemoryCapMB,
	}
}

func (s *ObservabilityService) CurrentObservabilitySample() ObservabilitySample {
	return ObservabilitySample{
		DateTime: time.Now().Unix(),
		CPU:      s.ServerService.GetCpuPercent(),
		Memory:   s.ServerService.GetMemInfo(),
		Network:  s.ServerService.GetNetInfo(),
	}
}

func (s *ObservabilityService) CurrentCoreSample() CoreSample {
	return CoreSample{
		DateTime: time.Now().Unix(),
		Core:     s.ServerService.GetSingboxInfo(),
	}
}

func (s *ObservabilityService) History() []ObservabilitySample {
	if err := s.RecordObservabilitySample(ObservabilityBucket2s, s.CurrentObservabilitySample()); err != nil {
		logger.Warning("record observability sample failed:", err)
	}
	samples, err := s.HistoryForBucket(ObservabilityBucket2s)
	if err != nil {
		logger.Warning("read observability history failed:", err)
		return nil
	}
	return samples
}

func (s *ObservabilityService) CoreHistory() []CoreSample {
	if err := s.RecordCoreSample(ObservabilityBucket2s, s.CurrentCoreSample()); err != nil {
		logger.Warning("record core observability sample failed:", err)
	}
	samples, err := s.CoreHistoryForBucket(ObservabilityBucket2s)
	if err != nil {
		logger.Warning("read core observability history failed:", err)
		return nil
	}
	return samples
}

func (s *ObservabilityService) RecordObservabilitySample(bucket ObservabilityBucket, sample ObservabilitySample) error {
	if !IsValidObservabilityBucket(bucket) {
		return common.NewError("invalid observability bucket")
	}
	caps, capMB := s.observabilityCaps()
	observabilityHistory.Lock()
	defer observabilityHistory.Unlock()
	observabilityHistory.applyCaps(caps, capMB)
	observabilityHistory.samples[bucket].append(sample)
	return nil
}

func (s *ObservabilityService) RecordCoreSample(bucket ObservabilityBucket, sample CoreSample) error {
	if !IsValidObservabilityBucket(bucket) {
		return common.NewError("invalid observability bucket")
	}
	caps, capMB := s.observabilityCaps()
	observabilityHistory.Lock()
	defer observabilityHistory.Unlock()
	observabilityHistory.applyCaps(caps, capMB)
	observabilityHistory.core[bucket].append(sample)
	return nil
}

func (s *ObservabilityService) HistoryForBucket(bucket ObservabilityBucket) ([]ObservabilitySample, error) {
	if !IsValidObservabilityBucket(bucket) {
		return nil, common.NewError("invalid observability bucket")
	}
	caps, capMB := s.observabilityCaps()
	observabilityHistory.Lock()
	defer observabilityHistory.Unlock()
	observabilityHistory.applyCaps(caps, capMB)
	return observabilityHistory.samples[bucket].snapshot(), nil
}

func (s *ObservabilityService) CoreHistoryForBucket(bucket ObservabilityBucket) ([]CoreSample, error) {
	if !IsValidObservabilityBucket(bucket) {
		return nil, common.NewError("invalid observability bucket")
	}
	caps, capMB := s.observabilityCaps()
	observabilityHistory.Lock()
	defer observabilityHistory.Unlock()
	observabilityHistory.applyCaps(caps, capMB)
	return observabilityHistory.core[bucket].snapshot(), nil
}

func IsValidObservabilityBucket(bucket ObservabilityBucket) bool {
	_, ok := observabilityDefaultBucketCaps[bucket]
	return ok
}

func ParseObservabilityBucket(raw string) (ObservabilityBucket, error) {
	if raw == "" {
		return ObservabilityBucket2s, nil
	}
	bucket := ObservabilityBucket(raw)
	if !IsValidObservabilityBucket(bucket) {
		return "", common.NewError("invalid observability bucket")
	}
	return bucket, nil
}

func (s *ObservabilityService) observabilityCaps() (map[ObservabilityBucket]int, int) {
	capMB, err := s.SettingService.GetObservabilityMemoryCapMB()
	if err != nil || capMB <= 0 {
		capMB = observabilityDefaultMemoryCapMB
	}
	return capsForObservabilityMemory(capMB), capMB
}

func capsForObservabilityMemory(capMB int) map[ObservabilityBucket]int {
	caps := copyObservabilityCaps(observabilityDefaultBucketCaps)
	capBytes := int64(capMB) * 1024 * 1024
	defaultBytes := estimatedObservabilityBytes(observabilityDefaultBucketCaps)
	if capBytes >= defaultBytes {
		return caps
	}
	if capBytes <= 0 {
		capBytes = 1
	}
	scale := float64(capBytes) / float64(defaultBytes)
	for bucket, defaultCap := range observabilityDefaultBucketCaps {
		capacity := int(float64(defaultCap) * scale)
		if capacity < 1 {
			capacity = 1
		}
		caps[bucket] = capacity
	}
	return caps
}

func estimatedObservabilityBytes(caps map[ObservabilityBucket]int) int64 {
	var total int64
	for _, cap := range caps {
		total += int64(cap) * (observabilitySampleEstimateBytes + observabilityCoreSampleBytes)
	}
	return total
}

func copyObservabilityCaps(src map[ObservabilityBucket]int) map[ObservabilityBucket]int {
	dst := make(map[ObservabilityBucket]int, len(src))
	for bucket, capacity := range src {
		dst[bucket] = capacity
	}
	return dst
}

func newObservabilityRings[T any](caps map[ObservabilityBucket]int) map[ObservabilityBucket]*ringBuffer[T] {
	rings := make(map[ObservabilityBucket]*ringBuffer[T], len(observabilityDefaultBucketCaps))
	for bucket := range observabilityDefaultBucketCaps {
		rings[bucket] = newRingBuffer[T](caps[bucket])
	}
	return rings
}

func (h *observabilityStore) applyCaps(caps map[ObservabilityBucket]int, capMB int) {
	for bucket := range observabilityDefaultBucketCaps {
		capacity := caps[bucket]
		if h.samples[bucket] == nil {
			h.samples[bucket] = newRingBuffer[ObservabilitySample](capacity)
		}
		if h.core[bucket] == nil {
			h.core[bucket] = newRingBuffer[CoreSample](capacity)
		}
		h.samples[bucket].setCap(capacity)
		h.core[bucket].setCap(capacity)
	}
	h.warnIfCapped(caps, capMB)
	h.lastAppliedMemoryCap = capMB
}

func (h *observabilityStore) warnIfCapped(caps map[ObservabilityBucket]int, capMB int) {
	if estimatedObservabilityBytes(caps) >= estimatedObservabilityBytes(observabilityDefaultBucketCaps) {
		return
	}
	now := time.Now().Unix()
	if h.lastMemoryWarnCapMB == capMB && now-h.lastMemoryWarnUnix < observabilityWarnMemoryMinSeconds {
		return
	}
	h.lastMemoryWarnCapMB = capMB
	h.lastMemoryWarnUnix = now
	logger.Warningf("observability history capacities reduced by observabilityMemoryCapMB=%d", capMB)
}

type ringBuffer[T any] struct {
	items []T
	next  int
	full  bool
}

func newRingBuffer[T any](capacity int) *ringBuffer[T] {
	if capacity < 1 {
		capacity = 1
	}
	return &ringBuffer[T]{
		items: make([]T, 0, capacity),
	}
}

func (r *ringBuffer[T]) append(item T) {
	if cap(r.items) == 0 {
		r.items = make([]T, 0, 1)
	}
	if len(r.items) < cap(r.items) {
		r.items = append(r.items, item)
		if len(r.items) == cap(r.items) {
			r.full = true
			r.next = 0
		}
		return
	}
	r.items[r.next] = item
	r.next = (r.next + 1) % len(r.items)
	r.full = true
}

func (r *ringBuffer[T]) setCap(capacity int) {
	if capacity < 1 {
		capacity = 1
	}
	if cap(r.items) == capacity {
		return
	}
	current := r.snapshot()
	if len(current) > capacity {
		current = current[len(current)-capacity:]
	}
	r.items = make([]T, 0, capacity)
	r.next = 0
	r.full = false
	for _, item := range current {
		r.append(item)
	}
}

func (r *ringBuffer[T]) snapshot() []T {
	if len(r.items) == 0 {
		return nil
	}
	out := make([]T, 0, len(r.items))
	if !r.full {
		return append(out, r.items...)
	}
	out = append(out, r.items[r.next:]...)
	out = append(out, r.items[:r.next]...)
	return out
}
