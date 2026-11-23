package models

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	IsCreate bool   `json:"isCreate"`
	TeamID string `json:"teamId"`
	CreateTeam bool   `json:"createTeam"`
	TeamName   string `json:"teamName"`
}

type LoginResponse struct {
	Status  string `json:"status"`
	Token   string `json:"token,omitempty"`
	Message string `json:"message,omitempty"`
}
