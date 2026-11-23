package proxy

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageSnifferNonStreamingResponse(t *testing.T) {
	var out bytes.Buffer
	s := newUsageSniffer(&out)

	body := `{"id":"msg_1","content":[],"usage":{"input_tokens":1200,"output_tokens":350}}`
	n, err := s.Write([]byte(body))

	require.NoError(t, err)
	require.Equal(t, len(body), n)
	require.Equal(t, body, out.String(), "the body must pass through untouched")
	require.Equal(t, 1200, s.inputTokens)
	require.Equal(t, 350, s.outputTokens)
}

func TestUsageSnifferStreamingTakesFinalCount(t *testing.T) {
	var out bytes.Buffer
	s := newUsageSniffer(&out)

	frames := []string{
		`event: message_start` + "\n" + `data: {"message":{"usage":{"input_tokens":900,"output_tokens":1}}}` + "\n\n",
		`event: message_delta` + "\n" + `data: {"usage":{"output_tokens":42}}` + "\n\n",
		`event: message_delta` + "\n" + `data: {"usage":{"output_tokens":128}}` + "\n\n",
	}

	var expected string
	for _, f := range frames {
		_, err := s.Write([]byte(f))
		require.NoError(t, err)
		expected += f
	}

	require.Equal(t, expected, out.String())
	require.Equal(t, 900, s.inputTokens)
	require.Equal(t, 128, s.outputTokens, "the last streamed count wins")
}

func TestUsageSnifferCountSplitAcrossChunks(t *testing.T) {
	var out bytes.Buffer
	s := newUsageSniffer(&out)

	// The counter straddles a chunk boundary, which the carry buffer must bridge.
	_, err := s.Write([]byte(`{"usage":{"input_to`))
	require.NoError(t, err)
	_, err = s.Write([]byte(`kens":777,"output_tokens":11}}`))
	require.NoError(t, err)

	require.Equal(t, `{"usage":{"input_tokens":777,"output_tokens":11}}`, out.String())
	require.Equal(t, 777, s.inputTokens)
	require.Equal(t, 11, s.outputTokens)
}

func TestStrippedPath(t *testing.T) {
	require.Equal(t, "/v1/messages", strippedPath("/api/v1/messages"))
	require.Equal(t, "/v1/messages", strippedPath("/api/v1/proxy/v1/messages"))
	require.Equal(t, "/v1/messages", strippedPath("/v1/messages"))
}
