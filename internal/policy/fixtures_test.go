package policy

import (
	"time"

	"github.com/thenameisvicky/tokenly-mako/internal/storage"
)

var policyFixture = storage.Policy{
	TeamID:              "team-1",
	AllowedModels:       []string{"claude-sonnet-4"},
	BlockedModels:       []string{"claude-opus-4"},
	MaxTokensPerRequest: 1000,
}

func farFuture() time.Time { return time.Now().Add(time.Hour) }
