package test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/thenameisvicky/tokenly-mako/internal/config"
	"github.com/thenameisvicky/tokenly-mako/internal/server"
	"github.com/thenameisvicky/tokenly-mako/internal/storage"
	"github.com/thenameisvicky/tokenly-mako/pkg/models"
)

// TestFullServerPipeline signs in, syncs a transcript, then lists and reads it back.
func TestFullServerPipeline(t *testing.T) {
	ctx := context.Background()

	db, err := storage.New(testDBURL)
	require.NoError(t, err)
	defer db.Close()

	teamID := uuid.New().String()
	userID := uuid.New().String()
	_ = db.CreateTeam(ctx, &storage.Team{ID: teamID, Name: "Chat Sync Team"})

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("mypassword"), 10)
	username := "pipeline_user_" + uuid.New().String()[:8]
	_ = db.CreateUser(ctx, &storage.User{
		ID:       userID,
		Username: username,
		Email:    username + "@example.com",
		Password: string(hashedPassword),
		TeamID:   teamID,
		Role:     "owner",
	})

	srv := server.New(config.Config{
		PORT:           "8081",
		DB_URL:         testDBURL,
		JWT_SECRET:     testJWTSecret,
		ENCRYPTION_KEY: testEncryptionKey,
		APP_BASE_URL:   "http://localhost:8081",
		Address:        ":8081",
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	defer func() { _ = srv.Close(ctx) }()

	// Health
	healthResp, err := http.Get(ts.URL + "/health")
	require.NoError(t, err)
	healthResp.Body.Close()
	assert.Equal(t, http.StatusOK, healthResp.StatusCode)

	// Login
	loginPayload, _ := json.Marshal(map[string]string{"username": username, "password": "mypassword"})
	resp, err := http.Post(ts.URL+"/api/v1/login", "application/json", bytes.NewBuffer(loginPayload))
	require.NoError(t, err)
	var login models.LoginResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&login))
	resp.Body.Close()
	require.Equal(t, "success", login.Status)
	require.NotEmpty(t, login.Token)

	call := func(method, path string, body any, out any) int {
		var buf bytes.Buffer
		if body != nil {
			require.NoError(t, json.NewEncoder(&buf).Encode(body))
		}
		req, err := http.NewRequest(method, ts.URL+path, &buf)
		require.NoError(t, err)
		req.Header.Set("X-Mako-Token", login.Token)
		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()
		require.NoError(t, json.NewDecoder(res.Body).Decode(out))
		return res.StatusCode
	}

	// Sync
	sessionID := uuid.New().String()
	transcript := `{"type":"user","message":{"content":"hello"}}` + "\n"
	var synced models.ChatSyncResponse
	status := call(http.MethodPost, "/api/v1/chats/sync", models.ChatSyncRequest{Sessions: []models.ChatSessionPayload{{
		SessionID: sessionID, ProjectPath: "/tmp/project", Title: "hello", Transcript: transcript, MessageCount: 1,
	}}}, &synced)
	require.Equal(t, http.StatusOK, status, synced.Message)

	// List
	var list models.ChatListResponse
	require.Equal(t, http.StatusOK, call(http.MethodGet, "/api/v1/chats", nil, &list))
	found := false
	for _, s := range list.Sessions {
		if s.SessionID == sessionID {
			found = true
			assert.True(t, s.Mine)
			assert.Equal(t, "/tmp/project", s.ProjectPath)
		}
	}
	assert.True(t, found, "synced session should be listed")

	// Get
	var got models.ChatGetResponse
	require.Equal(t, http.StatusOK, call(http.MethodGet, "/api/v1/chats/"+sessionID, nil, &got))
	assert.Equal(t, transcript, got.Session.Transcript)
}
