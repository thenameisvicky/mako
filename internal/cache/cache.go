package cache

import (
	"sync"
	"time"

	"github.com/thenameisvicky/tokenly-mako/internal/storage"
)

type userEntry struct {
	user       *storage.User
	expiration time.Time
}

// Cache is a thread-safe, process-local TTL cache of authenticated users, so a
// warm request skips the database. It is not shared across server instances.
type Cache struct {
	mu         sync.RWMutex
	users      map[string]userEntry
	defaultTTL time.Duration
	stopJan    chan struct{}
}

// New creates a Cache and starts a background janitor that evicts expired entries.
func New(defaultTTL time.Duration) *Cache {
	c := &Cache{
		users:      make(map[string]userEntry),
		defaultTTL: defaultTTL,
		stopJan:    make(chan struct{}),
	}
	go c.startJanitor(time.Minute)
	return c
}

// Close stops the janitor and drops every entry.
func (c *Cache) Close() {
	close(c.stopJan)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.users = nil
}

// GetUser returns the cached user if present and not expired.
func (c *Cache) GetUser(userID string) (*storage.User, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, found := c.users[userID]
	if !found || time.Now().After(entry.expiration) {
		return nil, false
	}
	return entry.user, true
}

// SetUser caches a user for the default TTL.
func (c *Cache) SetUser(user *storage.User) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.users[user.ID] = userEntry{user: user, expiration: time.Now().Add(c.defaultTTL)}
}

// InvalidateUser removes a user from the cache.
func (c *Cache) InvalidateUser(userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.users, userID)
}

func (c *Cache) startJanitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for k, v := range c.users {
				if now.After(v.expiration) {
					delete(c.users, k)
				}
			}
			c.mu.Unlock()
		case <-c.stopJan:
			return
		}
	}
}
