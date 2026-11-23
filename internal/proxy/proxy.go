package proxy

import (
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/thenameisvicky/tokenly-mako/internal/budget"
	"github.com/thenameisvicky/tokenly-mako/internal/policy"
	"github.com/thenameisvicky/tokenly-mako/internal/requestctx"
)

var bufPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 32*1024)
	},
}

// hopByHopHeaders are connection-scoped and must not be forwarded to the upstream.
var hopByHopHeaders = map[string]bool{
	"connection":          true,
	"keep-alive":          true,
	"proxy-authenticate":  true,
	"proxy-authorization": true,
	"te":                  true,
	"trailer":             true,
	"transfer-encoding":   true,
	"upgrade":             true,
}

// ProxyHandler forwards Claude Code traffic to the upstream provider.
//
// The proxy does NOT substitute credentials. The caller's own provider API key
// is forwarded exactly as received; Mako authenticates the caller separately
// via the X-Mako-Token header and applies policy and budgeting around the
// request. Keeping the two credentials distinct is what stops the proxy from
// becoming a way to share one account's access across many users.
type ProxyHandler struct {
	policy      *policy.Engine
	budget      *budget.Tracker
	upstreamURL string
	httpClient  *http.Client
}

// NewProxyHandler constructs the proxy with a shared, connection-pooled client
func NewProxyHandler(engine *policy.Engine, tracker *budget.Tracker, upstreamURL string) *ProxyHandler {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
		ForceAttemptHTTP2:   true,
	}

	return &ProxyHandler{
		policy:      engine,
		budget:      tracker,
		upstreamURL: upstreamURL,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   0, // streaming responses must not be cut off mid-flight
		},
	}
}

func (ph *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	state, hasState := requestctx.GetState(r.Context())
	if !hasState || state.User == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Unauthenticated")
		return
	}

	user := state.User

	// Inspect only the front of the body, then replay it untouched upstream.
	facts, body := policy.PeekBody(r.Body)
	if hasState {
		state.Provider = providerFor(r)
		state.Model = facts.Model
	}

	// Policy check - served from cache, no database work once warm.
	if decision := ph.policy.Evaluate(r.Context(), user.TeamID, facts); !decision.Allowed {
		writeJSONError(w, http.StatusForbidden, decision.Code, decision.Reason)
		return
	}

	// Budget check - atomic in-memory counter read.
	if decision := ph.budget.Check(r.Context(), user.TeamID, user.ID); !decision.Allowed {
		writeJSONError(w, http.StatusPaymentRequired, "BUDGET_EXCEEDED", decision.Reason)
		return
	}

	targetURL := strings.TrimSuffix(ph.upstreamURL, "/") + strippedPath(r.URL.Path)
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, body)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "BAD_OUTBOUND_REQUEST", "Failed to prepare outbound request")
		return
	}

	// Forward the caller's headers verbatim, including their own provider
	// credential. Only Mako's own auth header and hop-by-hop headers are
	// stripped - the provider key is never read, logged, or replaced.
	for k, vv := range r.Header {
		lower := strings.ToLower(k)
		if hopByHopHeaders[lower] || lower == "x-mako-token" {
			continue
		}
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}
	req.ContentLength = r.ContentLength

	resp, err := ph.httpClient.Do(req)
	if err != nil {
		log.Printf("[Proxy] upstream request failed: %v", err)
		writeJSONError(w, http.StatusBadGateway, "UPSTREAM_UNREACHABLE", "Upstream provider unreachable")
		return
	}
	defer resp.Body.Close()

	for k, vv := range resp.Header {
		if hopByHopHeaders[strings.ToLower(k)] {
			continue
		}
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Stream the response back, counting tokens as the bytes go past, and flush
	// each chunk so streaming responses stay interactive.
	sniffer := newUsageSniffer(&flushWriter{w: w})
	buf := bufPool.Get().([]byte)
	if _, err := io.CopyBuffer(sniffer, resp.Body, buf); err != nil {
		log.Printf("[Proxy] response copy interrupted: %v", err)
	}
	bufPool.Put(buf)

	// Usage is queued, never written inline - the request does not wait on the DB.
	ph.budget.Record(user.TeamID, user.ID, facts.Model, sniffer.inputTokens, sniffer.outputTokens)
	if hasState {
		state.InputTokens = sniffer.inputTokens
		state.OutputTokens = sniffer.outputTokens
	}
}

// flushWriter pushes each chunk to the client immediately, so server-sent events
// are not held back by the response buffer.
type flushWriter struct {
	w http.ResponseWriter
}

func (fw *flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if f, ok := fw.w.(http.Flusher); ok {
		f.Flush()
	}
	return n, err
}

// strippedPath maps a proxy path onto the upstream provider's path
func strippedPath(path string) string {
	var stripped string
	switch {
	case strings.HasPrefix(path, "/api/v1/proxy"):
		stripped = strings.TrimPrefix(path, "/api/v1/proxy")
	case strings.HasPrefix(path, "/api/"):
		stripped = strings.TrimPrefix(path, "/api")
	default:
		stripped = path
	}
	if !strings.HasPrefix(stripped, "/") {
		stripped = "/" + stripped
	}
	return stripped
}

func providerFor(r *http.Request) string {
	if p := r.Header.Get("X-Provider"); p != "" {
		return p
	}
	return "anthropic"
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"status":"error","code":"` + code + `","message":"` + message + `"}`))
}
