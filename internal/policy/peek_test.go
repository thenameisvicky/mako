package policy

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPeekBodyExtractsFactsAndReplaysBody(t *testing.T) {
	body := `{"model":"claude-opus-4","max_tokens":4096,"stream":true,"messages":[{"role":"user","content":"hi"}]}`

	facts, replay := PeekBody(strings.NewReader(body))

	require.Equal(t, "claude-opus-4", facts.Model)
	require.Equal(t, 4096, facts.MaxTokens)
	require.True(t, facts.Stream)

	// The upstream request must receive the body byte-for-byte.
	got, err := io.ReadAll(replay)
	require.NoError(t, err)
	require.Equal(t, body, string(got))
}

func TestPeekBodyReplaysBodyLargerThanPeekLimit(t *testing.T) {
	filler := strings.Repeat("x", peekLimit*3)
	body := `{"model":"claude-sonnet-4","messages":[{"role":"user","content":"` + filler + `"}]}`

	facts, replay := PeekBody(strings.NewReader(body))
	require.Equal(t, "claude-sonnet-4", facts.Model)

	got, err := io.ReadAll(replay)
	require.NoError(t, err)
	require.Equal(t, body, string(got), "a body past the peek limit must still be forwarded intact")
}

func TestPeekBodyHandlesMissingAndTruncatedFields(t *testing.T) {
	facts, replay := PeekBody(strings.NewReader(`{"messages":[]}`))
	require.Empty(t, facts.Model)
	require.Zero(t, facts.MaxTokens)
	require.False(t, facts.Stream)
	require.NotNil(t, replay)

	// A model name cut off by the peek limit must not yield a partial match.
	truncated := `{"padding":"` + strings.Repeat("y", peekLimit) + `","model":"claude-opus-4"}`
	facts, _ = PeekBody(strings.NewReader(truncated))
	require.Empty(t, facts.Model)
}

func TestPeekBodyNilBody(t *testing.T) {
	facts, replay := PeekBody(nil)
	require.Empty(t, facts.Model)
	require.Nil(t, replay)
}

func TestEvaluateBlocksAndAllows(t *testing.T) {
	// An engine with no storage still serves cached decisions.
	e := &Engine{cache: map[string]entry{}, stop: make(chan struct{})}
	e.cache["team-1"] = entry{
		policy: &policyFixture,
		// far-future expiry so policyFor never reaches the nil storage
		expiration: farFuture(),
	}

	blocked := e.Evaluate(nil, "team-1", RequestFacts{Model: "claude-opus-4"})
	require.False(t, blocked.Allowed)
	require.Equal(t, "MODEL_BLOCKED", blocked.Code)

	notAllowed := e.Evaluate(nil, "team-1", RequestFacts{Model: "gpt-4"})
	require.False(t, notAllowed.Allowed)
	require.Equal(t, "MODEL_NOT_ALLOWED", notAllowed.Code)

	overLimit := e.Evaluate(nil, "team-1", RequestFacts{Model: "claude-sonnet-4", MaxTokens: 9999})
	require.False(t, overLimit.Allowed)
	require.Equal(t, "MAX_TOKENS_EXCEEDED", overLimit.Code)

	ok := e.Evaluate(nil, "team-1", RequestFacts{Model: "claude-sonnet-4", MaxTokens: 100})
	require.True(t, ok.Allowed)
}
