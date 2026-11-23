package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// Policy is the per-team rule set applied to proxied requests.
// Empty AllowedModels means "allow everything"; MaxTokensPerRequest 0 means no cap.
type Policy struct {
	ID                  string    `json:"id"`
	TeamID              string    `json:"team_id"`
	AllowedModels       []string  `json:"allowed_models"`
	BlockedModels       []string  `json:"blocked_models"`
	MaxTokensPerRequest int       `json:"max_tokens_per_request"`
	FlagUseStatus       int       `json:"-"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// Budget caps token spend for a team, or for one user when UserID is set.
type Budget struct {
	ID            string    `json:"id"`
	TeamID        string    `json:"team_id"`
	UserID        string    `json:"user_id,omitempty"`
	Period        string    `json:"period"`
	TokenLimit    int64     `json:"token_limit"`
	FlagUseStatus int       `json:"-"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// UsageRecord is one proxied request's token spend, written off the hot path.
type UsageRecord struct {
	TeamID       string
	UserID       string
	Model        string
	InputTokens  int
	OutputTokens int
	OccurredAt   time.Time
}

// GetPolicyForTeam loads a team's policy, or nil when the team has none.
func (s *Storage) GetPolicyForTeam(ctx context.Context, teamID string) (*Policy, error) {
	qb := NewQueryBuilder().
		Select("id", "team_id", "allowed_models", "blocked_models", "max_tokens_per_request", "flag_use_status", "created_at", "updated_at").
		From("policies").
		Where("team_id", teamID).
		Where("flag_use_status", 1)

	query, args, err := qb.Build()
	if err != nil {
		return nil, err
	}

	// SQL: SELECT ... FROM policies WHERE team_id = $1 AND flag_use_status = $2
	var p Policy
	err = s.db.QueryRowContext(ctx, query, args...).Scan(
		&p.ID, &p.TeamID, pq.Array(&p.AllowedModels), pq.Array(&p.BlockedModels),
		&p.MaxTokensPerRequest, &p.FlagUseStatus, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan policy: %w", err)
	}
	return &p, nil
}

// GetBudgetForTeam loads a team's budget, or nil when the team has none.
func (s *Storage) GetBudgetForTeam(ctx context.Context, teamID string) (*Budget, error) {
	// Built by hand rather than through the query builder: the team-wide budget is
	// the row with no user_id, and the builder only emits equality predicates.
	//
	// Mongo equivalent: db.budgets.findOne({team_id, user_id: null, flag_use_status: 1})
	const query = `
		SELECT id, team_id, user_id, period, token_limit, flag_use_status, created_at, updated_at
		FROM budgets
		WHERE team_id = $1 AND user_id IS NULL AND flag_use_status = 1`
	var b Budget
	var userID sql.NullString
	err := s.db.QueryRowContext(ctx, query, teamID).Scan(
		&b.ID, &b.TeamID, &userID, &b.Period, &b.TokenLimit, &b.FlagUseStatus, &b.CreatedAt, &b.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan budget: %w", err)
	}
	b.UserID = userID.String
	return &b, nil
}

// RecordUsageBatch appends usage rows in a single round trip.
// Called only from the background usage worker, never on the hot path.
func (s *Storage) RecordUsageBatch(ctx context.Context, records []UsageRecord) error {
	if len(records) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin usage transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Mongo equivalent: db.usage_records.insertMany([...])
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO usage_records (team_id, user_id, model, input_tokens, output_tokens, occurred_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`)
	if err != nil {
		return fmt.Errorf("failed to prepare usage insert: %w", err)
	}
	defer stmt.Close()

	for _, rec := range records {
		if _, err := stmt.ExecContext(ctx, rec.TeamID, rec.UserID, rec.Model,
			rec.InputTokens, rec.OutputTokens, rec.OccurredAt); err != nil {
			return fmt.Errorf("failed to insert usage record: %w", err)
		}
	}
	return tx.Commit()
}

// SumUsageSince totals a user's tokens since a cutoff, used to warm the in-memory
// budget counters on a cache miss.
func (s *Storage) SumUsageSince(ctx context.Context, teamID, userID string, since time.Time) (int64, error) {
	// Mongo equivalent: db.usage_records.aggregate([{$match:...},{$group:{_id:null,total:{$sum:...}}}])
	const query = `
		SELECT COALESCE(SUM(input_tokens + output_tokens), 0)
		FROM usage_records
		WHERE team_id = $1 AND user_id = $2 AND occurred_at >= $3`

	var total int64
	if err := s.db.QueryRowContext(ctx, query, teamID, userID, since).Scan(&total); err != nil {
		return 0, fmt.Errorf("failed to sum usage: %w", err)
	}
	return total, nil
}

// UpsertPolicy writes a team's policy, replacing any existing one.
func (s *Storage) UpsertPolicy(ctx context.Context, p *Policy) error {
	now := time.Now()
	p.UpdatedAt = now
	p.FlagUseStatus = 1

	// Mongo equivalent: updateOne({team_id}, {$set: {...}}, {upsert: true})
	const query = `
		INSERT INTO policies
			(team_id, allowed_models, blocked_models, max_tokens_per_request, flag_use_status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 1, $5, $6)
		ON CONFLICT (team_id) WHERE flag_use_status = 1
		DO UPDATE SET
			allowed_models         = EXCLUDED.allowed_models,
			blocked_models         = EXCLUDED.blocked_models,
			max_tokens_per_request = EXCLUDED.max_tokens_per_request,
			updated_at             = EXCLUDED.updated_at
		RETURNING id, created_at`

	err := s.db.QueryRowContext(ctx, query,
		p.TeamID, pq.Array(p.AllowedModels), pq.Array(p.BlockedModels),
		p.MaxTokensPerRequest, now, p.UpdatedAt,
	).Scan(&p.ID, &p.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to upsert policy: %w", err)
	}
	return nil
}

// UpsertBudget writes a team-wide budget, replacing any existing one.
func (s *Storage) UpsertBudget(ctx context.Context, b *Budget) error {
	now := time.Now()
	b.UpdatedAt = now
	b.FlagUseStatus = 1
	if b.Period == "" {
		b.Period = "monthly"
	}

	// Mongo equivalent: updateOne({team_id, user_id: null}, {$set: {...}}, {upsert: true})
	const query = `
		INSERT INTO budgets
			(team_id, user_id, period, token_limit, flag_use_status, created_at, updated_at)
		VALUES ($1, NULL, $2, $3, 1, $4, $5)
		ON CONFLICT (team_id) WHERE flag_use_status = 1 AND user_id IS NULL
		DO UPDATE SET
			period      = EXCLUDED.period,
			token_limit = EXCLUDED.token_limit,
			updated_at  = EXCLUDED.updated_at
		RETURNING id, created_at`

	err := s.db.QueryRowContext(ctx, query,
		b.TeamID, b.Period, b.TokenLimit, now, b.UpdatedAt,
	).Scan(&b.ID, &b.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to upsert budget: %w", err)
	}
	return nil
}

// TeamUsage is a rollup of a team's token spend for the current period.
type TeamUsage struct {
	TotalTokens  int64        `json:"totalTokens"`
	InputTokens  int64        `json:"inputTokens"`
	OutputTokens int64        `json:"outputTokens"`
	Requests     int64        `json:"requests"`
	ByModel      []ModelUsage `json:"byModel"`
	Daily        []DailyUsage `json:"daily"`
}

type ModelUsage struct {
	Model  string `json:"model"`
	Tokens int64  `json:"tokens"`
	Count  int64  `json:"count"`
}

type DailyUsage struct {
	Day    string `json:"day"`
	Tokens int64  `json:"tokens"`
}

// GetTeamUsage summarises a team's spend since a cutoff, for the dashboard.
// This is a reporting query and never runs on the proxy hot path.
func (s *Storage) GetTeamUsage(ctx context.Context, teamID string, since time.Time) (*TeamUsage, error) {
	usage := &TeamUsage{ByModel: []ModelUsage{}, Daily: []DailyUsage{}}

	// Mongo equivalent: aggregate([{$match}, {$group: {_id: null, ...}}])
	const totals = `
		SELECT COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COUNT(*)
		FROM usage_records
		WHERE team_id = $1 AND occurred_at >= $2`
	if err := s.db.QueryRowContext(ctx, totals, teamID, since).
		Scan(&usage.InputTokens, &usage.OutputTokens, &usage.Requests); err != nil {
		return nil, fmt.Errorf("failed to total team usage: %w", err)
	}
	usage.TotalTokens = usage.InputTokens + usage.OutputTokens

	// Mongo equivalent: aggregate([{$match}, {$group: {_id: "$model", ...}}, {$sort}])
	const byModel = `
		SELECT model, COALESCE(SUM(input_tokens + output_tokens), 0), COUNT(*)
		FROM usage_records
		WHERE team_id = $1 AND occurred_at >= $2
		GROUP BY model
		ORDER BY 2 DESC
		LIMIT 20`
	modelRows, err := s.db.QueryContext(ctx, byModel, teamID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to group usage by model: %w", err)
	}
	defer modelRows.Close()
	for modelRows.Next() {
		var m ModelUsage
		if err := modelRows.Scan(&m.Model, &m.Tokens, &m.Count); err != nil {
			return nil, err
		}
		if m.Model == "" {
			m.Model = "unknown"
		}
		usage.ByModel = append(usage.ByModel, m)
	}
	if err := modelRows.Err(); err != nil {
		return nil, err
	}

	// Mongo equivalent: aggregate([{$match}, {$group: {_id: {$dateToString}}}, {$sort}])
	const daily = `
		SELECT to_char(date_trunc('day', occurred_at), 'YYYY-MM-DD'),
		       COALESCE(SUM(input_tokens + output_tokens), 0)
		FROM usage_records
		WHERE team_id = $1 AND occurred_at >= $2
		GROUP BY 1
		ORDER BY 1`
	dailyRows, err := s.db.QueryContext(ctx, daily, teamID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to group usage by day: %w", err)
	}
	defer dailyRows.Close()
	for dailyRows.Next() {
		var d DailyUsage
		if err := dailyRows.Scan(&d.Day, &d.Tokens); err != nil {
			return nil, err
		}
		usage.Daily = append(usage.Daily, d)
	}
	return usage, dailyRows.Err()
}
