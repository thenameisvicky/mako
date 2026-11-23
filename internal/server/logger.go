package server

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/thenameisvicky/tokenly-mako/internal/requestctx"
	"github.com/thenameisvicky/tokenly-mako/internal/telemetry"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

var lrwPool = sync.Pool{
	New: func() interface{} {
		return &loggingResponseWriter{}
	},
}

var requestIDPool = sync.Pool{
	New: func() interface{} {
		// 16 bytes for random src, 32 bytes for hex dst
		return make([]byte, 48)
	},
}

// GenerateRequestID generates a hex-encoded request ID reusing a pooled buffer
func GenerateRequestID() string {
	buf := requestIDPool.Get().([]byte)
	defer requestIDPool.Put(buf)

	_, _ = rand.Read(buf[:16])
	hex.Encode(buf[16:48], buf[:16])
	return string(buf[16:48])
}

// LoggingMiddleware logs safe metadata of each request and updates the latency tracker
func LoggingMiddleware(tracker *telemetry.LatencyTracker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			lrw := lrwPool.Get().(*loggingResponseWriter)
			lrw.ResponseWriter = w
			lrw.statusCode = http.StatusOK
			defer func() {
				lrw.ResponseWriter = nil
				lrwPool.Put(lrw)
			}()

			// Initialize the RequestState context
			ctx, state := requestctx.WithState(r.Context())
			r = r.WithContext(ctx)

			// Generate and attach request ID
			state.RequestID = GenerateRequestID()
			w.Header().Set("X-Request-ID", state.RequestID)

			// Serve the request
			next.ServeHTTP(lrw, r)

			duration := time.Since(start)
			tracker.Record(duration)

			userID := "unauthenticated"
			teamID := "unauthenticated"
			provider := "none"
			cacheHitStr := "n/a"

			if state.User != nil {
				userID = state.User.ID
				teamID = state.User.TeamID
			}
			if state.Provider != "" {
				provider = state.Provider
				if state.CacheHit {
					cacheHitStr = "HIT"
				} else {
					cacheHitStr = "MISS"
				}
			}

			model := state.Model
			if model == "" {
				model = "none"
			}

			// Metadata only. Never log the Mako token, the caller's provider key,
			// any Authorization header, or the request body.
			log.Printf("[Request] id=%s method=%s path=%s status=%d duration=%s user_id=%s team_id=%s provider=%s model=%s cache=%s tokens_in=%d tokens_out=%d",
				state.RequestID, r.Method, r.URL.Path, lrw.statusCode, duration, userID, teamID,
				provider, model, cacheHitStr, state.InputTokens, state.OutputTokens)

			// Clean up and release the request state back to the pool
			requestctx.ReleaseState(state)
		})
	}
}
