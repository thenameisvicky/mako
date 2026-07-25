package shell

import (
	"context"
	"fmt"
	"time"

	"github.com/thenameisvicky/tokenly-cli/internal/agent_runtime"
	"github.com/thenameisvicky/tokenly-cli/internal/auth"
	"github.com/thenameisvicky/tokenly-cli/internal/proxy"
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

	r.RegisterCommand("/connect", "Establish connection", CommandHandlerFunc(func(args []string) error {
		fmt.Printf("\n  %sConnecting to Tokenly proxy…%s\n", colorDim, colorReset)
		if err := proxy.Start(context.Background()); err != nil {
			fmt.Printf("  %sCould not start proxy: %v%s\n\n", colorRed, err, colorReset)
			return err
		}
		fmt.Printf("  %sConnected to proxy.%s\n\n", colorDim, colorReset)
		return nil
	}))

	r.RegisterCommand("/disconnect", "Disconnect from service", CommandHandlerFunc(func(args []string) error {
		fmt.Printf("\n  %sDisconnecting from Tokenly proxy…%s\n", colorDim, colorReset)
		if err := proxy.Stop(); err != nil {
			fmt.Printf("  %sCould not stop proxy: %v%s\n\n", colorRed, err, colorReset)
			return err
		}
		fmt.Printf("  %sDisconnected from proxy.%s\n\n", colorDim, colorReset)
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

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
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
	fmt.Printf("  %sSession Metrics%s\n", colorBold, colorReset)
	fmt.Println()

	var avgTTFB time.Duration
	if proxy.GlobalMetrics.Requests > 0 {
		avgTTFB = time.Duration(proxy.GlobalMetrics.TotalTTFB/proxy.GlobalMetrics.Requests) * time.Microsecond
	}
	fmt.Printf("    %-20s %d\n", "Requests:", proxy.GlobalMetrics.Requests)
	fmt.Printf("    %-20s %v\n", "Avg Latency:", avgTTFB.Round(time.Millisecond))
	fmt.Printf("    %-20s %s\n", "Bytes Uploaded:", formatBytes(proxy.GlobalMetrics.BytesUploaded))
	fmt.Printf("    %-20s %s\n", "Bytes Downloaded:", formatBytes(proxy.GlobalMetrics.BytesDownloaded))
	fmt.Println()
}