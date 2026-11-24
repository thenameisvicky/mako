package auth

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/thenameisvicky/tokenly-mako/internal/encryption"
	"github.com/thenameisvicky/tokenly-mako/internal/storage"
	"github.com/thenameisvicky/tokenly-mako/pkg/models"
)

type AuthHandler struct {
	storage   *storage.Storage
	jwtSecret string
	encryptor *encryption.Encryptor
}

// NewAuthHandler constructs a new AuthHandler
func NewAuthHandler(store *storage.Storage, jwtSecret string, enc *encryption.Encryptor) *AuthHandler {
	return &AuthHandler{
		storage:   store,
		jwtSecret: jwtSecret,
		encryptor: enc,
	}
}

// HandleLogin authenticates a user, registering them first when isCreate is set,
// and returns a Mako token.
//
// Identity columns are AES-256-GCM encrypted, so the user is located by the blind
// index of their username rather than by the username itself.
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeLogin(w, http.StatusBadRequest, models.LoginResponse{Status: "error", Message: "Invalid payload"})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		writeLogin(w, http.StatusBadRequest, models.LoginResponse{Status: "error", Message: "Username and password are required"})
		return
	}

	ctx := r.Context()
	user, err := h.findUser(ctx, req.Username)
	if err != nil {
		log.Printf("[Auth] user lookup failed: %v", err)
		writeLogin(w, http.StatusInternalServerError, models.LoginResponse{Status: "error", Message: "Database query failed"})
		return
	}

	if user != nil {
		if !h.verifyPassword(ctx, user, req.Password) {
			writeLogin(w, http.StatusUnauthorized, models.LoginResponse{Status: "error", Message: "Invalid credentials"})
			return
		}
	} else {
		user, err = h.registerUser(ctx, req)
		if err != nil {
			status, message := registrationError(err)
			writeLogin(w, status, models.LoginResponse{Status: "error", Message: message})
			return
		}
	}

	token, err := GenerateToken(user.ID, user.TeamID, h.jwtSecret, 7*24*time.Hour)
	if err != nil {
		writeLogin(w, http.StatusInternalServerError, models.LoginResponse{Status: "error", Message: "Failed to generate token"})
		return
	}

	writeLogin(w, http.StatusOK, models.LoginResponse{Status: "success", Token: token})
}

// findUser resolves a user by blind index, falling back to the legacy plaintext
// column so rows written before identity encryption still log in.
func (h *AuthHandler) findUser(ctx context.Context, username string) (*storage.User, error) {
	user, err := h.storage.GetUserByUsernameBidx(ctx, h.encryptor.BlindIndex(username))
	if err != nil {
		return nil, err
	}
	if user != nil {
		return user, nil
	}
	return h.storage.GetUserByUsername(ctx, username)
}

// verifyPassword checks a candidate password against the stored value.
//
// Passwords are stored AES-256-GCM encrypted. Rows still holding a bcrypt hash
// from before that change are verified with bcrypt and then re-encrypted in
// place, so the migration needs no password reset and no downtime.
func (h *AuthHandler) verifyPassword(ctx context.Context, user *storage.User, candidate string) bool {
	if encryption.IsEncryptedField(user.Password) {
		return h.encryptor.VerifyPassword(user.Password, candidate)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(candidate)); err != nil {
		return false
	}

	if err := h.migrateIdentity(ctx, user, candidate); err != nil {
		// The user is authenticated either way; migration retries on next login.
		log.Printf("[Auth] identity migration failed for user %s: %v", user.ID, err)
	}
	return true
}

// migrateIdentity rewrites a legacy row's identity columns as AES-256-GCM
func (h *AuthHandler) migrateIdentity(ctx context.Context, user *storage.User, plaintextPassword string) error {
	plainUsername := user.Username
	if encryption.IsEncryptedField(user.Username) {
		if decoded, err := h.encryptor.DecryptField(user.Username); err == nil {
			plainUsername = decoded
		}
	}

	encUsername, err := h.encryptor.EncryptField(plainUsername)
	if err != nil {
		return err
	}
	encEmail, err := h.encryptor.EncryptField(user.Email)
	if err != nil {
		return err
	}
	encPassword, err := h.encryptor.EncryptField(plaintextPassword)
	if err != nil {
		return err
	}

	user.Username = encUsername
	user.UsernameBidx = h.encryptor.BlindIndex(plainUsername)
	user.Email = encEmail
	user.Password = encPassword

	return h.storage.UpdateUserIdentity(ctx, user)
}

// registerUser creates a new user with an encrypted identity
func (h *AuthHandler) registerUser(ctx context.Context, req models.LoginRequest) (*storage.User, error) {
	if !req.IsCreate {
		return nil, errInvalidCredentials
	}

	user, err := h.buildUser(req)
	if err != nil {
		return nil, err
	}

	// Starting a new team: no team id exists yet, so the caller supplies a name
	// and becomes that team's admin. This is how the first user of a deployment
	// signs up.
	if req.CreateTeam {
		name := strings.TrimSpace(req.TeamName)
		if name == "" {
			return nil, errTeamNameRequired
		}
		user.Role = "admin"
		if err := h.storage.CreateTeamWithUser(ctx, &storage.Team{Name: name}, user); err != nil {
			return nil, err
		}
		return user, nil
	}

	// Joining an existing team.
	if req.TeamID == "" {
		return nil, errTeamRequired
	}
	team, err := h.storage.GetTeamByID(ctx, req.TeamID)
	if err != nil || team == nil {
		return nil, errTeamNotFound
	}

	user.TeamID = req.TeamID
	if err := h.storage.CreateUser(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// buildUser encrypts the identity fields shared by both signup paths
func (h *AuthHandler) buildUser(req models.LoginRequest) (*storage.User, error) {
	encUsername, err := h.encryptor.EncryptField(req.Username)
	if err != nil {
		return nil, err
	}
	encEmail, err := h.encryptor.EncryptField(req.Username + "@example.com")
	if err != nil {
		return nil, err
	}
	encPassword, err := h.encryptor.EncryptField(req.Password)
	if err != nil {
		return nil, err
	}

	return &storage.User{
		ID:           uuid.New().String(),
		Username:     encUsername,
		UsernameBidx: h.encryptor.BlindIndex(req.Username),
		Email:        encEmail,
		Password:     encPassword,
		Role:         "member",
	}, nil
}

func writeLogin(w http.ResponseWriter, status int, body models.LoginResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// RegisterRoutes registers the authentication endpoints
func RegisterRoutes(mux *http.ServeMux, handler *AuthHandler) {
	mux.HandleFunc("/api/v1/login", handler.HandleLogin)
}
