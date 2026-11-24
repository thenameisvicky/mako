package models

type LoginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	IsCreate   bool   `json:"isCreate"`
	TeamID     string `json:"teamId"`
	CreateTeam bool   `json:"createTeam"`
	TeamName   string `json:"teamName"`
}

type LoginResponse struct {
	Status  string `json:"status"`
	Token   string `json:"token,omitempty"`
	Message string `json:"message,omitempty"`
}

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
