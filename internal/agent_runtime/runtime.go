package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/thenameisvicky/tokenly-cli/internal/proxy"
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


func LaunchClaude() error {
	status, err := proxy.Status()
	if err != nil {
		return fmt.Errorf("failed to check proxy status: %w", err)
	}

	proxyStartedByAgent := false
	if status.State != proxy.StateConnected {
		fmt.Printf("  %sStarting Tokenly proxy for Claude connection…%s\n", colorDim, colorReset)
		if err := proxy.Start(context.Background()); err != nil {
			return fmt.Errorf("failed to start proxy: %w", err)
		}
		proxyStartedByAgent = true
		fmt.Printf("  %sProxy is running.%s\n", colorDim, colorReset)
	}

	claudePath, err := exec.LookPath("claude")
	if err != nil {
		if proxyStartedByAgent {
			_ = proxy.Stop()
		}
		return fmt.Errorf("claude CLI command not found on PATH. Please install Claude Code first")
	}

	fmt.Print("\033[?1049h\033[H\033[2J")

	cmd := exec.Command(claudePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	localProxy := "http://" + proxy.GetLocalAddr()
	cmd.Env = append(os.Environ(),
		"HTTP_PROXY="+localProxy,
		"HTTPS_PROXY="+localProxy,
		"http_proxy="+localProxy,
		"https_proxy="+localProxy,
		"ANTHROPIC_BASE_URL="+localProxy,
		"ANTHROPIC_API_URL="+localProxy,
	)

	runErr := cmd.Run()

	fmt.Print("\033[?1049l")

	if proxyStartedByAgent {
		fmt.Printf("  %sStopping Tokenly proxy…%s\n", colorDim, colorReset)
		if err := proxy.Stop(); err != nil {
			fmt.Printf("  %sError stopping proxy: %v%s\n", colorRed, err, colorReset)
		} else {
			fmt.Printf("  %sProxy stopped.%s\n", colorDim, colorReset)
		}
	}

	fmt.Printf("  %sReturned from Claude Code shell.%s\n\n", colorDim, colorReset)

	if runErr != nil {
		return fmt.Errorf("claude shell exited with error: %w", runErr)
	}
	return nil
}
