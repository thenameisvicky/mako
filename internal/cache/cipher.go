package cache

import (
	"sync"
	"time"

	"github.com/thenameisvicky/tokenly-gateway/internal/storage"
)

type credEntry struct {
	decryptedKey []byte
	expiration   time.Time
}

type userEntry struct {
	user       *storage.User
	expiration time.Time
}

// Cache is a thread-safe, process-local in-memory cache with TTL.
//
// NOTE: This cache is process-local. It runs inside the gateway process memory
// and therefore does not synchronize across multiple Gateway instances.
type Cache struct {
	mu         sync.RWMutex
	creds      map[string]credEntry
	users      map[string]userEntry
	defaultTTL time.Duration
	stopJan    chan struct{}
}

// New creates a new in-memory Cache with a default TTL and starts a background cleanup janitor
func New(defaultTTL time.Duration) *Cache {
	c := &Cache{
		creds:      make(map[string]credEntry),
		users:      make(map[string]userEntry),
		defaultTTL: defaultTTL,
		stopJan:    make(chan struct{}),
	}

	go c.startJanitor(1 * time.Minute)

	return c
}

// Close stops the background janitor and securely wipes all credentials
func (c *Cache) Close() {
	close(c.stopJan)
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, entry := range c.creds {
		wipeBytes(entry.decryptedKey)
	}
	c.creds = nil
	c.users = nil
}

// GetCredential retrieves the decrypted credential for a user and provider if it exists and is not expired
func (c *Cache) GetCredential(userID, provider string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := userID + ":" + provider
	entry, found := c.creds[key]
	if !found {
		return nil, false
	}

	if time.Now().After(entry.expiration) {
		return nil, false
	}

	return entry.decryptedKey, true
}

// SetCredential stores the decrypted credential for a user and provider
func (c *Cache) SetCredential(userID, provider string, decryptedKey []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := userID + ":" + provider
	if existing, found := c.creds[key]; found {
		wipeBytes(existing.decryptedKey)
	}

	keyCopy := make([]byte, len(decryptedKey))
	copy(keyCopy, decryptedKey)

	c.creds[key] = credEntry{
		decryptedKey: keyCopy,
		expiration:   time.Now().Add(c.defaultTTL),
	}
}

// InvalidateCredential removes a cached credential for a user and provider, wiping it securely
func (c *Cache) InvalidateCredential(userID, provider string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := userID + ":" + provider
	if entry, found := c.creds[key]; found {
		wipeBytes(entry.decryptedKey)
		delete(c.creds, key)
	}
}

// InvalidateUserCredentials removes and securely wipes all credentials cached for a user
func (c *Cache) InvalidateUserCredentials(userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	prefix := userID + ":"
	for key, entry := range c.creds {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			wipeBytes(entry.decryptedKey)
			delete(c.creds, key)
		}
	}
}

// GetUser retrieves the user if it exists and is not expired
func (c *Cache) GetUser(userID string) (*storage.User, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, found := c.users[userID]
	if !found {
		return nil, false
	}

	if time.Now().After(entry.expiration) {
		return nil, false
	}

	return entry.user, true
}

// SetUser stores the user in the cache
func (c *Cache) SetUser(user *storage.User) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.users[user.ID] = userEntry{
		user:       user,
		expiration: time.Now().Add(c.defaultTTL),
	}
}

// InvalidateUser removes a user from cache
func (c *Cache) InvalidateUser(userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.users, userID)
}

func wipeBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func (c *Cache) startJanitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for k, v := range c.creds {
				if now.After(v.expiration) {
					wipeBytes(v.decryptedKey)
					delete(c.creds, k)
				}
			}
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
