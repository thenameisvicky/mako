package chatsync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thenameisvicky/tokenly-mako/pkg/models"
)

// Claude Code stores each conversation as a JSON Lines file at
//
//	~/.claude/projects/<sanitised-project-path>/<session-uuid>.jsonl
//
// The directory name is the project's absolute path with the separators replaced
// by dashes. Each line is one record: user turns, assistant turns, tool calls and
// session bookkeeping. We move the file verbatim so an imported session replays
// in Claude Code exactly as it did on the original machine.

// ProjectsDir returns the local Claude Code transcript directory
func ProjectsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("unable to locate home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

// LocalSession is a transcript found on this machine.
type LocalSession struct {
	SessionID    string
	ProjectPath  string
	Path         string
	Title        string
	MessageCount int
	SizeBytes    int64
	Checksum     string
}

// Scan walks the local transcript directory and returns every session found.
func Scan() ([]LocalSession, error) {
	root, err := ProjectsDir()
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("no Claude Code transcripts found at %s", root)
	}

	var sessions []LocalSession
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries rather than aborting the scan
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}

		stat, err := d.Info()
		if err != nil {
			return nil
		}

		session := LocalSession{
			SessionID:   strings.TrimSuffix(d.Name(), ".jsonl"),
			ProjectPath: decodeProjectDir(filepath.Base(filepath.Dir(path))),
			Path:        path,
			SizeBytes:   stat.Size(),
		}
		sessions = append(sessions, session)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return sessions, nil
}

// Load reads a session's transcript and derives its title, message count and
// checksum. The checksum lets a re-sync skip files that have not changed.
func Load(s LocalSession) (models.ChatSessionPayload, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return models.ChatSessionPayload{}, fmt.Errorf("failed to read %s: %w", s.Path, err)
	}

	sum := sha256.Sum256(data)
	title, count := summarise(data)

	return models.ChatSessionPayload{
		SessionID:    s.SessionID,
		ProjectPath:  s.ProjectPath,
		Title:        title,
		Transcript:   string(data),
		MessageCount: count,
		Checksum:     hex.EncodeToString(sum[:]),
	}, nil
}

// LocalChecksums maps session id to the checksum of the copy on this machine,
// so an import can skip transcripts that are already here and unchanged.
func LocalChecksums() map[string]string {
	out := map[string]string{}

	sessions, err := Scan()
	if err != nil {
		return out // nothing local yet; every remote session is new
	}
	for _, s := range sessions {
		payload, err := Load(s)
		if err != nil {
			continue
		}
		out[s.SessionID] = payload.Checksum
	}
	return out
}

// Write restores a downloaded transcript into the local Claude Code directory so
// it appears in `claude --resume` on this machine.
func Write(payload models.ChatSessionPayload) (string, error) {
	root, err := ProjectsDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(root, encodeProjectDir(payload.ProjectPath))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create %s: %w", dir, err)
	}

	target := filepath.Join(dir, payload.SessionID+".jsonl")
	if err := os.WriteFile(target, []byte(payload.Transcript), 0o600); err != nil {
		return "", fmt.Errorf("failed to write %s: %w", target, err)
	}
	return target, nil
}

// summarise extracts a human-readable title from the first user turn, and counts
// the conversational records in the transcript.
func summarise(data []byte) (string, int) {
	var title string
	count := 0

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var record struct {
			Type    string `json:"type"`
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		if record.Type != "user" && record.Type != "assistant" {
			continue
		}
		count++

		if title == "" && record.Type == "user" {
			title = firstText(record.Message.Content)
		}
	}

	if title == "" {
		title = "Untitled session"
	}
	return title, count
}

// firstText pulls the leading text out of a message content field, which is
// either a plain string or an array of typed content blocks.
func firstText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var plain string
	if err := json.Unmarshal(raw, &plain); err == nil {
		return truncate(plain)
	}

	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		for _, b := range blocks {
			if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
				return truncate(b.Text)
			}
		}
	}
	return ""
}

func truncate(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) > 80 {
		return s[:77] + "..."
	}
	return s
}

// encodeProjectDir mirrors Claude Code's directory naming: an absolute path with
// every separator replaced by a dash.
func encodeProjectDir(projectPath string) string {
	if projectPath == "" {
		return "-imported"
	}
	return strings.ReplaceAll(projectPath, string(filepath.Separator), "-")
}

// decodeProjectDir reverses encodeProjectDir. The mapping is lossy - a directory
// whose name genuinely contains a dash is indistinguishable from a separator -
// so the result is used for display and re-import, never for local file access.
func decodeProjectDir(dir string) string {
	return strings.ReplaceAll(dir, "-", string(filepath.Separator))
}
