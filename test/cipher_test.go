package test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thenameisvicky/tokenly-gateway/internal/cache"
	"github.com/thenameisvicky/tokenly-gateway/internal/storage"
)

func TestCacheOperations(t *testing.T) {
	c := cache.New(50 * time.Millisecond)
	defer c.Close()

	userID := "test-user-123"
	provider := "anthropic"
	rawKey := []byte("my-super-secret-decrypted-key")

	// 1. Get non-existent credential
	_, found := c.GetCredential(userID, provider)
	assert.False(t, found)

	// 2. Set and Get credential
	c.SetCredential(userID, provider, rawKey)
	got, found := c.GetCredential(userID, provider)
	require.True(t, found)
	assert.Equal(t, rawKey, got)

	// 3. Invalidate credential
	c.InvalidateCredential(userID, provider)
	_, found = c.GetCredential(userID, provider)
	assert.False(t, found)

	// 4. Invalidate all user credentials
	c.SetCredential(userID, provider, rawKey)
	c.InvalidateUserCredentials(userID)
	_, found = c.GetCredential(userID, provider)
	assert.False(t, found)

	// 5. User caching
	user := &storage.User{
		ID:       userID,
		Username: "testuser",
	}
	_, found = c.GetUser(userID)
	assert.False(t, found)

	c.SetUser(user)
	gotUser, found := c.GetUser(userID)
	require.True(t, found)
	assert.Equal(t, "testuser", gotUser.Username)

	c.InvalidateUser(userID)
	_, found = c.GetUser(userID)
	assert.False(t, found)

	// 6. TTL expiration
	c.SetCredential(userID, provider, rawKey)
	time.Sleep(100 * time.Millisecond)
	_, found = c.GetCredential(userID, provider)
	assert.False(t, found)
}
