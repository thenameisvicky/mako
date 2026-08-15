package proxy

import (
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/thenameisvicky/tokenly-gateway/internal/cache"
	"github.com/thenameisvicky/tokenly-gateway/internal/encryption"
	"github.com/thenameisvicky/tokenly-gateway/internal/requestctx"
	"github.com/thenameisvicky/tokenly-gateway/internal/storage"
)

// A pool of byte buffers to eliminate allocation overhead during payload copy
var bufPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 32*1024) // 32KB
	},
}

type ProxyHandler struct {
	storage     *storage.Storage
	cache       *cache.Cache
	encryptor   *encryption.Encryptor
	upstreamURL string
	httpClient  *http.Client
}

// NewProxyHandler creates a new proxy handler to forward requests to the configured upstream provider
func NewProxyHandler(store *storage.Storage, cache *cache.Cache, enc *encryption.Encryptor, upstreamURL string) *ProxyHandler {
	// Configure connection pooling for the outbound http client
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}

	return &ProxyHandler{
		storage:     store,
		cache:       cache,
		encryptor:   enc,
		upstreamURL: upstreamURL,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   60 * time.Second,
		},
	}
}

func (ph *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	state, hasState := requestctx.GetState(r.Context())
	if !hasState || state.User == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"status":"error","message":"Unauthenticated"}`))
		return
	}

	user := state.User

	provider := r.Header.Get("X-Provider")
	if provider == "" {
		provider = "anthropic"
	}

	if hasState {
		state.Provider = provider
	}

	var apiKeyBytes []byte
	var hit bool

	// Retrieve unencrypted credential from cache directly
	apiKeyBytes, hit = ph.cache.GetCredential(user.ID, provider)
	if hasState {
		state.CacheHit = hit
	}

	if !hit {
		// Cache miss: retrieve from DB
		cred, err := ph.storage.GetCredential(r.Context(), user.ID, provider)
		if err != nil {
			log.Printf("[Proxy] DB error fetching credentials: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"status":"error","message":"Internal database error"}`))
			return
		}
		if cred == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"status":"error","message":"Provider credential not set"}`))
			return
		}

		encVal := &encryption.EncryptedValue{
			Ciphertext: encryption.Ciphertext(cred.Ciphertext),
			Nonce:      encryption.Nonce(cred.Nonce),
		}

		apiKey, err := ph.encryptor.Decrypt(encVal)
		if err != nil {
			log.Printf("[Proxy] Decryption failed for user %s: %v", user.ID, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"status":"error","message":"Internal credential decryption error"}`))
			return
		}

		apiKeyBytes = []byte(apiKey)
		ph.cache.SetCredential(user.ID, provider, apiKeyBytes)
	}

	apiKeyStr := string(apiKeyBytes)

	path := r.URL.Path
	strippedPath := strings.TrimPrefix(path, "/api/v1")
	if !strings.HasPrefix(strippedPath, "/") {
		strippedPath = "/" + strippedPath
	}

	targetURL := strings.TrimSuffix(ph.upstreamURL, "/") + strippedPath

	req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"status":"error","message":"Failed to prepare outbound request"}`))
		return
	}

	// Copy headers excluding incoming authorization
	for k, vv := range r.Header {
		if strings.ToLower(k) == "authorization" {
			continue
		}
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}

	// Set destination authorization
	req.Header.Set("Authorization", "Bearer "+apiKeyStr)
	req.Header.Set("x-api-key", apiKeyStr)

	resp, err := ph.httpClient.Do(req)
	if err != nil {
		log.Printf("[Proxy] Destination request failed: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"status":"error","message":"Upstream provider unreachable"}`))
		return
	}
	defer resp.Body.Close()

	// Copy headers from response
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}

	w.WriteHeader(resp.StatusCode)

	// Stream response body using io.CopyBuffer with a pooled 32KB buffer
	buf := bufPool.Get().([]byte)
	defer bufPool.Put(buf)
	_, _ = io.CopyBuffer(w, resp.Body, buf)
}
