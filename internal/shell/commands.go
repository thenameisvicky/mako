package shell

import (
	"fmt"
	"os"

	"github.com/thenameisvicky/tokenly-mako/internal/agent_runtime"
	"github.com/thenameisvicky/tokenly-mako/internal/auth"
	"github.com/thenameisvicky/tokenly-mako/internal/clientconfig"
	"github.com/thenameisvicky/tokenly-mako/internal/keychain"
)

func InitRouter() *Router {
	r := NewRouter()

	r.RegisterCommand("/login", "Log in to the Mako proxy", CommandHandlerFunc(func(args []string) error {
		if err := auth.Login(); err != nil {
			fmt.Printf("\n  %s%s%s %s%v%s\n\n", colorRed, symbolError, colorReset, colorRed, err, colorReset)
			return err
		}
		fmt.Printf("\n  %s%s%s %sSuccessfully logged in. Token saved to your system keychain.%s\n\n", colorGreen, symbolSuccess, colorReset, colorDim, colorReset)
		return nil
	}))

	r.RegisterCommand("/logout", "Clear stored credentials", CommandHandlerFunc(func(args []string) error {
		if err := auth.Logout(); err != nil {
			fmt.Printf("\n  %s%s%s %s%v%s\n\n", colorRed, symbolError, colorReset, colorRed, err, colorReset)
			return err
		}
		fmt.Printf("\n  %s%s%s %sSigned out. Credentials cleared from your keychain.%s\n\n", colorGreen, symbolSuccess, colorReset, colorDim, colorReset)
		return nil
	}))

	r.RegisterCommand("/help", "Show this help menu", &helpHandler{router: r})

	r.RegisterCommand("/status", "Show current connection status", CommandHandlerFunc(func(args []string) error {
		handleStatus()
		return nil
	}))

	r.RegisterCommand("/claude", "Launch Claude agent", CommandHandlerFunc(func(args []string) error {
		return agent.LaunchClaude()
	}))

	r.RegisterCommand("/set-provider-key", "Store your own Anthropic API key", CommandHandlerFunc(handleSetProviderKey))

	r.RegisterCommand("/local-chat-sync", "Upload this machine's Claude Code chats to the proxy",
		newAsyncCommand("Syncing local chats", handleLocalChatSync))

	r.RegisterCommand("/local-chat-list", "List chats your team has synced",
		newAsyncCommand("Fetching synced chats", handleLocalChatList))

	r.RegisterCommand("/local-chat-sync-create", "Import every synced chat onto this machine",
		newAsyncCommand("Creating local chats", handleLocalChatSyncCreate))

	r.RegisterCommand("/local-chat-import", "Import one synced chat by session id",
		newAsyncCommand("Importing chat", handleLocalChatImport))

	r.RegisterCommand("/exit", "Exit the interactive shell", CommandHandlerFunc(func(args []string) error {
		return ErrExit
	}))

	r.RegisterCommand("/clean", "Clear the terminal screen", CommandHandlerFunc(func(args []string) error {
		fmt.Print("\033[H\033[2J\033[3J")
		return nil
	}))

	r.RegisterCommand("/clear", "Clear the terminal screen", CommandHandlerFunc(func(args []string) error {
		fmt.Print("\033[H\033[2J\033[3J")
		return nil
	}))

	return r
}

type helpHandler struct {
	router *Router
}

func (h *helpHandler) Execute(args []string) error {
	fmt.Println()
	fmt.Printf("  %sMako%s %s— slash commands%s\n", colorBold, colorReset, colorDim, colorReset)
	fmt.Println()

	// Size the name column to the longest command so descriptions stay aligned.
	width := 14
	for _, cmd := range h.router.Commands() {
		if len(cmd.Name) > width {
			width = len(cmd.Name)
		}
	}

	for _, cmd := range h.router.Commands() {
		fmt.Printf("    %s%-*s%s  %s%s%s\n", colorGreen, width, cmd.Name, colorReset, colorDim, cmd.Description, colorReset)
	}

	fmt.Println()
	fmt.Printf("  %sTab%s %sto autocomplete · %s?%s %sfor shortcuts%s\n", colorBold, colorReset, colorDim, colorBold, colorReset, colorDim, colorReset)
	fmt.Println()
	return nil
}

func handleStatus() {
	fmt.Println()
	fmt.Printf("  %sStatus%s\n", colorBold, colorReset)
	fmt.Println()

	if auth.HasCredentials() {
		fmt.Printf("   %s●%s  %sMako:%s          Signed in\n", colorGreen, colorReset, colorBold, colorReset)
	} else {
		fmt.Printf("   %s●%s  %sMako:%s          Not signed in — run /login\n", colorOrange, colorReset, colorBold, colorReset)
	}

	// The provider key is the user's own and is forwarded untouched; the proxy
	// never stores or substitutes it.
	if key, err := keychain.LoadProviderKey(); err == nil && key != "" {
		fmt.Printf("   %s●%s  %sProvider key:%s  Stored in keychain\n", colorGreen, colorReset, colorBold, colorReset)
	} else if os.Getenv("ANTHROPIC_API_KEY") != "" {
		fmt.Printf("   %s●%s  %sProvider key:%s  Using ANTHROPIC_API_KEY from environment\n", colorGreen, colorReset, colorBold, colorReset)
	} else {
		fmt.Printf("   %s●%s  %sProvider key:%s  Not set — run /set-provider-key\n", colorOrange, colorReset, colorBold, colorReset)
	}

	fmt.Printf("   %s●%s  %sProxy:%s         %s\n", colorDim, colorReset, colorBold, colorReset, clientconfig.ProxyURL())
	fmt.Println()
}
