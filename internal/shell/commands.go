package shell

import (
	"fmt"

	"github.com/thenameisvicky/tokenly-mako/internal/agent_runtime"
	"github.com/thenameisvicky/tokenly-mako/internal/auth"
)

func InitRouter() *Router {
	r := NewRouter()

	r.RegisterCommand("/init", "Configure credentials securely", CommandHandlerFunc(func(args []string) error {
		if err := auth.Init(); err != nil {
			fmt.Printf("\n  %s%s%s %s%v%s\n\n", colorRed, symbolError, colorReset, colorRed, err, colorReset)
			return err
		}
		fmt.Printf("\n  %s%s%s %sCredentials saved to your system keychain.%s\n\n", colorGreen, symbolSuccess, colorReset, colorDim, colorReset)
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
	fmt.Printf("  %sTokenly%s %s— slash commands%s\n", colorBold, colorReset, colorDim, colorReset)
	fmt.Println()

	for _, cmd := range h.router.Commands() {
		fmt.Printf("    %s%-14s%s %s%s%s\n", colorGreen, cmd.Name, colorReset, colorDim, cmd.Description, colorReset)
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
		fmt.Printf("   %s●%s  %sKeychain:%s      Connected (credentials loaded)\n", colorGreen, colorReset, colorBold, colorReset)
	} else {
		fmt.Printf("   %s●%s  %sKeychain:%s      Disconnected (no credentials)\n", colorOrange, colorReset, colorBold, colorReset)
	}
	fmt.Println()
}