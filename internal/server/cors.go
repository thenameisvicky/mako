package server

import (
	"log"
	"net/http"
	"strings"
	"sync"
)

// CORSMiddleware allows the web dashboard, served from a different origin, to
// call the proxy.
//
// Allowed origins are configured explicitly rather than reflected back from the
// request: a proxy that echoes any Origin would let any website on the
// internet make authenticated calls with a signed-in user's browser.
//
// Two conveniences are supported for local development:
//
//   - disables the check entirely
//     http://localhost:*    allows any port on that host
//
// The port wildcard matters because Next.js silently picks the next free port
// when its default is taken, which otherwise produces a blocked preflight that
// looks exactly like an unreachable server.
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	var (
		allowAll bool
		exact    = make(map[string]bool, len(allowedOrigins))
		prefixes []string // entries ending in ":*", stored as "scheme://host:"
	)

	for _, o := range allowedOrigins {
		o = strings.TrimSpace(o)
		switch {
		case o == "":
		case o == "*":
			allowAll = true
		case strings.HasSuffix(o, ":*"):
			prefixes = append(prefixes, strings.TrimSuffix(o, "*"))
		default:
			exact[o] = true
		}
	}

	// Rejections are logged once per origin, so a misconfigured dashboard is
	// visible in the proxy log instead of failing silently in the browser.
	var (
		mu       sync.Mutex
		reported = make(map[string]bool)
	)
	reportOnce := func(origin string) {
		mu.Lock()
		defer mu.Unlock()
		if reported[origin] {
			return
		}
		reported[origin] = true
		log.Printf("[CORS] blocked origin %q — add it to CORS_ALLOWED_ORIGINS (currently %v)",
			origin, allowedOrigins)
	}

	permitted := func(origin string) bool {
		if allowAll || exact[origin] {
			return true
		}
		for _, p := range prefixes {
			if rest, ok := strings.CutPrefix(origin, p); ok && isPort(rest) {
				return true
			}
		}
		return false
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if origin != "" {
				if permitted(origin) {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Mako-Token, Authorization, x-api-key, X-Provider")
					w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
					w.Header().Set("Access-Control-Max-Age", "600")
					// Responses differ by Origin, so caches must not share them.
					w.Header().Add("Vary", "Origin")
				} else {
					reportOnce(origin)
				}
			}

			// Preflight never reaches a handler.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isPort reports whether s is a non-empty run of digits, so a "host:*" pattern
// matches only a port and never a longer hostname.
func isPort(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
