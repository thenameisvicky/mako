package requestctx

import (
	"context"
	"sync"

	"github.com/thenameisvicky/tokenly-mako/internal/storage"
)

type RequestState struct {
	User      *storage.User
	Provider  string
	Model     string
	CacheHit  bool
	RequestID string

	// Token counts observed on the response, used for logging and telemetry.
	InputTokens  int
	OutputTokens int
}

type contextKey string

const (
	StateKey contextKey = "request_state"
)

var statePool = sync.Pool{
	New: func() interface{} {
		return &RequestState{}
	},
}

// WithState initializes a RequestState on the context from the sync.Pool
func WithState(ctx context.Context) (context.Context, *RequestState) {
	state := statePool.Get().(*RequestState)
	return context.WithValue(ctx, StateKey, state), state
}

// ReleaseState resets the RequestState and returns it to the pool
func ReleaseState(state *RequestState) {
	state.User = nil
	state.Provider = ""
	state.Model = ""
	state.CacheHit = false
	state.RequestID = ""
	state.InputTokens = 0
	state.OutputTokens = 0
	statePool.Put(state)
}

// GetState retrieves the RequestState pointer from the context
func GetState(ctx context.Context) (*RequestState, bool) {
	state, ok := ctx.Value(StateKey).(*RequestState)
	return state, ok
}
