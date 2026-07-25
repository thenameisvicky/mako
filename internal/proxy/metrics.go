package proxy

import (
	"context"
	"crypto/tls"
	"net/http/httptrace"
	"sync"
	"sync/atomic"
	"time"
)

type SessionMetrics struct {
	Requests           int64
	SuccessfulRequests int64
	FailedRequests     int64
	TotalTTFB          int64 // in microseconds
	TotalDuration      int64 // in microseconds
	BytesUploaded      uint64
	BytesDownloaded    uint64
}

var (
	GlobalMetrics SessionMetrics
)

type Tracker struct {
	StartVal     time.Time
	DialStartVal time.Time
	DialEndVal   time.Time
	TLSStartVal  time.Time
	TLSEndVal    time.Time
	FirstByteVal time.Time
	LastByteVal  time.Time
	Mu           sync.Mutex
}

func NewTracker() *Tracker {
	return &Tracker{
		StartVal: time.Now(),
	}
}

func (t *Tracker) RecordDialStart() {
	t.Mu.Lock()
	defer t.Mu.Unlock()
	t.DialStartVal = time.Now()
}

func (t *Tracker) RecordDialEnd(success bool) {
	t.Mu.Lock()
	defer t.Mu.Unlock()
	t.DialEndVal = time.Now()
}

func (t *Tracker) RecordTLSStart() {
	t.Mu.Lock()
	defer t.Mu.Unlock()
	t.TLSStartVal = time.Now()
}

func (t *Tracker) RecordTLSEnd(success bool) {
	t.Mu.Lock()
	defer t.Mu.Unlock()
	t.TLSEndVal = time.Now()
}

func (t *Tracker) RecordFirstByte() {
	t.Mu.Lock()
	defer t.Mu.Unlock()
	t.FirstByteVal = time.Now()
	
	ttfb := time.Since(t.StartVal)
	atomic.AddInt64(&GlobalMetrics.TotalTTFB, int64(ttfb/time.Microsecond))
}

func (t *Tracker) RecordLastByte() {
	t.Mu.Lock()
	defer t.Mu.Unlock()
	t.LastByteVal = time.Now()
	
	dur := time.Since(t.StartVal)
	atomic.AddInt64(&GlobalMetrics.TotalDuration, int64(dur/time.Microsecond))
}

func IncrementRequests() {
	atomic.AddInt64(&GlobalMetrics.Requests, 1)
}

func IncrementSuccessfulRequests() {
	atomic.AddInt64(&GlobalMetrics.SuccessfulRequests, 1)
}

func IncrementFailedRequests() {
	atomic.AddInt64(&GlobalMetrics.FailedRequests, 1)
}

func AddBytesUploaded(n uint64) {
	atomic.AddUint64(&GlobalMetrics.BytesUploaded, n)
}

func AddBytesDownloaded(n uint64) {
	atomic.AddUint64(&GlobalMetrics.BytesDownloaded, n)
}

func WithMetricsTrace(ctx context.Context, tracker *Tracker) context.Context {
	trace := &httptrace.ClientTrace{
		ConnectStart: func(network, addr string) {
			tracker.RecordDialStart()
		},
		ConnectDone: func(network, addr string, err error) {
			tracker.RecordDialEnd(err == nil)
		},
		TLSHandshakeStart: func() {
			tracker.RecordTLSStart()
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			tracker.RecordTLSEnd(err == nil)
		},
		GotFirstResponseByte: func() {
			tracker.RecordFirstByte()
		},
	}
	return httptrace.WithClientTrace(ctx, trace)
}
