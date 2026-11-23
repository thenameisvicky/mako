package server

import (
	"net/http"

	"github.com/thenameisvicky/tokenly-mako/internal/auth"
	"github.com/thenameisvicky/tokenly-mako/internal/chats"
	"github.com/thenameisvicky/tokenly-mako/internal/governance"
	"github.com/thenameisvicky/tokenly-mako/internal/proxy"
	"github.com/thenameisvicky/tokenly-mako/internal/telemetry"
)

// Handlers bundles every modular handler the router needs to register.
type Handlers struct {
	Auth       *auth.AuthHandler
	Chats      *chats.Handler
	Governance *governance.Handler
	Proxy      *proxy.ProxyHandler
	Telemetry  *telemetry.TelemetryHandler

	Middleware     *auth.Middleware
	Tracker        *telemetry.LatencyTracker
	AllowedOrigins []string
}

// NewRouter builds the main multiplexer and wires all modular endpoints
func NewRouter(h Handlers) http.Handler {
	mux := http.NewServeMux()

	// Each package owns its own paths and method matching.
	auth.RegisterRoutes(mux, h.Auth)
	telemetry.RegisterRoutes(mux, h.Telemetry)
	proxy.RegisterRoutes(mux, h.Proxy, h.Middleware)
	chats.RegisterRoutes(mux, h.Chats, h.Middleware.Authenticate)
	governance.RegisterRoutes(mux, h.Governance, h.Middleware.Authenticate)

	// Outermost first: CORS answers preflight before anything else runs, and
	// logging measures every request that gets past it.
	return CORSMiddleware(h.AllowedOrigins)(LoggingMiddleware(h.Tracker)(mux))
}
