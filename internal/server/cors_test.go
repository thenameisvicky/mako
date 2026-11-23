package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func TestCORSAllowsConfiguredOrigin(t *testing.T) {
	h := CORSMiddleware([]string{"http://localhost:3000"})(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/policy", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
	require.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), "X-Mako-Token")
	require.Equal(t, "Origin", rec.Header().Get("Vary"))
}

// An unlisted origin must not be reflected back: doing so would let any site on
// the internet make authenticated calls with a signed-in user's browser.
func TestCORSDoesNotReflectUnknownOrigin(t *testing.T) {
	h := CORSMiddleware([]string{"http://localhost:3000"})(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/policy", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSPreflightShortCircuits(t *testing.T) {
	var reached bool
	h := CORSMiddleware([]string{"http://localhost:3000"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/policy", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.False(t, reached, "preflight must not reach the handler")
}

func TestCORSWildcard(t *testing.T) {
	h := CORSMiddleware([]string{"*"})(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://anything.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, "https://anything.example.com", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSNoOriginHeaderIsUntouched(t *testing.T) {
	h := CORSMiddleware([]string{"http://localhost:3000"})(okHandler())

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSPortWildcardAllowsAnyPortOnHost(t *testing.T) {
	h := CORSMiddleware([]string{"http://localhost:*"})(okHandler())

	for _, origin := range []string{
		"http://localhost:3000",
		"http://localhost:3002",
		"http://localhost:8123",
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/policy", nil)
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, origin, rec.Header().Get("Access-Control-Allow-Origin"), origin)
	}
}

// The wildcard must match a port and nothing else, or "http://localhost:*"
// would also admit a lookalike host an attacker controls.
func TestCORSPortWildcardDoesNotMatchLookalikeHosts(t *testing.T) {
	h := CORSMiddleware([]string{"http://localhost:*"})(okHandler())

	for _, origin := range []string{
		"http://localhost:3000.evil.com",
		"http://localhost:evil",
		"http://localhost.evil.com",
		"https://localhost:3000",
		"http://localhost:",
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/policy", nil)
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"), origin)
	}
}

func TestCORSExactAndWildcardTogether(t *testing.T) {
	h := CORSMiddleware([]string{"https://mako.example.com", "http://localhost:*"})(okHandler())

	for origin, want := range map[string]string{
		"https://mako.example.com": "https://mako.example.com",
		"http://localhost:4321":    "http://localhost:4321",
		"https://evil.example.com": "",
	} {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, want, rec.Header().Get("Access-Control-Allow-Origin"), origin)
	}
}
