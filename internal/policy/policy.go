package policy

import (
	"context"
	"sync"
	"time"

	"github.com/thenameisvicky/tokenly-mako/internal/storage"
)

// Decision is the outcome of evaluating a request against a team's policy.
type Decision struct {
	Allowed bool
	Reason  string
	Code    string
}

var allowed = Decision{Allowed: true}

type entry struct {
	policy     *storage.Policy // nil means "team has no policy" — a cached negative
	expiration time.Time
}

// Engine evaluates per-team policy on the hot path.
//
// Policies are read from an in-process TTL cache. A miss falls back to one DB
// read and is then cached, including the negative case, so a team without a
// policy does not hit the database on every request.
type Engine struct {
	mu      sync.RWMutex
	cache   map[string]entry
	storage *storage.Storage
	ttl     time.Duration
	stop    chan struct{}

	// static engines serve `fixed` for every team and never touch the database.
	static bool
	fixed  *storage.Policy
}

// NewEngine constructs a policy engine backed by store
func NewEngine(store *storage.Storage, ttl time.Duration) *Engine {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	e := &Engine{
		cache:   make(map[string]entry),
		storage: store,
		ttl:     ttl,
		stop:    make(chan struct{}),
	}
	go e.janitor(time.Minute)
	return e
}

// Close stops the background janitor
func (e *Engine) Close() {
	if e == nil {
		return
	}
	select {
	case <-e.stop:
		return // already closed
	default:
		close(e.stop)
	}
}

// Evaluate applies the team's policy to a request. It never blocks on the
// database when the policy is already cached, which is the common case.
func (e *Engine) Evaluate(ctx context.Context, teamID string, facts RequestFacts) Decision {
	if e == nil {
		return allowed // policy enforcement disabled
	}

	p, err := e.policyFor(ctx, teamID)
	if err != nil {
		// Fail open: a policy lookup failure must not take the proxy down.
		// The request is still recorded against the user's budget.
		return allowed
	}
	if p == nil {
		return allowed
	}

	if facts.Model != "" {
		for _, blocked := range p.BlockedModels {
			if blocked == facts.Model {
				return Decision{Reason: "model " + facts.Model + " is blocked by team policy", Code: "MODEL_BLOCKED"}
			}
		}
		if len(p.AllowedModels) > 0 {
			permitted := false
			for _, m := range p.AllowedModels {
				if m == facts.Model {
					permitted = true
					break
				}
			}
			if !permitted {
				return Decision{Reason: "model " + facts.Model + " is not in the team's allowed list", Code: "MODEL_NOT_ALLOWED"}
			}
		}
	}

	if p.MaxTokensPerRequest > 0 && facts.MaxTokens > p.MaxTokensPerRequest {
		return Decision{Reason: "max_tokens exceeds the team limit", Code: "MAX_TOKENS_EXCEEDED"}
	}

	return allowed
}

// Invalidate drops a team's cached policy so the next request re-reads it
func (e *Engine) Invalidate(teamID string) {
	e.mu.Lock()
	delete(e.cache, teamID)
	e.mu.Unlock()
}

func (e *Engine) policyFor(ctx context.Context, teamID string) (*storage.Policy, error) {
	if e.static {
		return e.fixed, nil
	}

	e.mu.RLock()
	ent, found := e.cache[teamID]
	e.mu.RUnlock()

	if found && time.Now().Before(ent.expiration) {
		return ent.policy, nil
	}

	p, err := e.storage.GetPolicyForTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}

	e.mu.Lock()
	e.cache[teamID] = entry{policy: p, expiration: time.Now().Add(e.ttl)}
	e.mu.Unlock()

	return p, nil
}

func (e *Engine) janitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.mu.Lock()
			now := time.Now()
			for k, v := range e.cache {
				if now.After(v.expiration) {
					delete(e.cache, k)
				}
			}
			e.mu.Unlock()
		case <-e.stop:
			return
		}
	}
}

// NewEngineWithPolicy builds an engine that serves one fixed policy for every
// team, without any database access. Intended for tests and for running the
// proxy against a static policy.
func NewEngineWithPolicy(p *storage.Policy) *Engine {
	return &Engine{
		cache:  map[string]entry{},
		static: true,
		fixed:  p,
		stop:   make(chan struct{}),
	}
}
