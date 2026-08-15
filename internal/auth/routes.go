package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/thenameisvicky/tokenly-gateway/internal/storage"
	"github.com/thenameisvicky/tokenly-gateway/pkg/models"
)

type AuthHandler struct {
	storage   *storage.Storage
	jwtSecret string
}

// NewAuthHandler constructs a new AuthHandler
func NewAuthHandler(store *storage.Storage, jwtSecret string) *AuthHandler {
	return &AuthHandler{
		storage:   store,
		jwtSecret: jwtSecret,
	}
}

// HandleLogin processes user login requests, authenticating using bcrypt and issuing a JWT
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(models.LoginResponse{Status: "error", Message: "Invalid payload"})
		return
	}

	user, err := h.storage.GetUserByUsername(r.Context(), req.Username)
	if err != nil || user == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(models.LoginResponse{Status: "error", Message: "Invalid credentials"})
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(models.LoginResponse{Status: "error", Message: "Invalid credentials"})
		return
	}

	token, err := GenerateToken(user.ID, user.TeamID, h.jwtSecret, 7*24*time.Hour)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(models.LoginResponse{Status: "error", Message: "Failed to generate token"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(models.LoginResponse{Status: "success", Token: token})
}

// RegisterRoutes registers the authentication endpoints
func RegisterRoutes(mux *http.ServeMux, handler *AuthHandler) {
	mux.HandleFunc("/api/v1/login", handler.HandleLogin)
}
