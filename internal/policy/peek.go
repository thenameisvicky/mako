package policy

import (
	"bytes"
	"io"
	"strconv"
	"sync"
)

// peekLimit caps how much of the request body we inspect.
//
// Anthropic request bodies put "model" and "max_tokens" near the front, but the
// message array that follows can be megabytes. Reading a bounded prefix keeps
// policy evaluation O(8KB) instead of O(body), which is what lets the hot path
// stay inside its 1ms budget on large-context requests.
const peekLimit = 8 * 1024

var peekPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, peekLimit)
		return &b
	},
}

// RequestFacts is what the policy engine needs from a request body.
type RequestFacts struct {
	Model     string
	MaxTokens int
	Stream    bool
}

// PeekBody extracts policy-relevant fields from the front of a request body and
// returns a reader that replays the whole body unchanged.
//
// The returned reader must be fully consumed or closed by the caller. The body is
// never buffered in full, so a 10MB request costs the same peek as a 1KB one.
func PeekBody(body io.Reader) (RequestFacts, io.Reader) {
	var facts RequestFacts
	if body == nil {
		return facts, nil
	}

	bufPtr := peekPool.Get().(*[]byte)
	buf := *bufPtr

	n, _ := io.ReadFull(body, buf)
	prefix := buf[:n]

	facts.Model = stringField(prefix, "model")
	facts.MaxTokens = intField(prefix, "max_tokens")
	facts.Stream = boolField(prefix, "stream")

	// Copy the prefix out before the pooled buffer is reused, then stitch it back
	// in front of the untouched remainder of the stream.
	replay := make([]byte, n)
	copy(replay, prefix)
	*bufPtr = buf
	peekPool.Put(bufPtr)

	return facts, io.MultiReader(bytes.NewReader(replay), body)
}

// findKey locates the value position for a top-level JSON key in a byte prefix.
// It is a deliberate substring scan rather than a JSON parse: the prefix is
// usually truncated mid-document, which any real parser would reject.
func findKey(data []byte, key string) int {
	needle := append(append([]byte{'"'}, key...), '"')
	idx := bytes.Index(data, needle)
	if idx < 0 {
		return -1
	}
	i := idx + len(needle)
	for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
		i++
	}
	if i >= len(data) || data[i] != ':' {
		return -1
	}
	i++
	for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
		i++
	}
	if i >= len(data) {
		return -1
	}
	return i
}

func stringField(data []byte, key string) string {
	i := findKey(data, key)
	if i < 0 || data[i] != '"' {
		return ""
	}
	i++
	start := i
	for i < len(data) && data[i] != '"' {
		if data[i] == '\\' {
			i++
		}
		i++
	}
	if i >= len(data) {
		return "" // value was cut off by the peek limit
	}
	return string(data[start:i])
}

func intField(data []byte, key string) int {
	i := findKey(data, key)
	if i < 0 {
		return 0
	}
	start := i
	for i < len(data) && data[i] >= '0' && data[i] <= '9' {
		i++
	}
	if i == start || i >= len(data) {
		return 0
	}
	v, err := strconv.Atoi(string(data[start:i]))
	if err != nil {
		return 0
	}
	return v
}

func boolField(data []byte, key string) bool {
	i := findKey(data, key)
	return i >= 0 && bytes.HasPrefix(data[i:], []byte("true"))
}
