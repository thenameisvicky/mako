package test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thenameisvicky/tokenly-gateway/internal/auth"
	"github.com/thenameisvicky/tokenly-gateway/internal/cache"
	"github.com/thenameisvicky/tokenly-gateway/internal/storage"
)

func TestJWTGenerationAndVerification(t *testing.T) {
	userID := uuid.New().String()
	companyID := uuid.New().String()

	// 1. Successful flow
	token, err := auth.GenerateToken(userID, companyID, testJWTSecret, 10*time.Second)
	require.NoError(t, err)

	gotUserID, gotCompanyID, err := auth.VerifyToken(token, testJWTSecret)
	require.NoError(t, err)
	assert.Equal(t, userID, gotUserID)
	assert.Equal(t, companyID, gotCompanyID)

	// 2. Expired token flow
	expiredToken, err := auth.GenerateToken(userID, companyID, testJWTSecret, -5*time.Second)
	require.NoError(t, err)

	_, _, err = auth.VerifyToken(expiredToken, testJWTSecret)
	assert.Equal(t, auth.ErrTokenExpired, err)

	// 3. Invalid signature
	_, _, err = auth.VerifyToken(token, "wrong-secret-key")
	assert.Equal(t, auth.ErrInvalidToken, err)
}

func TestMiddleware(t *testing.T) {
	ctx := context.Background()
	s, err := storage.New(testDBURL)
	require.NoError(t, err)
	defer s.Close()

	teamID := uuid.New().String()
	userID := uuid.New().String()

	// Seed User and Team
	_ = s.CreateTeam(ctx, &storage.Team{ID: teamID, Name: "Middleware Team"})
	_ = s.CreateUser(ctx, &storage.User{
		ID:       userID,
		Username: "mwuser_" + uuid.New().String()[:8],
		Email:    "mwuser_" + uuid.New().String()[:8] + "@example.com",
		Password: "password",
		TeamID:   teamID,
		Role:     "owner",
	})

	c := cache.New(5 * time.Minute)
	defer c.Close()
	m := auth.NewMiddleware(testJWTSecret, s, c)

	mux := http.NewServeMux()
	mux.Handle("/api/v1/protected", m.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})))

	// 1. Test missing Authorization header
	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	var resp map[string]string
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Missing or invalid token", resp["message"])

	// 2. Test valid token
	token, _ := auth.GenerateToken(userID, teamID, testJWTSecret, 10*time.Second)
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, req2)

	assert.Equal(t, http.StatusOK, rr2.Code)
	assert.Equal(t, "success", rr2.Body.String())
}
