package test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/thenameisvicky/tokenly-gateway/internal/auth"
	"github.com/thenameisvicky/tokenly-gateway/internal/config"
	"github.com/thenameisvicky/tokenly-gateway/internal/encryption"
	"github.com/thenameisvicky/tokenly-gateway/internal/server"
	"github.com/thenameisvicky/tokenly-gateway/internal/storage"
	"github.com/thenameisvicky/tokenly-gateway/pkg/models"
)

func TestFullServerPipeline(t *testing.T) {
	ctx := context.Background()

	// 1. Create mock upstream provider server
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		apiKeyHeader := r.Header.Get("x-api-key")

		if authHeader != "Bearer my-secret-anthropic-key" || apiKeyHeader != "my-secret-anthropic-key" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("forbidden mock upstream"))
			return
		}

		if r.URL.Path != "/proxy" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"mock":"response"}`))
	}))
	defer mockUpstream.Close()

	// 2. Setup database records (User, Team, and Credential)
	db, err := storage.New(testDBURL)
	require.NoError(t, err)
	defer db.Close()

	teamID := uuid.New().String()
	userID := uuid.New().String()

	_ = db.CreateTeam(ctx, &storage.Team{ID: teamID, Name: "Proxy Pipeline Team"})

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("mypassword"), 10)
	_ = db.CreateUser(ctx, &storage.User{
		ID:       userID,
		Username: "pipeline_user_" + uuid.New().String()[:8],
		Email:    "pipeline_user_" + uuid.New().String()[:8] + "@example.com",
		Password: string(hashedPassword),
		TeamID:   teamID,
		Role:     "owner",
	})

	// Encrypt the Anthropic API key
	enc, _ := encryption.New(testEncryptionKey)
	encValue, err := enc.Encrypt("my-secret-anthropic-key")
	require.NoError(t, err)

	_ = db.CreateCredential(ctx, &storage.Credential{
		ID:         uuid.New().String(),
		UserID:     userID,
		TeamID:     teamID,
		Provider:   "anthropic",
		Ciphertext: string(encValue.Ciphertext),
		Nonce:      string(encValue.Nonce),
	})

	// 3. Initialize Gateway Server pointing to mock upstream
	cfg := config.Config{
		PORT:                  "8081",
		DB_URL:                testDBURL,
		JWT_SECRET:            testJWTSecret,
		ENCRYPTION_KEY:        testEncryptionKey,
		APP_BASE_URL:          "http://localhost:8081",
		UPSTREAM_PROVIDER_URL: mockUpstream.URL,
		Address:               ":8081",
	}

	srv := server.New(cfg)
	testGatewayServer := httptest.NewServer(srv.Handler())
	defer testGatewayServer.Close()
	defer func() { _ = srv.Close(ctx) }()

	// 4. Test login to get a Tokenly JWT
	dbUser, err := db.GetUserByID(ctx, userID)
	require.NoError(t, err)
	require.NotNil(t, dbUser)

	loginPayload, _ := json.Marshal(map[string]string{
		"username": dbUser.Username,
		"password": "mypassword",
	})

	resp, err := http.Post(testGatewayServer.URL+"/api/v1/login", "application/json", bytes.NewBuffer(loginPayload))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var loginResp models.LoginResponse
	err = json.NewDecoder(resp.Body).Decode(&loginResp)
	require.NoError(t, err)
	assert.Equal(t, "success", loginResp.Status)
	assert.NotEmpty(t, loginResp.Token)

	token := loginResp.Token

	// 5. Query /health to check if tracker works
	healthResp, err := http.Get(testGatewayServer.URL + "/health")
	require.NoError(t, err)
	defer healthResp.Body.Close()
	assert.Equal(t, http.StatusOK, healthResp.StatusCode)

	// 6. Access proxy endpoint using Tokenly JWT
	req, err := http.NewRequest(http.MethodPost, testGatewayServer.URL+"/api/v1/proxy", bytes.NewBuffer([]byte(`{"messages":[]}`)))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 5 * time.Second}
	proxyResp, err := client.Do(req)
	require.NoError(t, err)
	defer proxyResp.Body.Close()

	assert.Equal(t, http.StatusOK, proxyResp.StatusCode)

	respBody, err := io.ReadAll(proxyResp.Body)
	require.NoError(t, err)
	var upstreamJSON map[string]string
	err = json.Unmarshal(respBody, &upstreamJSON)
	require.NoError(t, err)
	assert.Equal(t, "response", upstreamJSON["mock"])
}

func BenchmarkProxyPipeline(b *testing.B) {
	ctx := context.Background()

	// 1. Setup mock upstream
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer mockUpstream.Close()

	// 2. Setup database records
	db, err := storage.New(testDBURL)
	if err != nil {
		b.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()

	teamID := uuid.New().String()
	userID := uuid.New().String()

	_ = db.CreateTeam(ctx, &storage.Team{ID: teamID, Name: "Benchmark Team"})

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password"), 10)
	_ = db.CreateUser(ctx, &storage.User{
		ID:       userID,
		Username: "bench_user_" + uuid.New().String()[:8],
		Email:    "bench_user_" + uuid.New().String()[:8] + "@example.com",
		Password: string(hashedPassword),
		TeamID:   teamID,
		Role:     "owner",
	})

	enc, _ := encryption.New(testEncryptionKey)
	encValue, _ := enc.Encrypt("my-secret-anthropic-key")

	_ = db.CreateCredential(ctx, &storage.Credential{
		ID:         uuid.New().String(),
		UserID:     userID,
		TeamID:     teamID,
		Provider:   "anthropic",
		Ciphertext: string(encValue.Ciphertext),
		Nonce:      string(encValue.Nonce),
	})

	cfg := config.Config{
		PORT:                  "8082",
		DB_URL:                testDBURL,
		JWT_SECRET:            testJWTSecret,
		ENCRYPTION_KEY:        testEncryptionKey,
		APP_BASE_URL:          "http://localhost:8082",
		UPSTREAM_PROVIDER_URL: mockUpstream.URL,
		Address:               ":8082",
	}

	srv := server.New(cfg)
	defer func() { _ = srv.Close(ctx) }()

	testGatewayServer := httptest.NewServer(srv.Handler())
	defer testGatewayServer.Close()

	token, _ := auth.GenerateToken(userID, teamID, testJWTSecret, 1*time.Hour)

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost, testGatewayServer.URL+"/api/v1/proxy", bytes.NewBuffer([]byte(`{"messages":[]}`)))
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		if err != nil {
			b.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			b.Fatalf("bad status: %d %s", resp.StatusCode, string(body))
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
