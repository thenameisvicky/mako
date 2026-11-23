// Package governance exposes the configuration surface the web dashboard needs:
// the caller's identity, the team's policy and budget, and a usage rollup.
//
// These are control-plane endpoints. None of them run on the proxy hot path, so
// they read and write the database directly. Policy writes invalidate the
// engine's cache so a change takes effect on the next proxied request.
package governance

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"strings"

	"github.com/thenameisvicky/tokenly-mako/internal/encryption"
	"github.com/thenameisvicky/tokenly-mako/internal/policy"
	"github.com/thenameisvicky/tokenly-mako/internal/requestctx"
	"github.com/thenameisvicky/tokenly-mako/internal/storage"
	"github.com/thenameisvicky/tokenly-mako/pkg/models"
)

type Handler struct {
	storage   *storage.Storage
	encryptor *encryption.Encryptor
	policy    *policy.Engine
}

// NewHandler constructs the governance handler
func NewHandler(store *storage.Storage, enc *encryption.Encryptor, engine *policy.Engine) *Handler {
	return &Handler{storage: store, encryptor: enc, policy: engine}
}

// HandleMe returns the authenticated user's identity and team.
// Usernames are encrypted at rest, so they are decrypted for display here.
func (h *Handler) HandleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	user, ok := currentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.MeResponse{Status: "error", Message: "Unauthenticated"})
		return
	}

	team, err := h.storage.GetTeamForUser(r.Context(), user.ID)
	if err != nil {
		log.Printf("[Governance] failed to load team: %v", err)
	}

	me := models.Me{
		UserID:   user.ID,
		Username: h.display(user.Username),
		Email:    h.display(user.Email),
		TeamID:   user.TeamID,
		Role:     user.Role,
	}
	if team != nil {
		me.TeamName = team.Name
	}

	writeJSON(w, http.StatusOK, models.MeResponse{Status: "success", Me: me})
}

// HandlePolicy reads (GET) or replaces (PUT) the team's policy.
func (h *Handler) HandlePolicy(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.PolicyResponse{Status: "error", Message: "Unauthenticated"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		p, err := h.storage.GetPolicyForTeam(r.Context(), user.TeamID)
		if err != nil {
			log.Printf("[Governance] failed to read policy: %v", err)
			writeJSON(w, http.StatusInternalServerError, models.PolicyResponse{Status: "error", Message: "Failed to read policy"})
			return
		}
		writeJSON(w, http.StatusOK, models.PolicyResponse{Status: "success", Policy: toPolicyPayload(p)})

	case http.MethodPut:
		var req models.PolicyPayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, models.PolicyResponse{Status: "error", Message: "Invalid payload"})
			return
		}
		if req.MaxTokensPerRequest < 0 {
			writeJSON(w, http.StatusBadRequest, models.PolicyResponse{Status: "error", Message: "maxTokensPerRequest cannot be negative"})
			return
		}

		record := &storage.Policy{
			TeamID:              user.TeamID,
			AllowedModels:       cleanList(req.AllowedModels),
			BlockedModels:       cleanList(req.BlockedModels),
			MaxTokensPerRequest: req.MaxTokensPerRequest,
		}
		if err := h.storage.UpsertPolicy(r.Context(), record); err != nil {
			log.Printf("[Governance] failed to write policy: %v", err)
			writeJSON(w, http.StatusInternalServerError, models.PolicyResponse{Status: "error", Message: "Failed to save policy"})
			return
		}

		// Drop the cached copy so the change applies to the next proxied request.
		h.policy.Invalidate(user.TeamID)

		writeJSON(w, http.StatusOK, models.PolicyResponse{Status: "success", Policy: toPolicyPayload(record)})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// HandleBudget reads (GET) or replaces (PUT) the team-wide token budget.
func (h *Handler) HandleBudget(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.BudgetResponse{Status: "error", Message: "Unauthenticated"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		b, err := h.storage.GetBudgetForTeam(r.Context(), user.TeamID)
		if err != nil {
			log.Printf("[Governance] failed to read budget: %v", err)
			writeJSON(w, http.StatusInternalServerError, models.BudgetResponse{Status: "error", Message: "Failed to read budget"})
			return
		}
		writeJSON(w, http.StatusOK, models.BudgetResponse{Status: "success", Budget: toBudgetPayload(b)})

	case http.MethodPut:
		var req models.BudgetPayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, models.BudgetResponse{Status: "error", Message: "Invalid payload"})
			return
		}
		if req.TokenLimit < 0 {
			writeJSON(w, http.StatusBadRequest, models.BudgetResponse{Status: "error", Message: "tokenLimit cannot be negative"})
			return
		}
		if !validPeriod(req.Period) {
			writeJSON(w, http.StatusBadRequest, models.BudgetResponse{Status: "error", Message: "period must be daily, weekly or monthly"})
			return
		}

		record := &storage.Budget{
			TeamID:     user.TeamID,
			Period:     req.Period,
			TokenLimit: req.TokenLimit,
		}
		if err := h.storage.UpsertBudget(r.Context(), record); err != nil {
			log.Printf("[Governance] failed to write budget: %v", err)
			writeJSON(w, http.StatusInternalServerError, models.BudgetResponse{Status: "error", Message: "Failed to save budget"})
			return
		}

		writeJSON(w, http.StatusOK, models.BudgetResponse{Status: "success", Budget: toBudgetPayload(record)})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// HandleUsage returns the team's token spend for the current budget period.
func (h *Handler) HandleUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	user, ok := currentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.UsageResponse{Status: "error", Message: "Unauthenticated"})
		return
	}

	period := "monthly"
	if b, err := h.storage.GetBudgetForTeam(r.Context(), user.TeamID); err == nil && b != nil && b.Period != "" {
		period = b.Period
	}

	usage, err := h.storage.GetTeamUsage(r.Context(), user.TeamID, periodStart(period))
	if err != nil {
		log.Printf("[Governance] failed to read usage: %v", err)
		writeJSON(w, http.StatusInternalServerError, models.UsageResponse{Status: "error", Message: "Failed to read usage"})
		return
	}

	writeJSON(w, http.StatusOK, models.UsageResponse{
		Status: "success",
		Period: period,
		Usage:  *usage,
	})
}

// display decrypts an encrypted identity column, falling back to the raw value
// for rows written before identity encryption.
func (h *Handler) display(value string) string {
	if !encryption.IsEncryptedField(value) {
		return value
	}
	plaintext, err := h.encryptor.DecryptField(value)
	if err != nil {
		return ""
	}
	return plaintext
}

func toPolicyPayload(p *storage.Policy) models.PolicyPayload {
	if p == nil {
		// No policy configured means everything is allowed.
		return models.PolicyPayload{AllowedModels: []string{}, BlockedModels: []string{}}
	}
	return models.PolicyPayload{
		AllowedModels:       orEmpty(p.AllowedModels),
		BlockedModels:       orEmpty(p.BlockedModels),
		MaxTokensPerRequest: p.MaxTokensPerRequest,
		Configured:          true,
		UpdatedAt:           p.UpdatedAt,
	}
}

func toBudgetPayload(b *storage.Budget) models.BudgetPayload {
	if b == nil {
		return models.BudgetPayload{Period: "monthly"}
	}
	return models.BudgetPayload{
		Period:     b.Period,
		TokenLimit: b.TokenLimit,
		Configured: true,
		UpdatedAt:  b.UpdatedAt,
	}
}

// cleanList trims blanks out of a model list so an empty text box does not
// become a rule matching the empty string.
func cleanList(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func orEmpty(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func validPeriod(p string) bool {
	return p == "daily" || p == "weekly" || p == "monthly"
}

// periodStart mirrors the budget tracker's period boundaries
func periodStart(period string) time.Time {
	now := time.Now()
	switch period {
	case "daily":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "weekly":
		return now.AddDate(0, 0, -int(now.Weekday()))
	default:
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	}
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

// RegisterRoutes registers the governance endpoints behind the auth middleware
func RegisterRoutes(mux *http.ServeMux, handler *Handler, authenticate func(http.Handler) http.Handler) {
	mux.Handle("/api/v1/me", authenticate(http.HandlerFunc(handler.HandleMe)))
	mux.Handle("/api/v1/policy", authenticate(http.HandlerFunc(handler.HandlePolicy)))
	mux.Handle("/api/v1/budget", authenticate(http.HandlerFunc(handler.HandleBudget)))
	mux.Handle("/api/v1/usage", authenticate(http.HandlerFunc(handler.HandleUsage)))
}
