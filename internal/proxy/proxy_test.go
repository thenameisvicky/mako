package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thenameisvicky/tokenly-mako/internal/policy"
	"github.com/thenameisvicky/tokenly-mako/internal/requestctx"
	"github.com/thenameisvicky/tokenly-mako/internal/storage"
)

// newTestProxy builds a handler with policy and budgeting disabled, isolating
// the forwarding behaviour. Both components are nil-safe by design.
func newTestProxy(upstream string) *ProxyHandler {
	return NewProxyHandler(nil, nil, upstream)
}

// withUser attaches an authenticated request state, as the auth middleware would.
func withUser(r *http.Request) *http.Request {
	ctx, state := requestctx.WithState(r.Context())
	state.User = &storage.User{ID: "user-1", TeamID: "team-1"}
	return r.WithContext(ctx)
}

func TestProxyForwardsCallerCredentialUntouched(t *testing.T) {
	var gotAuth, gotAPIKey, gotMakoToken, gotBody string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAPIKey = r.Header.Get("x-api-key")
		gotMakoToken = r.Header.Get("X-Mako-Token")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":10,"output_tokens":5}}`))
	}))
	defer upstream.Close()

	ph := newTestProxy(upstream.URL)

	body := `{"model":"claude-sonnet-4","max_tokens":100}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer sk-ant-users-own-key")
	req.Header.Set("x-api-key", "sk-ant-users-own-key")
	req.Header.Set("X-Mako-Token", "the-mako-token")
	req = withUser(req)

	rec := httptest.NewRecorder()
	ph.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	// The user's own provider credential must arrive unmodified.
	require.Equal(t, "Bearer sk-ant-users-own-key", gotAuth)
	require.Equal(t, "sk-ant-users-own-key", gotAPIKey)
	// Mako's own token must never leak upstream.
	require.Empty(t, gotMakoToken)
	// The peeked body must be replayed in full.
	require.Equal(t, body, gotBody)
}

func TestProxyRejectsUnauthenticatedRequest(t *testing.T) {
	ph := newTestProxy("http://127.0.0.1:1")

	rec := httptest.NewRecorder()
	ph.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/messages", nil))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "UNAUTHENTICATED")
}

func TestProxyBlocksRequestRejectedByPolicy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upstream must not be reached when policy denies the request")
	}))
	defer upstream.Close()

	ph := newTestProxy(upstream.URL)
	ph.policy = denyingPolicy()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages",
		strings.NewReader(`{"model":"claude-opus-4"}`))
	req = withUser(req)

	rec := httptest.NewRecorder()
	ph.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "MODEL_BLOCKED")
}

func denyingPolicy() *policy.Engine {
	return policy.NewEngineWithPolicy(&storage.Policy{
		TeamID:        "team-1",
		BlockedModels: []string{"claude-opus-4"},
	})
}
