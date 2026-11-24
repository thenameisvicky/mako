package server

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/thenameisvicky/tokenly-mako/internal/requestctx"
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

// LoggingMiddleware logs safe metadata of each request.
func LoggingMiddleware() func(http.Handler) http.Handler {
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

			userID, teamID := "unauthenticated", "unauthenticated"
			if state.User != nil {
				userID, teamID = state.User.ID, state.User.TeamID
			}

			// Metadata only. Never log the Mako token or the request body.
			log.Printf("[Request] id=%s method=%s path=%s status=%d duration=%s user_id=%s team_id=%s",
				state.RequestID, r.Method, r.URL.Path, lrw.statusCode, time.Since(start), userID, teamID)

			// Clean up and release the request state back to the pool
			requestctx.ReleaseState(state)
		})
	}
}
