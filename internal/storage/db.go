package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type Team struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	FlagUseStatus int       `json:"flag_use_status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type User struct {
	ID            string    `json:"id"`
	Username      string    `json:"username"`
	Email         string    `json:"email"`
	Password      string    `json:"-"` // hashed password
	TeamID        string    `json:"team_id"`
	Role          string    `json:"role"`
	IsActive      bool      `json:"is_active"`
	FlagUseStatus int       `json:"flag_use_status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Credential struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	TeamID        string    `json:"team_id"`
	Provider      string    `json:"provider"`
	Ciphertext    string    `json:"ciphertext"`
	Nonce         string    `json:"nonce"`
	FlagUseStatus int       `json:"flag_use_status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Storage struct {
	db *sql.DB
}

// New initializes the database connection pool optimized for low-latency
func New(dbURL string) (*Storage, error) {
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Optimize connection pool settings for hot path
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Storage{db: db}, nil
}

// Close closes the database connection
func (s *Storage) Close() error {
	return s.db.Close()
}

// GetUserByID retrieves a user by ID if active
func (s *Storage) GetUserByID(ctx context.Context, id string) (*User, error) {
	qb := NewQueryBuilder().
		Select("id", "username", "email", "password", "team_id", "role", "is_active", "flag_use_status", "created_at", "updated_at").
		From("users").
		Where("id", id).
		Where("flag_use_status", 1)

	query, args, err := qb.Build()
	if err != nil {
		return nil, err
	}

	// SQL: SELECT id, username, email, password, team_id, role, is_active, flag_use_status, created_at, updated_at FROM users WHERE id = $1 AND flag_use_status = $2
	row := s.db.QueryRowContext(ctx, query, args...)

	var u User
	err = row.Scan(&u.ID, &u.Username, &u.Email, &u.Password, &u.TeamID, &u.Role, &u.IsActive, &u.FlagUseStatus, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}
	return &u, nil
}

// GetUserByUsername retrieves a user by username
func (s *Storage) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	qb := NewQueryBuilder().
		Select("id", "username", "email", "password", "team_id", "role", "is_active", "flag_use_status", "created_at", "updated_at").
		From("users").
		Where("username", username).
		Where("flag_use_status", 1)

	query, args, err := qb.Build()
	if err != nil {
		return nil, err
	}

	// SQL: SELECT id, username, email, password, team_id, role, is_active, flag_use_status, created_at, updated_at FROM users WHERE username = $1 AND flag_use_status = $2
	row := s.db.QueryRowContext(ctx, query, args...)

	var u User
	err = row.Scan(&u.ID, &u.Username, &u.Email, &u.Password, &u.TeamID, &u.Role, &u.IsActive, &u.FlagUseStatus, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan user by username: %w", err)
	}
	return &u, nil
}

// GetCredential retrieves a credential by userID and provider
func (s *Storage) GetCredential(ctx context.Context, userID string, provider string) (*Credential, error) {
	qb := NewQueryBuilder().
		Select("id", "user_id", "team_id", "provider", "ciphertext", "nonce", "flag_use_status", "created_at", "updated_at").
		From("credentials").
		Where("user_id", userID).
		Where("provider", provider).
		Where("flag_use_status", 1)

	query, args, err := qb.Build()
	if err != nil {
		return nil, err
	}

	// SQL: SELECT id, user_id, team_id, provider, ciphertext, nonce, flag_use_status, created_at, updated_at FROM credentials WHERE user_id = $1 AND provider = $2 AND flag_use_status = $3
	row := s.db.QueryRowContext(ctx, query, args...)

	var c Credential
	err = row.Scan(&c.ID, &c.UserID, &c.TeamID, &c.Provider, &c.Ciphertext, &c.Nonce, &c.FlagUseStatus, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan credential: %w", err)
	}
	return &c, nil
}

// CreateCredential inserts a new credential
func (s *Storage) CreateCredential(ctx context.Context, cred *Credential) error {
	now := time.Now()
	cred.CreatedAt = now
	cred.UpdatedAt = now
	cred.FlagUseStatus = 1

	qb := NewQueryBuilder().
		InsertInto("credentials").
		Columns("id", "user_id", "team_id", "provider", "ciphertext", "nonce", "flag_use_status", "created_at", "updated_at").
		Values(cred.ID, cred.UserID, cred.TeamID, cred.Provider, cred.Ciphertext, cred.Nonce, cred.FlagUseStatus, cred.CreatedAt, cred.UpdatedAt)

	query, args, err := qb.Build()
	if err != nil {
		return err
	}

	// SQL: INSERT INTO credentials (id, user_id, team_id, provider, ciphertext, nonce, flag_use_status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	_, err = s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to create credential: %w", err)
	}
	return nil
}

// GetTeamForUser retrieves the team of a specific user
func (s *Storage) GetTeamForUser(ctx context.Context, userID string) (*Team, error) {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	qb := NewQueryBuilder().
		Select("id", "name", "flag_use_status", "created_at", "updated_at").
		From("teams").
		Where("id", user.TeamID).
		Where("flag_use_status", 1)

	query, args, err := qb.Build()
	if err != nil {
		return nil, err
	}

	// SQL: SELECT id, name, flag_use_status, created_at, updated_at FROM teams WHERE id = $1 AND flag_use_status = $2
	row := s.db.QueryRowContext(ctx, query, args...)

	var t Team
	err = row.Scan(&t.ID, &t.Name, &t.FlagUseStatus, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan team: %w", err)
	}
	return &t, nil
}

// UpdateCredential updates an existing credential
func (s *Storage) UpdateCredential(ctx context.Context, cred *Credential) error {
	now := time.Now()
	cred.UpdatedAt = now

	qb := NewQueryBuilder().
		Update("credentials").
		Set("ciphertext", cred.Ciphertext).
		Set("nonce", cred.Nonce).
		Set("updated_at", cred.UpdatedAt).
		Where("id", cred.ID).
		Where("flag_use_status", 1)

	query, args, err := qb.Build()
	if err != nil {
		return err
	}

	// SQL: UPDATE credentials SET ciphertext = $1, nonce = $2, updated_at = $3 WHERE id = $4 AND flag_use_status = $5
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update credential: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("no credential rows updated")
	}
	return nil
}

// DeleteRowData performs soft-deletion by setting flag_use_status to 0
func (s *Storage) DeleteRowData(ctx context.Context, tableName string, id string) error {
	validTables := map[string]bool{
		"users":       true,
		"teams":       true,
		"credentials": true,
	}
	if !validTables[tableName] {
		return fmt.Errorf("invalid table name for soft deletion: %s", tableName)
	}

	qb := NewQueryBuilder().
		Update(tableName).
		Set("flag_use_status", 0).
		Set("updated_at", time.Now()).
		Where("id", id).
		Where("flag_use_status", 1)

	query, args, err := qb.Build()
	if err != nil {
		return err
	}

	// SQL: UPDATE <table> SET flag_use_status = 0, updated_at = $1 WHERE id = $2 AND flag_use_status = $3
	_, err = s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete row from %s: %w", tableName, err)
	}
	return nil
}

// CreateTeam helper to register team (useful for seeding/auth testing)
func (s *Storage) CreateTeam(ctx context.Context, team *Team) error {
	now := time.Now()
	team.CreatedAt = now
	team.UpdatedAt = now
	team.FlagUseStatus = 1

	qb := NewQueryBuilder().
		InsertInto("teams").
		Columns("id", "name", "flag_use_status", "created_at", "updated_at").
		Values(team.ID, team.Name, team.FlagUseStatus, team.CreatedAt, team.UpdatedAt)

	query, args, err := qb.Build()
	if err != nil {
		return err
	}

	// SQL: INSERT INTO teams (id, name, flag_use_status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)
	_, err = s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to create team: %w", err)
	}
	return nil
}

// CreateUser helper to register user (useful for seeding/auth testing)
func (s *Storage) CreateUser(ctx context.Context, u *User) error {
	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now
	u.FlagUseStatus = 1
	u.IsActive = true

	qb := NewQueryBuilder().
		InsertInto("users").
		Columns("id", "username", "email", "password", "team_id", "role", "is_active", "flag_use_status", "created_at", "updated_at").
		Values(u.ID, u.Username, u.Email, u.Password, u.TeamID, u.Role, u.IsActive, u.FlagUseStatus, u.CreatedAt, u.UpdatedAt)

	query, args, err := qb.Build()
	if err != nil {
		return err
	}

	// SQL: INSERT INTO users (id, username, email, password, team_id, role, is_active, flag_use_status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	_, err = s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}
