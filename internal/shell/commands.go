package shell

import (
	"context"
	"fmt"
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
			fmt.Printf("\n%s%s %sError: %v%s\n\n", colorBold, symbolError, colorRed, err, colorReset)
			return err
		}
		fmt.Printf("\n%s%s %sCredentials saved to system keychain.%s\n\n", colorBold, symbolSuccess, colorGreen, colorReset)
		return nil
	}))

	r.RegisterCommand("/logout", "Clear stored credentials", CommandHandlerFunc(func(args []string) error {
		if err := auth.Logout(); err != nil {
			fmt.Printf("\n%s%s %sError: %v%s\n\n", colorBold, symbolError, colorRed, err, colorReset)
			return err
		}
		fmt.Printf("\n%s%s %sLogged out successfully.%s\n\n", colorBold, symbolSuccess, colorGreen, colorReset)
		return nil
	}))

	r.RegisterCommand("/help", "Show this help menu", &helpHandler{router: r})

	r.RegisterCommand("/status", "Show current connection status", CommandHandlerFunc(func(args []string) error {
		handleStatus()
		return nil
	}))

	r.RegisterCommand("/connect", "Establish connection", CommandHandlerFunc(func(args []string) error {
		fmt.Printf("\n%s Establishing connection to Tokenly proxy...\n", symbolInfo)
		if err := proxy.Start(context.Background()); err != nil {
			fmt.Printf("%s%s %sError starting proxy: %v%s\n\n", colorBold, symbolError, colorRed, err, colorReset)
			return err
		}
		fmt.Printf("%s%s %sConnected to proxy successfully.%s\n\n", colorBold, symbolSuccess, colorGreen, colorReset)
		return nil
	}))

	r.RegisterCommand("/disconnect", "Disconnect from service", CommandHandlerFunc(func(args []string) error {
		fmt.Printf("\n%s Disconnecting from Tokenly proxy...\n", symbolInfo)
		if err := proxy.Stop(); err != nil {
			fmt.Printf("%s%s %sError stopping proxy: %v%s\n\n", colorBold, symbolError, colorRed, err, colorReset)
			return err
		}
		fmt.Printf("%s%s %sDisconnected from proxy.%s\n\n", colorBold, symbolSuccess, colorGreen, colorReset)
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
	fmt.Printf("%s%s  Tokenly Commands:%s\n", colorBold, colorGreen, colorReset)
	fmt.Println()

	for _, cmd := range h.router.Commands() {
		fmt.Printf("    %s%-15s%s  %s%s%s\n", colorBold+colorGreen, cmd.Name, colorReset, colorDim, cmd.Description, colorReset)
	}

	fmt.Println()
	fmt.Printf("  %sUse Tab to autocomplete commands.%s\n", colorDim, colorReset)
	fmt.Println()
	return nil
}

func handleStatus() {
	fmt.Println()
	fmt.Printf("%s%s  System Status:%s\n", colorBold, colorGreen, colorReset)
	fmt.Println()

	if auth.HasCredentials() {
		fmt.Printf("   %s●%s  %sKeychain:%s      Connected (credentials loaded)\n", colorGreen, colorReset, colorBold, colorReset)
	} else {
		fmt.Printf("   %s●%s  %sKeychain:%s      Disconnected (no credentials)\n", colorOrange, colorReset, colorBold, colorReset)
	}

	// Configuration
	if config.Exists() {
		cfg, err := config.Load()
		if err == nil {
			fmt.Printf("   %s●%s  %sProfile:%s       %s\n", colorBlue, colorReset, colorBold, colorReset, cfg.CurrentProfile)
			fmt.Printf("   %s●%s  %sDefault Agent:%s %s\n", colorBlue, colorReset, colorBold, colorReset, cfg.DefaultAgent)
			fmt.Printf("   %s●%s  %sDefault Proxy:%s %s\n", colorBlue, colorReset, colorBold, colorReset, cfg.DefaultProxy)
			fmt.Printf("   %s●%s  %sVersion:%s       %s\n", colorBlue, colorReset, colorBold, colorReset, cfg.Version)
		} else {
			fmt.Printf("   %s●%s  %sConfig:%s        %sError: %v%s\n", colorRed, colorReset, colorBold, colorReset, colorRed, err, colorReset)
		}
	} else {
		fmt.Printf("   %s●%s  %sConfig:%s        Not configured (run /init)\n", colorOrange, colorReset, colorBold, colorReset)
	}

	// Proxy status
	pStatus, err := proxy.Status()
	if err == nil {
		stateCol := colorGreen
		if pStatus.State != proxy.StateConnected {
			stateCol = colorOrange
		}
		fmt.Printf("   %s●%s  %sProxy State:%s   %s%s%s\n", stateCol, colorReset, colorBold, colorReset, stateCol, pStatus.State, colorReset)
		if pStatus.State == proxy.StateConnected {
			fmt.Printf("   %s●%s  %sProxy Uptime:%s  %s\n", colorGreen, colorReset, colorBold, colorReset, pStatus.Uptime.Round(time.Second).String())
		}
	} else {
		fmt.Printf("   %s●%s  %sProxy Status:%s  Offline (%v)\n", colorOrange, colorReset, colorBold, colorReset, err)
	}
	fmt.Println()
}
