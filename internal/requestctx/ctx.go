package requestctx

import (
	"context"
	"sync"

	"github.com/thenameisvicky/tokenly-gateway/internal/storage"
)

type RequestState struct {
	User      *storage.User
	Provider  string
	CacheHit  bool
	RequestID string
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
	state.CacheHit = false
	state.RequestID = ""
	statePool.Put(state)
}

// GetState retrieves the RequestState pointer from the context
func GetState(ctx context.Context) (*RequestState, bool) {
	state, ok := ctx.Value(StateKey).(*RequestState)
	return state, ok
}
