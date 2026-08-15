package telemetry

import (
	"math"
	"sort"
	"sync"
	"time"
)

type LatencyTracker struct {
	mu        sync.Mutex
	durations []time.Duration
	maxSize   int
}

// NewLatencyTracker creates a new tracker keeping up to maxSize records
func NewLatencyTracker(maxSize int) *LatencyTracker {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &LatencyTracker{
		durations: make([]time.Duration, 0, maxSize),
		maxSize:   maxSize,
	}
}

// Record saves a request duration
func (lt *LatencyTracker) Record(d time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	if len(lt.durations) >= lt.maxSize {
		lt.durations = lt.durations[1:]
	}
	lt.durations = append(lt.durations, d)
}

// GetPercentiles calculates and returns the p50, p95, and p99 request latencies
func (lt *LatencyTracker) GetPercentiles() (p50, p95, p99 time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	n := len(lt.durations)
	if n == 0 {
		return 0, 0, 0
	}

	sorted := make([]time.Duration, n)
	copy(sorted, lt.durations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	p50 = sorted[int(math.Min(float64(n-1), float64(n)*0.50))]
	p95 = sorted[int(math.Min(float64(n-1), float64(n)*0.95))]
	p99 = sorted[int(math.Min(float64(n-1), float64(n)*0.99))]

	return p50, p95, p99
}
