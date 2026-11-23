package agent

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/thenameisvicky/tokenly-mako/internal/clientconfig"
	"github.com/thenameisvicky/tokenly-mako/internal/keychain"
)

const (
	colorReset  = "\033[0m"
	colorDim    = "\033[38;5;244m"
	colorTan    = "\033[38;5;215m"
	colorGreen  = "\033[38;5;84m"
	colorRed    = "\033[38;5;203m"
	colorBold   = "\033[1m"
	symbolError = ""
)

// LaunchClaude starts Claude Code pointed at the Mako proxy.
//
// Two separate credentials are in play and they are deliberately kept apart:
//
//	ANTHROPIC_API_KEY      the user's OWN provider key, forwarded untouched
//	X-Mako-Token        the Mako token, consumed by the proxy
//
// The proxy does not substitute credentials. It authenticates the caller,
// applies team policy and token budgets, records usage, and passes the request
// through. Every user therefore bills against their own provider account.
func LaunchClaude() error {
	_, token, err := keychain.Load()
	if err != nil || token == "" {
		return fmt.Errorf("you are not logged in. Please run /login first")
	}

	providerKey, err := keychain.LoadProviderKey()
	if err != nil || providerKey == "" {
		// Fall back to the ambient key so an existing shell setup keeps working.
		providerKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if providerKey == "" {
		return fmt.Errorf("no Anthropic API key found. Run /set-provider-key to store yours, or set ANTHROPIC_API_KEY")
	}

	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude CLI command not found on PATH. Please install Claude Code first")
	}

	fmt.Print("\033[?1049h\033[H\033[2J")

	cmd := exec.Command(claudePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	baseURL := clientconfig.ProxyURL() + "/api"
	cmd.Env = append(os.Environ(),
		"ANTHROPIC_BASE_URL="+baseURL,
		"ANTHROPIC_API_URL="+baseURL,
		"ANTHROPIC_API_KEY="+providerKey,
		"ANTHROPIC_CUSTOM_HEADERS=X-Mako-Token: "+token,
	)

	runErr := cmd.Run()

	fmt.Print("\033[?1049l")

	fmt.Printf("  %sReturned from Claude Code shell.%s\n\n", colorDim, colorReset)

	if runErr != nil {
		return fmt.Errorf("claude shell exited with error: %w", runErr)
	}
	return nil
}
