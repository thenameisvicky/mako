package chatsync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thenameisvicky/tokenly-mako/pkg/models"
)

const sampleTranscript = `{"type":"bridge-session","sessionId":"abc"}
{"type":"user","message":{"role":"user","content":[{"type":"text","text":"How do I ship this?"}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Like so."}]}}
{"type":"queue-operation","operation":"enqueue"}
`

func TestSummariseTitleAndCount(t *testing.T) {
	title, count := summarise([]byte(sampleTranscript))

	require.Equal(t, "How do I ship this?", title)
	require.Equal(t, 2, count, "only user and assistant turns are counted")
}

func TestSummariseHandlesPlainStringContent(t *testing.T) {
	title, count := summarise([]byte(`{"type":"user","message":{"role":"user","content":"plain text turn"}}`))
	require.Equal(t, "plain text turn", title)
	require.Equal(t, 1, count)
}

func TestSummariseEmptyTranscript(t *testing.T) {
	title, count := summarise(nil)
	require.Equal(t, "Untitled session", title)
	require.Zero(t, count)
}

// The directory name Claude Code uses must survive a decode/encode round trip,
// otherwise an imported session lands in the wrong project folder.
func TestProjectDirRoundTrip(t *testing.T) {
	original := "-home-vicky-Documents-Personal-mako"
	require.Equal(t, original, encodeProjectDir(decodeProjectDir(original)))

	require.Equal(t, "-home-vicky-code", encodeProjectDir("/home/vicky/code"))
	require.Equal(t, "-imported", encodeProjectDir(""))
}

func TestScanAndLoadFindsTranscripts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".claude", "projects", "-home-vicky-demo")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "session-1.jsonl"), []byte(sampleTranscript), 0o600))
	// A non-transcript file in the same directory must be ignored.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0o600))

	sessions, err := Scan()
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	require.Equal(t, "session-1", sessions[0].SessionID)

	payload, err := Load(sessions[0])
	require.NoError(t, err)
	require.Equal(t, "session-1", payload.SessionID)
	require.Equal(t, sampleTranscript, payload.Transcript)
	require.Equal(t, "How do I ship this?", payload.Title)
	require.Equal(t, 2, payload.MessageCount)
	require.NotEmpty(t, payload.Checksum)
}

func TestWriteRestoresTranscript(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := Write(models.ChatSessionPayload{
		SessionID:   "session-2",
		ProjectPath: "/home/vicky/demo",
		Transcript:  sampleTranscript,
	})
	require.NoError(t, err)

	written, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, sampleTranscript, string(written), "an imported transcript must be byte-identical")
	require.Equal(t, filepath.Join(home, ".claude", "projects", "-home-vicky-demo", "session-2.jsonl"), path)
}

func TestScanMissingDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	_, err := Scan()
	require.Error(t, err)
}
