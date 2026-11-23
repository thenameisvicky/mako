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
	UsernameBidx  string    `json:"-"` // deterministic blind index for encrypted lookups
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
		Select("id", "username", "username_bidx", "email", "password", "team_id", "role", "is_active", "flag_use_status", "created_at", "updated_at").
		From("users").
		Where("id", id).
		Where("flag_use_status", 1)

	query, args, err := qb.Build()
	if err != nil {
		return nil, err
	}

	// SQL: SELECT id, username, email, password, team_id, role, is_active, flag_use_status, created_at, updated_at FROM users WHERE id = $1 AND flag_use_status = $2
	return scanUser(s.db.QueryRowContext(ctx, query, args...))
}

// GetUserByUsername retrieves a user by username
func (s *Storage) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	qb := NewQueryBuilder().
		Select("id", "username", "username_bidx", "email", "password", "team_id", "role", "is_active", "flag_use_status", "created_at", "updated_at").
		From("users").
		Where("username", username).
		Where("flag_use_status", 1)

	query, args, err := qb.Build()
	if err != nil {
		return nil, err
	}

	// SQL: SELECT id, username, email, password, team_id, role, is_active, flag_use_status, created_at, updated_at FROM users WHERE username = $1 AND flag_use_status = $2
	return scanUser(s.db.QueryRowContext(ctx, query, args...))
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

	cols := []string{"user_id", "team_id", "provider", "ciphertext", "nonce", "flag_use_status", "created_at", "updated_at"}
	vals := []interface{}{cred.UserID, cred.TeamID, cred.Provider, cred.Ciphertext, cred.Nonce, cred.FlagUseStatus, cred.CreatedAt, cred.UpdatedAt}

	if cred.ID != "" {
		cols = append([]string{"id"}, cols...)
		vals = append([]interface{}{cred.ID}, vals...)
	}

	qb := NewQueryBuilder().
		InsertInto("credentials").
		Columns(cols...).
		Values(vals...)

	query, args, err := qb.Build()
	if err != nil {
		return err
	}

	query += " RETURNING id"
	err = s.db.QueryRowContext(ctx, query, args...).Scan(&cred.ID)
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

// GetTeamByID retrieves a team by its ID
func (s *Storage) GetTeamByID(ctx context.Context, id string) (*Team, error) {
	qb := NewQueryBuilder().
		Select("id", "name", "flag_use_status", "created_at", "updated_at").
		From("teams").
		Where("id", id).
		Where("flag_use_status", 1)

	query, args, err := qb.Build()
	if err != nil {
		return nil, err
	}

	row := s.db.QueryRowContext(ctx, query, args...)

	var t Team
	err = row.Scan(&t.ID, &t.Name, &t.FlagUseStatus, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan team by ID: %w", err)
	}
	return &t, nil
}

// CreateTeam helper to register team (useful for seeding/auth testing)
func (s *Storage) CreateTeam(ctx context.Context, team *Team) error {
	now := time.Now()
	team.CreatedAt = now
	team.UpdatedAt = now
	team.FlagUseStatus = 1

	cols := []string{"name", "flag_use_status", "created_at", "updated_at"}
	vals := []interface{}{team.Name, team.FlagUseStatus, team.CreatedAt, team.UpdatedAt}

	if team.ID != "" {
		cols = append([]string{"id"}, cols...)
		vals = append([]interface{}{team.ID}, vals...)
	}

	qb := NewQueryBuilder().
		InsertInto("teams").
		Columns(cols...).
		Values(vals...)

	query, args, err := qb.Build()
	if err != nil {
		return err
	}

	query += " RETURNING id"
	err = s.db.QueryRowContext(ctx, query, args...).Scan(&team.ID)
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

	cols := []string{"username", "username_bidx", "email", "password", "team_id", "role", "is_active", "flag_use_status", "created_at", "updated_at"}
	vals := []interface{}{u.Username, u.UsernameBidx, u.Email, u.Password, u.TeamID, u.Role, u.IsActive, u.FlagUseStatus, u.CreatedAt, u.UpdatedAt}

	if u.ID != "" {
		cols = append([]string{"id"}, cols...)
		vals = append([]interface{}{u.ID}, vals...)
	}

	qb := NewQueryBuilder().
		InsertInto("users").
		Columns(cols...).
		Values(vals...)

	query, args, err := qb.Build()
	if err != nil {
		return err
	}

	query += " RETURNING id"
	err = s.db.QueryRowContext(ctx, query, args...).Scan(&u.ID)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// --- Encrypted identity ------------------------------------------------------

// GetUserByUsernameBidx retrieves a user by the blind index of their username.
//
// Usernames are AES-GCM encrypted, so they cannot be matched by equality; the
// blind index is the deterministic HMAC the unique index is built on.
func (s *Storage) GetUserByUsernameBidx(ctx context.Context, bidx string) (*User, error) {
	qb := NewQueryBuilder().
		Select("id", "username", "username_bidx", "email", "password", "team_id", "role", "is_active", "flag_use_status", "created_at", "updated_at").
		From("users").
		Where("username_bidx", bidx).
		Where("flag_use_status", 1)

	query, args, err := qb.Build()
	if err != nil {
		return nil, err
	}

	// SQL: SELECT ... FROM users WHERE username_bidx = $1 AND flag_use_status = $2
	return scanUser(s.db.QueryRowContext(ctx, query, args...))
}

// UpdateUserIdentity rewrites a user's encrypted identity columns in place.
//
// Used to migrate a legacy plaintext/bcrypt row to AES-GCM on the user's next
// successful login, so no password reset or downtime is required.
func (s *Storage) UpdateUserIdentity(ctx context.Context, u *User) error {
	u.UpdatedAt = time.Now()

	qb := NewQueryBuilder().
		Update("users").
		Set("username", u.Username).
		Set("username_bidx", u.UsernameBidx).
		Set("email", u.Email).
		Set("password", u.Password).
		Set("updated_at", u.UpdatedAt).
		Where("id", u.ID).
		Where("flag_use_status", 1)

	query, args, err := qb.Build()
	if err != nil {
		return err
	}

	// SQL: UPDATE users SET username = $1, username_bidx = $2, ... WHERE id = $6 AND flag_use_status = $7
	if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("failed to update user identity: %w", err)
	}
	return nil
}

// scanUser reads a users row in the canonical column order
func scanUser(row *sql.Row) (*User, error) {
	var u User
	var bidx sql.NullString
	err := row.Scan(&u.ID, &u.Username, &bidx, &u.Email, &u.Password, &u.TeamID, &u.Role, &u.IsActive, &u.FlagUseStatus, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}
	u.UsernameBidx = bidx.String
	return &u, nil
}

// CreateTeamWithUser creates a team and its first user atomically.
//
// The two inserts share a transaction so a failure on the user leaves no orphan
// team behind - which would otherwise be invisible, since nothing else in the
// system lists empty teams.
func (s *Storage) CreateTeamWithUser(ctx context.Context, team *Team, u *User) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin signup transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now()
	team.CreatedAt, team.UpdatedAt, team.FlagUseStatus = now, now, 1

	// SQL: INSERT INTO teams (name, flag_use_status, created_at, updated_at) VALUES (...) RETURNING id
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO teams (name, flag_use_status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		team.Name, team.FlagUseStatus, team.CreatedAt, team.UpdatedAt,
	).Scan(&team.ID); err != nil {
		return fmt.Errorf("failed to create team: %w", err)
	}

	u.TeamID = team.ID
	u.CreatedAt, u.UpdatedAt, u.FlagUseStatus, u.IsActive = now, now, 1, true

	// SQL: INSERT INTO users (username, username_bidx, email, password, team_id, role, ...) VALUES (...) RETURNING id
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO users (username, username_bidx, email, password, team_id, role,
		                    is_active, flag_use_status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`,
		u.Username, u.UsernameBidx, u.Email, u.Password, u.TeamID, u.Role,
		u.IsActive, u.FlagUseStatus, u.CreatedAt, u.UpdatedAt,
	).Scan(&u.ID); err != nil {
		return fmt.Errorf("failed to create first team user: %w", err)
	}

	return tx.Commit()
}
