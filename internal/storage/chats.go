package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ChatSession is one Claude Code transcript uploaded by /local-chat-sync.
//
// ProjectPath and Transcript hold AES-256-GCM ciphertext; the proxy decrypts
// them only when handing a session back to an authorised team member.
type ChatSession struct {
	ID            string    `json:"id"`
	SessionID     string    `json:"session_id"`
	UserID        string    `json:"user_id"`
	TeamID        string    `json:"team_id"`
	ProjectPath   string    `json:"project_path"`
	Title         string    `json:"title"`
	Transcript    string    `json:"transcript,omitempty"`
	MessageCount  int       `json:"message_count"`
	SizeBytes     int       `json:"size_bytes"`
	Checksum      string    `json:"checksum"`
	SyncedAt      time.Time `json:"synced_at"`
	FlagUseStatus int       `json:"-"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// UpsertChatSession inserts a transcript, or replaces it if this user has already
// synced that session. Re-running /local-chat-sync is therefore idempotent.
func (s *Storage) UpsertChatSession(ctx context.Context, c *ChatSession) error {
	now := time.Now()
	c.SyncedAt = now
	c.UpdatedAt = now
	c.FlagUseStatus = 1

	// Mongo equivalent: updateOne({user_id, session_id}, {$set: {...}}, {upsert: true})
	const query = `
		INSERT INTO chat_sessions
			(session_id, user_id, team_id, project_path, title, transcript,
			 message_count, size_bytes, checksum, synced_at, flag_use_status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 1, $11, $12)
		ON CONFLICT (user_id, session_id) WHERE flag_use_status = 1
		DO UPDATE SET
			project_path  = EXCLUDED.project_path,
			title         = EXCLUDED.title,
			transcript    = EXCLUDED.transcript,
			message_count = EXCLUDED.message_count,
			size_bytes    = EXCLUDED.size_bytes,
			checksum      = EXCLUDED.checksum,
			synced_at     = EXCLUDED.synced_at,
			updated_at    = EXCLUDED.updated_at
		RETURNING id`

	c.CreatedAt = now
	err := s.db.QueryRowContext(ctx, query,
		c.SessionID, c.UserID, c.TeamID, c.ProjectPath, c.Title, c.Transcript,
		c.MessageCount, c.SizeBytes, c.Checksum, c.SyncedAt, c.CreatedAt, c.UpdatedAt,
	).Scan(&c.ID)
	if err != nil {
		return fmt.Errorf("failed to upsert chat session: %w", err)
	}
	return nil
}

// ListChatSessionsForTeam returns transcript metadata for a team, newest first.
// The transcript body is deliberately excluded: listings stay small.
func (s *Storage) ListChatSessionsForTeam(ctx context.Context, teamID string, limit int) ([]ChatSession, error) {
	if limit <= 0 {
		limit = 100
	}

	qb := NewQueryBuilder().
		Select("id", "session_id", "user_id", "team_id", "project_path", "title",
			"message_count", "size_bytes", "checksum", "synced_at", "created_at", "updated_at").
		From("chat_sessions").
		Where("team_id", teamID).
		Where("flag_use_status", 1).
		OrderBy("synced_at DESC").
		Limit(limit)

	query, args, err := qb.Build()
	if err != nil {
		return nil, err
	}

	// SQL: SELECT ... FROM chat_sessions WHERE team_id = $1 AND flag_use_status = $2 ORDER BY synced_at DESC LIMIT n
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list chat sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]ChatSession, 0, limit)
	for rows.Next() {
		var c ChatSession
		if err := rows.Scan(&c.ID, &c.SessionID, &c.UserID, &c.TeamID, &c.ProjectPath, &c.Title,
			&c.MessageCount, &c.SizeBytes, &c.Checksum, &c.SyncedAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan chat session: %w", err)
		}
		sessions = append(sessions, c)
	}
	return sessions, rows.Err()
}

// GetChatSession fetches one transcript, scoped to the caller's team so a session
// can never be read across team boundaries.
func (s *Storage) GetChatSession(ctx context.Context, teamID, sessionID string) (*ChatSession, error) {
	qb := NewQueryBuilder().
		Select("id", "session_id", "user_id", "team_id", "project_path", "title", "transcript",
			"message_count", "size_bytes", "checksum", "synced_at", "created_at", "updated_at").
		From("chat_sessions").
		Where("team_id", teamID).
		Where("session_id", sessionID).
		Where("flag_use_status", 1)

	query, args, err := qb.Build()
	if err != nil {
		return nil, err
	}

	// SQL: SELECT ... FROM chat_sessions WHERE team_id = $1 AND session_id = $2 AND flag_use_status = $3
	var c ChatSession
	err = s.db.QueryRowContext(ctx, query, args...).Scan(
		&c.ID, &c.SessionID, &c.UserID, &c.TeamID, &c.ProjectPath, &c.Title, &c.Transcript,
		&c.MessageCount, &c.SizeBytes, &c.Checksum, &c.SyncedAt, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan chat session: %w", err)
	}
	return &c, nil
}
