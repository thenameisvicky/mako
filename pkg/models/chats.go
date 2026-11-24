package models

import "time"

// ChatSessionPayload is one Claude Code transcript moving over the wire.
type ChatSessionPayload struct {
	SessionID    string `json:"sessionId"`
	ProjectPath  string `json:"projectPath"`
	Title        string `json:"title"`
	Transcript   string `json:"transcript"`
	MessageCount int    `json:"messageCount"`
	Checksum     string `json:"checksum"`
}

type ChatSyncRequest struct {
	Sessions []ChatSessionPayload `json:"sessions"`
}

type ChatSyncResponse struct {
	Status  string `json:"status"`
	Synced  int    `json:"synced,omitempty"`
	Total   int    `json:"total,omitempty"`
	Message string `json:"message,omitempty"`
}

// ChatSessionSummary is transcript metadata without the transcript body.
type ChatSessionSummary struct {
	SessionID    string `json:"sessionId"`
	ProjectPath  string `json:"projectPath"`
	Title        string `json:"title"`
	MessageCount int    `json:"messageCount"`
	SizeBytes    int    `json:"sizeBytes"`
	// Checksum lets a client skip re-downloading a transcript it already has.
	Checksum string    `json:"checksum"`
	SyncedAt time.Time `json:"syncedAt"`
	Mine     bool      `json:"mine"`
}

type ChatListResponse struct {
	Status   string               `json:"status"`
	Sessions []ChatSessionSummary `json:"sessions,omitempty"`
	Message  string               `json:"message,omitempty"`
}

type ChatGetResponse struct {
	Status  string             `json:"status"`
	Session ChatSessionPayload `json:"session,omitempty"`
	Message string             `json:"message,omitempty"`
}
