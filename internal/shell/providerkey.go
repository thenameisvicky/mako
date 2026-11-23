package shell

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/thenameisvicky/tokenly-mako/internal/keychain"
	"golang.org/x/term"
)

// handleSetProviderKey stores the user's own upstream provider API key in the
// system keychain.
//
// Mako forwards this key untouched; it is never sent to the proxy and never
// stored server-side. Each user therefore bills against their own account.
func handleSetProviderKey(args []string) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("/set-provider-key needs an interactive terminal")
	}

	fmt.Printf("\n  %sStore your Anthropic API key%s\n", colorBold, colorReset)
	fmt.Printf("  %sKept in your system keychain. Never sent to the proxy.%s\n\n", colorDim, colorReset)

	fmt.Printf("  %s>%s %sAPI key%s ", colorTan, colorReset, colorBold, colorReset)
	raw, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return err
	}
	fmt.Println()

	apiKey := strings.TrimSpace(string(raw))
	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	if err := keychain.SaveProviderKey(apiKey); err != nil {
		return err
	}

	fmt.Printf("\n  %s%s%s %sAPI key saved. Run %s/claude%s to start a session.%s\n\n",
		colorGreen, symbolSuccess, colorReset, colorDim, colorGreen, colorDim, colorReset)
	return nil
}
