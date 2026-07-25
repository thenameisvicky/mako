package agent

import (
	"fmt"
	"os"
	"os/exec"
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
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude CLI command not found on PATH. Please install Claude Code first")
	}

	fmt.Print("\033[?1049h\033[H\033[2J")

	cmd := exec.Command(claudePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	runErr := cmd.Run()

	fmt.Print("\033[?1049l")

	fmt.Printf("  %sReturned from Claude Code shell.%s\n\n", colorDim, colorReset)

	if runErr != nil {
		return fmt.Errorf("claude shell exited with error: %w", runErr)
	}
	return nil
}
