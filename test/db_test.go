package test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thenameisvicky/tokenly-mako/internal/storage"
)

func TestStorageCRUD(t *testing.T) {
	ctx := context.Background()
	s, err := storage.New(testDBURL)
	require.NoError(t, err)
	defer s.Close()

	teamID := uuid.New().String()
	userID := uuid.New().String()

	// 1. Create a Team
	team := &storage.Team{
		ID:   teamID,
		Name: "Test Team",
	}
	err = s.CreateTeam(ctx, team)
	require.NoError(t, err)

	// 2. Create a User
	user := &storage.User{
		ID:       userID,
		Username: "testuser_" + uuid.New().String()[:8],
		Email:    "testuser_" + uuid.New().String()[:8] + "@example.com",
		Password: "hashed_password",
		TeamID:   teamID,
		Role:     "owner",
	}
	err = s.CreateUser(ctx, user)
	require.NoError(t, err)

	// 3. Get User By ID
	fetchedUser, err := s.GetUserByID(ctx, userID)
	require.NoError(t, err)
	require.NotNil(t, fetchedUser)
	assert.Equal(t, user.Username, fetchedUser.Username)

	// 4. Get Team for User
	fetchedTeam, err := s.GetTeamForUser(ctx, userID)
	require.NoError(t, err)
	require.NotNil(t, fetchedTeam)
	assert.Equal(t, "Test Team", fetchedTeam.Name)
}
