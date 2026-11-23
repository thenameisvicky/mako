package models

import (
	"time"

	"github.com/thenameisvicky/tokenly-mako/internal/storage"
)

// Me is the authenticated user's identity, decrypted for display.
type Me struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Email    string `json:"email"`
	TeamID   string `json:"teamId"`
	TeamName string `json:"teamName"`
	Role     string `json:"role"`
}

type MeResponse struct {
	Status  string `json:"status"`
	Me      Me     `json:"me,omitempty"`
	Message string `json:"message,omitempty"`
}

// PolicyPayload is a team's policy as the dashboard sees it.
// Configured is false when the team has no policy row, meaning "allow all".
type PolicyPayload struct {
	AllowedModels       []string  `json:"allowedModels"`
	BlockedModels       []string  `json:"blockedModels"`
	MaxTokensPerRequest int       `json:"maxTokensPerRequest"`
	Configured          bool      `json:"configured"`
	UpdatedAt           time.Time `json:"updatedAt,omitempty"`
}

type PolicyResponse struct {
	Status  string        `json:"status"`
	Policy  PolicyPayload `json:"policy"`
	Message string        `json:"message,omitempty"`
}

// BudgetPayload is a team's token budget. TokenLimit 0 means unlimited.
type BudgetPayload struct {
	Period     string    `json:"period"`
	TokenLimit int64     `json:"tokenLimit"`
	Configured bool      `json:"configured"`
	UpdatedAt  time.Time `json:"updatedAt,omitempty"`
}

type BudgetResponse struct {
	Status  string        `json:"status"`
	Budget  BudgetPayload `json:"budget"`
	Message string        `json:"message,omitempty"`
}

type UsageResponse struct {
	Status  string            `json:"status"`
	Period  string            `json:"period"`
	Usage   storage.TeamUsage `json:"usage"`
	Message string            `json:"message,omitempty"`
}
