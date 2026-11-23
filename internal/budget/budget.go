package budget

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/thenameisvicky/tokenly-mako/internal/storage"
)

// counter is one user's running token spend for the current period.
type counter struct {
	used       atomic.Int64
	limit      int64
	expiration time.Time
}

// Tracker enforces token budgets on the hot path and persists usage off it.
//
// Reads and increments are atomic in-memory operations. The only database work
// is a one-time warm-up per user per period, and a batched flush from a
// background worker — no request ever waits on a write.
type Tracker struct {
	mu       sync.RWMutex
	counters map[string]*counter

	storage *storage.Storage
	ttl     time.Duration

	usage chan storage.UsageRecord
	stop  chan struct{}
	wg    sync.WaitGroup
}

// Decision is the outcome of a budget check.
type Decision struct {
	Allowed bool
	Reason  string
	Used    int64
	Limit   int64
}

// NewTracker starts a budget tracker with a background usage writer.
// queueSize bounds how many unflushed usage records are held in memory.
func NewTracker(store *storage.Storage, ttl time.Duration, queueSize int) *Tracker {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if queueSize <= 0 {
		queueSize = 4096
	}

	t := &Tracker{
		counters: make(map[string]*counter),
		storage:  store,
		ttl:      ttl,
		usage:    make(chan storage.UsageRecord, queueSize),
		stop:     make(chan struct{}),
	}

	t.wg.Add(1)
	go t.usageWriter(5*time.Second, 128)

	return t
}

// Close drains and flushes any pending usage records
func (t *Tracker) Close() {
	if t == nil {
		return
	}
	close(t.stop)
	t.wg.Wait()
}

// Check reports whether a user is still within budget.
//
// On a cache miss it warms the counter from the database once, then serves every
// subsequent request from memory for the TTL.
func (t *Tracker) Check(ctx context.Context, teamID, userID string) Decision {
	if t == nil {
		return Decision{Allowed: true} // budgeting disabled
	}

	c := t.counterFor(ctx, teamID, userID)
	if c == nil || c.limit <= 0 {
		return Decision{Allowed: true} // no budget configured means unlimited
	}

	used := c.used.Load()
	if used >= c.limit {
		return Decision{Allowed: false, Reason: "token budget exhausted for this period", Used: used, Limit: c.limit}
	}
	return Decision{Allowed: true, Used: used, Limit: c.limit}
}

// Record adds a request's token spend to the in-memory counter and queues it for
// persistence. It never blocks: if the queue is full the record is dropped and
// counted in the log rather than slowing the hot path.
func (t *Tracker) Record(teamID, userID, model string, inputTokens, outputTokens int) {
	if t == nil {
		return // budgeting disabled
	}

	total := int64(inputTokens + outputTokens)
	if total <= 0 {
		return
	}

	key := teamID + ":" + userID
	t.mu.RLock()
	c, found := t.counters[key]
	t.mu.RUnlock()
	if found {
		c.used.Add(total)
	}

	select {
	case t.usage <- storage.UsageRecord{
		TeamID:       teamID,
		UserID:       userID,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		OccurredAt:   time.Now(),
	}:
	default:
		log.Printf("[Budget] usage queue full, dropping record for team=%s", teamID)
	}
}

func (t *Tracker) counterFor(ctx context.Context, teamID, userID string) *counter {
	key := teamID + ":" + userID

	t.mu.RLock()
	c, found := t.counters[key]
	t.mu.RUnlock()
	if found && time.Now().Before(c.expiration) {
		return c
	}

	b, err := t.storage.GetBudgetForTeam(ctx, teamID)
	if err != nil {
		// Fail open rather than block every request behind a database problem.
		return nil
	}

	fresh := &counter{expiration: time.Now().Add(t.ttl)}
	if b != nil {
		fresh.limit = b.TokenLimit
		if used, err := t.storage.SumUsageSince(ctx, teamID, userID, periodStart(b.Period)); err == nil {
			fresh.used.Store(used)
		}
	}

	t.mu.Lock()
	t.counters[key] = fresh
	t.mu.Unlock()

	return fresh
}

// usageWriter batches queued usage records and writes them in the background.
func (t *Tracker) usageWriter(flushInterval time.Duration, batchSize int) {
	defer t.wg.Done()

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	batch := make([]storage.UsageRecord, 0, batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := t.storage.RecordUsageBatch(ctx, batch); err != nil {
			log.Printf("[Budget] failed to persist %d usage records: %v", len(batch), err)
		}
		cancel()
		batch = batch[:0]
	}

	for {
		select {
		case rec := <-t.usage:
			batch = append(batch, rec)
			if len(batch) >= batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-t.stop:
			// Drain whatever is still queued before shutting down.
			for {
				select {
				case rec := <-t.usage:
					batch = append(batch, rec)
					if len(batch) >= batchSize {
						flush()
					}
					continue
				default:
				}
				break
			}
			flush()
			return
		}
	}
}

// periodStart returns the cutoff for the current budget period
func periodStart(period string) time.Time {
	now := time.Now()
	switch period {
	case "daily":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "weekly":
		return now.AddDate(0, 0, -int(now.Weekday()))
	default: // monthly
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	}
}
