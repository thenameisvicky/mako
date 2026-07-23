package shell

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/thenameisvicky/tokenly-cli/internal/agent"
	"github.com/thenameisvicky/tokenly-cli/internal/auth"
	"github.com/thenameisvicky/tokenly-cli/internal/config"
	"github.com/thenameisvicky/tokenly-cli/internal/proxy"
)

func InitRouter() *Router {
	r := NewRouter()

	r.RegisterCommand("/init", "Configure credentials securely", CommandHandlerFunc(func(args []string) error {
		if err := auth.Init(); err != nil {
			fmt.Printf("\n%s✖%s %sError:%s %v\n\n", colorRed, colorReset, colorBold, colorReset, err)
			return err
		}
		fmt.Printf("\n%s✔%s Credentials saved to system keychain.\n\n", colorGreen, colorReset)
		return nil
	}))

	r.RegisterCommand("/logout", "Clear stored credentials", CommandHandlerFunc(func(args []string) error {
		if err := auth.Logout(); err != nil {
			fmt.Printf("\n%s✖%s %sError:%s %v\n\n", colorRed, colorReset, colorBold, colorReset, err)
			return err
		}
		fmt.Printf("\n%s✔%s Logged out successfully.\n\n", colorGreen, colorReset)
		return nil
	}))

	r.RegisterCommand("/help", "Show this help menu", &helpHandler{router: r})

	r.RegisterCommand("/status", "Show current connection status", CommandHandlerFunc(func(args []string) error {
		handleStatus()
		return nil
	}))

	r.RegisterCommand("/connect", "Establish connection", CommandHandlerFunc(func(args []string) error {
		fmt.Println("\nConnecting to Tokenly proxy...")
		if err := proxy.Start(context.Background()); err != nil {
			fmt.Printf("✖ Error starting proxy: %v\n\n", err)
			return err
		}
		fmt.Println("✔ Connected to proxy successfully.")
		fmt.Println()
		return nil
	}))

	r.RegisterCommand("/disconnect", "Disconnect from service", CommandHandlerFunc(func(args []string) error {
		fmt.Println("\nDisconnecting from Tokenly proxy...")
		if err := proxy.Stop(); err != nil {
			fmt.Printf("✖ Error stopping proxy: %v\n\n", err)
			return err
		}
		fmt.Println("✔ Disconnected from proxy.")
		fmt.Println()
		return nil
	}))

	r.RegisterCommand("/claude", "Launch Claude agent", CommandHandlerFunc(func(args []string) error {
		return agent.LaunchClaude()
	}))

	r.RegisterCommand("/exit", "Exit the interactive shell", CommandHandlerFunc(func(args []string) error {
		return ErrExit
	}))

	return r
}

type helpHandler struct {
	router *Router
}

func (h *helpHandler) Execute(args []string) error {
	fmt.Println()
	fmt.Printf("%s┌────────────────────────────────────────────────────────────┐%s\n", colorBlue, colorReset)
	fmt.Printf("%s│%s  %sAvailable Commands%s                                        %s│%s\n", colorBlue, colorReset, colorBold, colorReset, colorBlue, colorReset)
	fmt.Printf("%s├────────────────────────────────────────────────────────────┤%s\n", colorBlue, colorReset)

	for _, cmd := range h.router.Commands() {
		leftText := fmt.Sprintf("  %-12s", cmd.Name)
		descText := cmd.Description
		totalTextLen := len(leftText) + len(descText)
		padding := 60 - totalTextLen
		if padding < 0 {
			padding = 0
		}

		fmt.Printf("%s│%s%s%s%s%s%s│%s\n",
			colorBlue, colorReset,
			leftText,
			colorDim, descText,
			colorReset+strings.Repeat(" ", padding),
			colorBlue, colorReset,
		)
	}

	fmt.Printf("%s└────────────────────────────────────────────────────────────┘%s\n", colorBlue, colorReset)
	fmt.Println()
	return nil
}

func handleStatus() {
	fmt.Println()
	fmt.Printf("%s┌────────────────────────────────────────────────────────────┐%s\n", colorBlue, colorReset)
	fmt.Printf("%s│%s  %sSystem Status%s                                             %s│%s\n", colorBlue, colorReset, colorBold, colorReset, colorBlue, colorReset)
	fmt.Printf("%s├────────────────────────────────────────────────────────────┤%s\n", colorBlue, colorReset)

	var authStatus string
	if auth.HasCredentials() {
		authStatus = "Connected (credentials loaded)"
	} else {
		authStatus = "Disconnected (no credentials)"
	}
	printStatusLine("Keychain", authStatus)

	if config.Exists() {
		cfg, err := config.Load()
		if err == nil {
			printStatusLine("Profile", cfg.CurrentProfile)
			printStatusLine("Default Agent", cfg.DefaultAgent)
			printStatusLine("Default Proxy", cfg.DefaultProxy)
			printStatusLine("Version", cfg.Version)
		} else {
			printStatusLine("Config", fmt.Sprintf("Error loading config: %v", err))
		}
	} else {
		printStatusLine("Config", "Not configured (run /init)")
	}

	pStatus, err := proxy.Status()
	if err == nil {
		printStatusLine("Proxy State", string(pStatus.State))
		if pStatus.State == proxy.StateConnected {
			printStatusLine("Proxy Uptime", pStatus.Uptime.Round(time.Second).String())
		}
	} else {
		printStatusLine("Proxy Status", fmt.Sprintf("Error: %v", err))
	}

	fmt.Printf("%s└────────────────────────────────────────────────────────────┘%s\n", colorBlue, colorReset)
	fmt.Println()
}

func printStatusLine(label, value string) {
	leftText := fmt.Sprintf("  %-15s %s", label+":", value)
	padding := 60 - len(leftText)
	if padding < 0 {
		padding = 0
	}
	fmt.Printf("%s│%s%s%s%s│%s\n", colorBlue, colorReset, leftText, strings.Repeat(" ", padding), colorBlue, colorReset)
}