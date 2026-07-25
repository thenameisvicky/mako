package shell

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"os/user"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/thenameisvicky/tokenly-cli/internal/auth"
	"golang.org/x/term"
)

const (
	colorReset  = "\033[0m"
	colorDim    = "\033[38;5;244m"
	colorBold   = "\033[1m"
	colorPurple = "\033[38;5;141m"
	colorBlue   = "\033[38;5;75m"
	colorOrange = "\033[38;5;208m"
	colorGreen  = "\033[38;5;84m"
	colorRed    = "\033[38;5;203m"
	colorYellow = "\033[38;5;220m"
	colorCyan   = "\033[38;5;80m"
)

const (
	symbolInfo    = "ℹ"
	symbolSuccess = "✔"
	symbolError   = "✖"
)

type AsyncCommandHandler interface {
	CommandHandler
	IsAsync() bool
	LoadingMessage() string
}

type Shell struct {
	router   *Router
	renderer *Renderer
	prompt   string
}

type stdReadWriter struct{}

func (stdReadWriter) Read(p []byte) (n int, err error) {
	return os.Stdin.Read(p)
}

func (stdReadWriter) Write(p []byte) (n int, err error) {
	return os.Stdout.Write(p)
}

func printHorizontalLine() {
	width := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		width = w
	}
	fmt.Printf("%s%s%s\n", colorDim, strings.Repeat("─", width), colorReset)
}

func NewShell(router *Router) *Shell {
	return &Shell{
		router:   router,
		renderer: &Renderer{},
		prompt:   "  ",
	}
}

func Start() error {
	sh := NewShell(InitRouter())
	return sh.Run()
}

func (s *Shell) Run() error {
	fd := int(os.Stdin.Fd())
	isTTY := term.IsTerminal(fd)

	s.renderer.PrintWelcomeBox(auth.HasCredentials())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Printf("\n%s%s %sInterrupted. Exiting...%s\n", colorBold, colorYellow, symbolInfo, colorReset)
		os.Exit(0)
	}()

	var t *term.Terminal
	if isTTY {
		t = term.NewTerminal(stdReadWriter{}, s.prompt)
		t.AutoCompleteCallback = s.makeCompleter()
	}

	upperLineNeeded := true

	for {
		var input string

		if isTTY {
			if upperLineNeeded {
				printHorizontalLine()
				fmt.Println()
				printHorizontalLine()
				fmt.Print("\033[2A\r")
			}

			oldState, err := term.MakeRaw(fd)
			if err != nil {
				return err
			}
			input, err = t.ReadLine()
			term.Restore(fd, oldState)

			if err != nil {
				if errors.Is(err, io.EOF) {
					fmt.Println()
					break
				}
				return err
			}

			input = strings.TrimSpace(input)
			if input == "" {
				fmt.Print("\033[1A\r")
				upperLineNeeded = false
				continue
			}

			printHorizontalLine()
			upperLineNeeded = true
		} else {
			scanner := bufio.NewScanner(os.Stdin)
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					return err
				}
				break
			}
			input = scanner.Text()
			input = strings.TrimSpace(input)
			if input == "" {
				continue
			}
		}

		parts := strings.Fields(input)
		if len(parts) == 0 {
			continue
		}
		cmdName := parts[0]
		cmdArgs := parts[1:]

		if s.router.HasCommand(cmdName) {
			handler := s.router.commands[cmdName].Handler

			var cmdErr error
			if asyncCmd, ok := handler.(AsyncCommandHandler); ok && asyncCmd.IsAsync() {
				spinner := NewSpinner()
				spinner.Start(asyncCmd.LoadingMessage())

				errChan := make(chan error, 1)
				go func() {
					errChan <- s.router.Route(cmdName, cmdArgs)
				}()

				cmdErr = <-errChan
				spinner.Stop()
			} else {
				cmdErr = s.router.Route(cmdName, cmdArgs)
			}

			if cmdErr != nil {
				if errors.Is(cmdErr, ErrExit) {
					fmt.Printf("\n%s%s %sTokenly session ended. Bye!%s\n\n", colorBold, symbolSuccess, colorGreen, colorReset)
					return nil
				}
				fmt.Printf("\n%s%s %s%s\n\n", colorBold, symbolError, colorRed, cmdErr.Error())
			}
		} else {
			if strings.HasPrefix(cmdName, "/") {
				fmt.Printf("\n%s%s %sUnrecognized command: %s. Type %s/help%s for available commands.\n\n", colorBold, symbolError, colorRed, cmdName, colorBold, colorReset)
			} else {
				fmt.Printf("\n%s%s You entered: %s%s %s(Type /help for available commands)%s\n\n", colorBold, colorGreen, colorReset, input, colorDim, colorReset)
			}
		}
	}
	return nil
}

func (s *Shell) makeCompleter() func(line string, pos int, key rune) (newLine string, newPos int, ok bool) {
	return func(line string, pos int, key rune) (newLine string, newPos int, ok bool) {
		if key != '\t' {
			return "", 0, false
		}

		if !strings.HasPrefix(line, "/") {
			return "", 0, false
		}

		var matches []string
		for _, cmd := range s.router.Commands() {
			if strings.HasPrefix(cmd.Name, line) {
				matches = append(matches, cmd.Name)
			}
		}

		if len(matches) == 1 {
			completed := matches[0] + " "
			return completed, len(completed), true
		}

		return "", 0, false
	}
}

type Renderer struct{}

func visualWidth(s string) int {
	return utf8.RuneCountInString(s)
}

func (r *Renderer) PrintWelcomeBox(hasCredentials bool) {
	displayName := "Dhanush"
	if emailUser := auth.GetUsername(); emailUser != "" {
		displayName = emailUser
	} else if u, err := user.Current(); err == nil {
		name := u.Username
		if u.Name != "" {
			name = u.Name
		}
		if name != "" {
			displayName = name
		}
	}
	if len(displayName) > 0 {
		displayName = strings.ToUpper(displayName[:1]) + displayName[1:]
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "~"
	}
	if home, err := os.UserHomeDir(); err == nil {
		if strings.HasPrefix(cwd, home) {
			cwd = "~" + strings.TrimPrefix(cwd, home)
		}
	}

	col1Width := 38
	col2Width := 46

	col1Raw := []string{
		fmt.Sprintf("  Welcome back %s!", displayName),
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"  Tokenly v0.1.0 · Local Proxy",
		"  " + cwd,
	}

	col2Raw := []string{
		"Tips for getting started",
		"Run /init to configure credentials.",
		"Type /help to see all available commands.",
		"",
		"Recent activity",
		"No recent activity",
	}

	col1Formatted := make([]string, len(col1Raw))
	for i, raw := range col1Raw {
		formatted := raw
		rawLen := visualWidth(raw)

		if i == 11 {
			if rawLen > col1Width {
				truncated := "  ..." + cwd[len(cwd)-(col1Width-5):]
				formatted = truncated
				rawLen = visualWidth(truncated)
			}
		}

		if rawLen < col1Width {
			formatted += strings.Repeat(" ", col1Width-rawLen)
		}
		col1Formatted[i] = formatted
	}

	col2Formatted := make([]string, len(col1Raw))
	for i := 0; i < len(col1Raw); i++ {
		var raw string
		if i < len(col2Raw) {
			raw = col2Raw[i]
		}
		rawLen := visualWidth(raw)

		formatted := raw
		if i == 0 || i == 4 {
			formatted = colorBold + colorGreen + raw + colorReset
		} else if i == 1 {
			formatted = strings.ReplaceAll(raw, "/init", colorGreen+"/init"+colorReset)
		} else if i == 2 {
			formatted = strings.ReplaceAll(raw, "/help", colorGreen+"/help"+colorReset)
		}

		if rawLen < col2Width {
			formatted += strings.Repeat(" ", col2Width-rawLen)
		}
		col2Formatted[i] = formatted
	}

	insideWidth := col1Width + 4 + col2Width + 2
	borderColor := colorDim

	fmt.Println()
	topLine := borderColor + "╭─" + colorReset + colorBold + "Tokenly v0.1.0 " + colorReset + borderColor + strings.Repeat("─", insideWidth-16) + "╮" + colorReset
	fmt.Println(topLine)

	for i := 0; i < len(col1Formatted); i++ {
		fmt.Printf("%s│%s %s    %s %s│%s\n", borderColor, colorReset, col1Formatted[i], col2Formatted[i], borderColor, colorReset)
	}

	bottomLine := borderColor + "╰" + strings.Repeat("─", insideWidth) + "╯" + colorReset
	fmt.Println(bottomLine)
}

type Spinner struct {
	stopChan chan struct{}
	wg       sync.WaitGroup
}

func NewSpinner() *Spinner {
	return &Spinner{
		stopChan: make(chan struct{}),
	}
}

func (s *Spinner) Start(msg string) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		dots := []string{".  ", ".. ", "...", "   "}
		i := 0
		for {
			select {
			case <-s.stopChan:
				fmt.Print("\r\033[K")
				return
			default:
				fmt.Printf("\r%s %s%s", symbolInfo, msg, dots[i])
				i = (i + 1) % len(dots)
				time.Sleep(250 * time.Millisecond)
			}
		}
	}()
}

func (s *Spinner) Stop() {
	select {
	case <-s.stopChan:
	default:
		close(s.stopChan)
	}
	s.wg.Wait()
}
