package test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thenameisvicky/tokenly-mako/internal/cache"
	"github.com/thenameisvicky/tokenly-mako/internal/storage"
)

func TestCacheOperations(t *testing.T) {
	c := cache.New(50 * time.Millisecond)
	defer c.Close()

	userID := "test-user-123"
	_, found := c.GetUser(userID)
	assert.False(t, found)

	c.SetUser(&storage.User{ID: userID, Username: "testuser"})
	got, found := c.GetUser(userID)
	require.True(t, found)
	assert.Equal(t, "testuser", got.Username)

	c.InvalidateUser(userID)
	_, found = c.GetUser(userID)
	assert.False(t, found)

	// TTL expiration
	c.SetUser(&storage.User{ID: userID})
	time.Sleep(100 * time.Millisecond)
	_, found = c.GetUser(userID)
	assert.False(t, found)
}
