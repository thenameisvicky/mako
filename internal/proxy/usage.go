package proxy

import (
	"bytes"
	"io"
	"strconv"
)

// usageSniffer counts tokens by watching the response stream go past.
//
// It writes through to the real destination and never buffers the body: each
// chunk is scanned for "input_tokens"/"output_tokens" and discarded. This covers
// both shapes Anthropic returns - a single JSON object for normal requests, and
// message_start / message_delta SSE frames when streaming - without the proxy
// having to understand either format.
type usageSniffer struct {
	dst io.Writer

	inputTokens  int
	outputTokens int

	// carry holds the tail of the previous chunk so a counter split across a
	// chunk boundary is still matched.
	carry []byte
}

// maxCarry is the longest key plus a generous value, enough to span any split.
const maxCarry = 64

var (
	inputKey  = []byte(`"input_tokens":`)
	outputKey = []byte(`"output_tokens":`)
)

func newUsageSniffer(dst io.Writer) *usageSniffer {
	return &usageSniffer{dst: dst, carry: make([]byte, 0, maxCarry)}
}

func (u *usageSniffer) Write(p []byte) (int, error) {
	// Scan the carry-over tail joined to this chunk, but write only this chunk.
	scan := p
	if len(u.carry) > 0 {
		scan = append(append(make([]byte, 0, len(u.carry)+len(p)), u.carry...), p...)
	}

	if v, ok := lastIntAfter(scan, inputKey); ok {
		u.inputTokens = v
	}
	// Streaming sends a running output_tokens count; the last frame wins.
	if v, ok := lastIntAfter(scan, outputKey); ok {
		u.outputTokens = v
	}

	if len(scan) > maxCarry {
		u.carry = append(u.carry[:0], scan[len(scan)-maxCarry:]...)
	} else {
		u.carry = append(u.carry[:0], scan...)
	}

	n, err := u.dst.Write(p)
	return n, err
}

// lastIntAfter returns the final integer following key in data
func lastIntAfter(data, key []byte) (int, bool) {
	idx := bytes.LastIndex(data, key)
	if idx < 0 {
		return 0, false
	}
	i := idx + len(key)
	for i < len(data) && data[i] == ' ' {
		i++
	}
	start := i
	for i < len(data) && data[i] >= '0' && data[i] <= '9' {
		i++
	}
	if i == start {
		return 0, false
	}
	v, err := strconv.Atoi(string(data[start:i]))
	if err != nil {
		return 0, false
	}
	return v, true
}
