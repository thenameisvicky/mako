package chats

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/thenameisvicky/tokenly-mako/internal/encryption"
	"github.com/thenameisvicky/tokenly-mako/internal/requestctx"
	"github.com/thenameisvicky/tokenly-mako/internal/storage"
	"github.com/thenameisvicky/tokenly-mako/pkg/models"
)

// maxTranscriptBytes caps a single uploaded transcript. Claude Code sessions are
// typically well under 2MB; the cap stops one machine filling the database.
const maxTranscriptBytes = 32 << 20 // 32MB

type Handler struct {
	storage   *storage.Storage
	encryptor *encryption.Encryptor
}

// NewHandler constructs the chat sync handler
func NewHandler(store *storage.Storage, enc *encryption.Encryptor) *Handler {
	return &Handler{storage: store, encryptor: enc}
}

// HandleSync accepts a batch of local Claude Code transcripts and stores them
// encrypted, scoped to the caller's team.
func (h *Handler) HandleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	user, ok := currentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.ChatSyncResponse{Status: "error", Message: "Unauthenticated"})
		return
	}

	var req models.ChatSyncRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxTranscriptBytes)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ChatSyncResponse{Status: "error", Message: "Invalid payload"})
		return
	}

	if len(req.Sessions) == 0 {
		writeJSON(w, http.StatusBadRequest, models.ChatSyncResponse{Status: "error", Message: "No sessions supplied"})
		return
	}

	var synced int
	for _, s := range req.Sessions {
		if s.SessionID == "" || s.Transcript == "" {
			continue
		}

		// Encrypt the transcript and the originating path before they touch the DB.
		encTranscript, err := h.encryptor.EncryptField(s.Transcript)
		if err != nil {
			log.Printf("[Chats] failed to encrypt transcript for session %s: %v", s.SessionID, err)
			continue
		}
		encPath, err := h.encryptor.EncryptField(s.ProjectPath)
		if err != nil {
			log.Printf("[Chats] failed to encrypt project path for session %s: %v", s.SessionID, err)
			continue
		}

		record := &storage.ChatSession{
			SessionID:    s.SessionID,
			UserID:       user.ID,
			TeamID:       user.TeamID,
			ProjectPath:  encPath,
			Title:        s.Title,
			Transcript:   encTranscript,
			MessageCount: s.MessageCount,
			SizeBytes:    len(s.Transcript),
			Checksum:     s.Checksum,
		}

		if err := h.storage.UpsertChatSession(r.Context(), record); err != nil {
			log.Printf("[Chats] failed to store session %s: %v", s.SessionID, err)
			continue
		}
		synced++
	}

	writeJSON(w, http.StatusOK, models.ChatSyncResponse{
		Status: "success",
		Synced: synced,
		Total:  len(req.Sessions),
	})
}

// HandleList returns transcript metadata for the caller's team, newest first.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	user, ok := currentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.ChatListResponse{Status: "error", Message: "Unauthenticated"})
		return
	}

	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	sessions, err := h.storage.ListChatSessionsForTeam(r.Context(), user.TeamID, limit)
	if err != nil {
		log.Printf("[Chats] failed to list sessions: %v", err)
		writeJSON(w, http.StatusInternalServerError, models.ChatListResponse{Status: "error", Message: "Failed to list sessions"})
		return
	}

	out := make([]models.ChatSessionSummary, 0, len(sessions))
	for _, s := range sessions {
		projectPath, err := h.encryptor.DecryptField(s.ProjectPath)
		if err != nil {
			projectPath = "(unreadable)"
		}
		out = append(out, models.ChatSessionSummary{
			SessionID:    s.SessionID,
			ProjectPath:  projectPath,
			Title:        s.Title,
			MessageCount: s.MessageCount,
			SizeBytes:    s.SizeBytes,
			Checksum:     s.Checksum,
			SyncedAt:     s.SyncedAt,
			Mine:         s.UserID == user.ID,
		})
	}

	writeJSON(w, http.StatusOK, models.ChatListResponse{Status: "success", Sessions: out})
}

// HandleGet returns one full transcript, decrypted, for import on another machine.
func (h *Handler) HandleGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	user, ok := currentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.ChatGetResponse{Status: "error", Message: "Unauthenticated"})
		return
	}

	sessionID := strings.TrimPrefix(r.URL.Path, "/api/v1/chats/")
	if sessionID == "" || strings.Contains(sessionID, "/") {
		writeJSON(w, http.StatusBadRequest, models.ChatGetResponse{Status: "error", Message: "Invalid session id"})
		return
	}

	// Scoped by team, so a session can never be read across team boundaries.
	session, err := h.storage.GetChatSession(r.Context(), user.TeamID, sessionID)
	if err != nil {
		log.Printf("[Chats] failed to fetch session %s: %v", sessionID, err)
		writeJSON(w, http.StatusInternalServerError, models.ChatGetResponse{Status: "error", Message: "Failed to fetch session"})
		return
	}
	if session == nil {
		writeJSON(w, http.StatusNotFound, models.ChatGetResponse{Status: "error", Message: "Session not found"})
		return
	}

	transcript, err := h.encryptor.DecryptField(session.Transcript)
	if err != nil {
		log.Printf("[Chats] failed to decrypt session %s: %v", sessionID, err)
		writeJSON(w, http.StatusInternalServerError, models.ChatGetResponse{Status: "error", Message: "Failed to decrypt session"})
		return
	}
	projectPath, err := h.encryptor.DecryptField(session.ProjectPath)
	if err != nil {
		projectPath = ""
	}

	writeJSON(w, http.StatusOK, models.ChatGetResponse{
		Status: "success",
		Session: models.ChatSessionPayload{
			SessionID:    session.SessionID,
			ProjectPath:  projectPath,
			Title:        session.Title,
			Transcript:   transcript,
			MessageCount: session.MessageCount,
			Checksum:     session.Checksum,
		},
	})
}

func currentUser(r *http.Request) (*storage.User, bool) {
	state, ok := requestctx.GetState(r.Context())
	if !ok || state.User == nil {
		return nil, false
	}
	return state.User, true
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// RegisterRoutes registers the chat sync endpoints behind the auth middleware
func RegisterRoutes(mux *http.ServeMux, handler *Handler, authenticate func(http.Handler) http.Handler) {
	mux.Handle("/api/v1/chats/sync", authenticate(http.HandlerFunc(handler.HandleSync)))
	mux.Handle("/api/v1/chats", authenticate(http.HandlerFunc(handler.HandleList)))
	mux.Handle("/api/v1/chats/", authenticate(http.HandlerFunc(handler.HandleGet)))
}
