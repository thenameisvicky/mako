package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/thenameisvicky/tokenly-gateway/internal/cache"
	"github.com/thenameisvicky/tokenly-gateway/internal/requestctx"
	"github.com/thenameisvicky/tokenly-gateway/internal/storage"
)

type Middleware struct {
	jwtSecret string
	storage   *storage.Storage
	cache     *cache.Cache
}

// NewMiddleware constructs the auth middleware
func NewMiddleware(jwtSecret string, store *storage.Storage, cache *cache.Cache) *Middleware {
	return &Middleware{
		jwtSecret: jwtSecret,
		storage:   store,
		cache:     cache,
	}
}

// Authenticate intercepts HTTP requests and validates the JWT Token
func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Match exactly: all /api/v1/* except /api/v1/login, /api/v1/health are protected
		if path == "/api/v1/login" || path == "/api/v1/health" || path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		// Protect all routes with prefix /api/v1/*
		if !strings.HasPrefix(path, "/api/v1/") {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status":  "error",
				"message": "Missing or invalid token",
			})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		token = strings.TrimSpace(token)

		sub, companyID, err := VerifyToken(token, m.jwtSecret)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			if errors.Is(err, ErrTokenExpired) {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status":  "error",
					"code":    "TOKEN_EXPIRED",
					"message": "Token expired, please log in again",
				})
			} else {
				_ = json.NewEncoder(w).Encode(map[string]string{
					"status":  "error",
					"message": "Invalid token",
				})
			}
			return
		}

		var user *storage.User
		var hit bool

		// Check the user cache first
		user, hit = m.cache.GetUser(sub)
		if !hit {
			var err error
			user, err = m.storage.GetUserByID(r.Context(), sub)
			if err != nil || user == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"status":"error","message":"User not found"}`))
				return
			}
			m.cache.SetUser(user)
		}

		// Verify company/team match
		if user.TeamID != companyID {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"status":"error","message":"User not found"}`))
			return
		}

		// Attach user identity to request context state
		if state, ok := requestctx.GetState(r.Context()); ok {
			state.User = user
		}

		next.ServeHTTP(w, r)
	})
}
