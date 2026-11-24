package server

import (
	"net/http"

	"github.com/thenameisvicky/tokenly-mako/internal/auth"
	"github.com/thenameisvicky/tokenly-mako/internal/chats"
)

// Handlers bundles every modular handler the router needs to register.
type Handlers struct {
	Auth           *auth.AuthHandler
	Chats          *chats.Handler
	Middleware     *auth.Middleware
	AllowedOrigins []string
}

// NewRouter builds the main multiplexer and wires all modular endpoints
func NewRouter(h Handlers) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/api/v1/health", handleHealth)

	// Each package owns its own paths and method matching.
	auth.RegisterRoutes(mux, h.Auth)
	mux.Handle("/api/v1/me", h.Middleware.Authenticate(http.HandlerFunc(h.Auth.HandleMe)))
	chats.RegisterRoutes(mux, h.Chats, h.Middleware.Authenticate)

	// Outermost first: CORS answers preflight before anything else runs, and
	// logging records every request that gets past it.
	return CORSMiddleware(h.AllowedOrigins)(LoggingMiddleware()(mux))
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"OK"}`))
}
