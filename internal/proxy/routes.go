package proxy

import (
	"net/http"

	"github.com/thenameisvicky/tokenly-mako/internal/auth"
)

// RegisterRoutes registers the proxy handler on the serve mux wrapped in authorization middleware
func RegisterRoutes(mux *http.ServeMux, handler *ProxyHandler, authMiddleware *auth.Middleware) {
	mux.Handle("/api/v1/proxy", authMiddleware.Authenticate(handler))
	mux.Handle("/api/v1/messages", authMiddleware.Authenticate(handler))
	mux.Handle("/api/v1/messages/", authMiddleware.Authenticate(handler))
}
