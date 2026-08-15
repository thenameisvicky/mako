package server

import (
	"net/http"

	"github.com/thenameisvicky/tokenly-gateway/internal/auth"
	"github.com/thenameisvicky/tokenly-gateway/internal/proxy"
	"github.com/thenameisvicky/tokenly-gateway/internal/telemetry"
)

// NewRouter builds the main routing multiplexer and wires all modular endpoints and middlewares
func NewRouter(
	authHandler *auth.AuthHandler,
	authMiddleware *auth.Middleware,
	telemetryHandler *telemetry.TelemetryHandler,
	proxyHandler *proxy.ProxyHandler,
	tracker *telemetry.LatencyTracker,
) http.Handler {
	mux := http.NewServeMux()

	// Register modular routes
	auth.RegisterRoutes(mux, authHandler)
	telemetry.RegisterRoutes(mux, telemetryHandler)
	proxy.RegisterRoutes(mux, proxyHandler, authMiddleware)

	// Wrap in logging middleware
	return LoggingMiddleware(tracker)(mux)
}
