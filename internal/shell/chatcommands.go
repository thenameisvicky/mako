package shell

import (
	"fmt"
	"sort"
	"strings"

	"github.com/thenameisvicky/tokenly-mako/internal/chatsync"
	"github.com/thenameisvicky/tokenly-mako/internal/clientconfig"
	"github.com/thenameisvicky/tokenly-mako/pkg/models"
)

// handleLocalChatSync uploads every Claude Code transcript on this machine to the
// proxy, where it is encrypted at rest and shared with the user's team.
func handleLocalChatSync(args []string) error {
	client, err := chatsync.NewClient(clientconfig.ProxyURL())
	if err != nil {
		return err
	}

	local, err := chatsync.Scan()
	if err != nil {
		return err
	}
	if len(local) == 0 {
		fmt.Printf("\n  %sNo Claude Code sessions found on this machine.%s\n\n", colorDim, colorReset)
		return nil
	}

	payloads := make([]models.ChatSessionPayload, 0, len(local))
	var skipped int
	for _, s := range local {
		payload, err := chatsync.Load(s)
		if err != nil {
			skipped++
			continue
		}
		payloads = append(payloads, payload)
	}

	if len(payloads) == 0 {
		return fmt.Errorf("found %d sessions but none could be read", len(local))
	}

	resp, err := client.Sync(payloads)
	if err != nil {
		return err
	}

	fmt.Printf("\n  %s%s%s %sSynced %s%d of %d%s sessions to the proxy.%s\n",
		colorGreen, symbolSuccess, colorReset, colorDim, colorReset, resp.Synced, resp.Total, colorDim, colorReset)
	if skipped > 0 {
		fmt.Printf("  %s%d local file(s) could not be read and were skipped.%s\n", colorDim, skipped, colorReset)
	}
	fmt.Printf("  %sRun %s/log%s on another machine to import them.%s\n\n",
		colorDim, colorGreen, colorDim, colorReset)
	return nil
}

// handleLocalChatList shows the transcripts the user's team has synced.
func handleLocalChatList(args []string) error {
	client, err := chatsync.NewClient(clientconfig.ProxyURL())
	if err != nil {
		return err
	}

	sessions, err := client.List()
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		fmt.Printf("\n  %sNo synced sessions yet. Run %s/push%s to upload this machine's chats.%s\n\n",
			colorDim, colorGreen, colorDim, colorReset)
		return nil
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].SyncedAt.After(sessions[j].SyncedAt)
	})

	fmt.Printf("\n  %sSynced sessions%s %s(%d)%s\n\n", colorBold, colorReset, colorDim, len(sessions), colorReset)
	for _, s := range sessions {
		owner := colorDim + "team" + colorReset
		if s.Mine {
			owner = colorGreen + "you" + colorReset
		}
		fmt.Printf("    %s%s%s  %s\n", colorCyan, shortID(s.SessionID), colorReset, s.Title)
		fmt.Printf("      %s%s · %d messages · %s · %s%s\n",
			colorDim, s.ProjectPath, s.MessageCount, s.SyncedAt.Format("2006-01-02 15:04"), owner, colorReset)
	}
	fmt.Printf("\n  %sImport one with %s/pull <session-id>%s\n\n", colorDim, colorGreen, colorReset)
	return nil
}

// handleLocalChatImport downloads a transcript and writes it into this machine's
// Claude Code directory, so it shows up in `claude --resume`.
func handleLocalChatImport(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: /pull <session-id>  (run /log to see ids)")
	}

	client, err := chatsync.NewClient(clientconfig.ProxyURL())
	if err != nil {
		return err
	}

	sessionID, err := resolveSessionID(client, args[0])
	if err != nil {
		return err
	}

	payload, err := client.Get(sessionID)
	if err != nil {
		return err
	}

	path, err := chatsync.Write(*payload)
	if err != nil {
		return err
	}

	fmt.Printf("\n  %s%s%s %sImported %s%s%s\n", colorGreen, symbolSuccess, colorReset, colorDim, colorReset, payload.Title, colorReset)
	fmt.Printf("  %s%s%s\n", colorDim, path, colorReset)
	fmt.Printf("  %sOpen it with %sclaude --resume%s\n\n", colorDim, colorGreen, colorReset)
	return nil
}

// resolveSessionID accepts either a full session id or the short prefix shown by
// /log, and fails clearly when a prefix is ambiguous.
func resolveSessionID(client *chatsync.Client, input string) (string, error) {
	sessions, err := client.List()
	if err != nil {
		return "", err
	}

	var matches []string
	for _, s := range sessions {
		if s.SessionID == input {
			return s.SessionID, nil
		}
		if strings.HasPrefix(s.SessionID, input) {
			matches = append(matches, s.SessionID)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no synced session matches %q", input)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("%q matches %d sessions, use more characters", input, len(matches))
	}
}

// shortID trims a session UUID to the prefix shown in listings
func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// handleLocalChatSyncCreate imports every transcript the team has synced that is
// not already on this machine, so a new machine can be brought up to date in one
// command rather than importing session by session.
func handleLocalChatSyncCreate(args []string) error {
	client, err := chatsync.NewClient(clientconfig.ProxyURL())
	if err != nil {
		return err
	}

	sessions, err := client.List()
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		fmt.Printf("\n  %sNothing synced yet. Run %s/push%s on the other machine first.%s\n\n",
			colorDim, colorGreen, colorDim, colorReset)
		return nil
	}

	// Checksums let us skip transcripts already here and unchanged, so running
	// this repeatedly costs one listing rather than a full re-download.
	local := chatsync.LocalChecksums()

	var created, updated, skipped, failed int
	fmt.Println()
	for _, s := range sessions {
		if have, ok := local[s.SessionID]; ok && have == s.Checksum && s.Checksum != "" {
			skipped++
			continue
		}
		existed := local[s.SessionID] != ""

		payload, err := client.Get(s.SessionID)
		if err != nil {
			failed++
			fmt.Printf("  %s%s%s %s%s · %v%s\n", colorRed, symbolError, colorReset,
				colorDim, shortID(s.SessionID), err, colorReset)
			continue
		}

		path, err := chatsync.Write(*payload)
		if err != nil {
			failed++
			fmt.Printf("  %s%s%s %s%s · %v%s\n", colorRed, symbolError, colorReset,
				colorDim, shortID(s.SessionID), err, colorReset)
			continue
		}

		if existed {
			updated++
		} else {
			created++
		}
		fmt.Printf("  %s%s%s %s%s%s  %s\n", colorGreen, symbolSuccess, colorReset,
			colorCyan, shortID(s.SessionID), colorReset, payload.Title)
		fmt.Printf("      %s%s%s\n", colorDim, path, colorReset)
	}

	fmt.Printf("\n  %s%d created · %d updated · %d already current",
		colorDim, created, updated, skipped)
	if failed > 0 {
		fmt.Printf(" · %s%d failed%s%s", colorRed, failed, colorReset, colorDim)
	}
	fmt.Printf("%s\n", colorReset)

	if created+updated > 0 {
		fmt.Printf("  %sOpen one with %sclaude --resume%s\n", colorDim, colorGreen, colorReset)
	}
	fmt.Println()
	return nil
}
