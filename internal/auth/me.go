package auth

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/thenameisvicky/tokenly-mako/internal/encryption"
	"github.com/thenameisvicky/tokenly-mako/internal/requestctx"
	"github.com/thenameisvicky/tokenly-mako/pkg/models"
)

// HandleMe returns the authenticated user's identity and team, used by the
// dashboard header. Usernames are encrypted at rest and decrypted for display.
func (h *AuthHandler) HandleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	state, ok := requestctx.GetState(r.Context())
	if !ok || state.User == nil {
		writeMe(w, http.StatusUnauthorized, models.MeResponse{Status: "error", Message: "Unauthenticated"})
		return
	}
	user := state.User

	me := models.Me{
		UserID:   user.ID,
		Username: h.display(user.Username),
		Email:    h.display(user.Email),
		TeamID:   user.TeamID,
		Role:     user.Role,
	}
	if team, err := h.storage.GetTeamForUser(r.Context(), user.ID); err != nil {
		log.Printf("[Auth] failed to load team: %v", err)
	} else if team != nil {
		me.TeamName = team.Name
	}

	writeMe(w, http.StatusOK, models.MeResponse{Status: "success", Me: me})
}

// display decrypts an encrypted identity field, passing legacy plaintext through.
func (h *AuthHandler) display(value string) string {
	if !encryption.IsEncryptedField(value) {
		return value
	}
	plaintext, err := h.encryptor.DecryptField(value)
	if err != nil {
		return ""
	}
	return plaintext
}

func writeMe(w http.ResponseWriter, status int, body models.MeResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
