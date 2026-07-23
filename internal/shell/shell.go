package shell

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/thenameisvicky/tokenly-cli/internal/auth"
	"golang.org/x/term"
)

const (
	colorReset = "\033[0m"
	colorDim   = "\033[2m"
	colorBold  = "\033[1m"
	colorRed   = "\033[31m"
	colorGreen = "\033[32m"
	colorBlue  = "\033[1;34m"
	colorCyan  = "\033[36m"
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

func NewShell(router *Router) *Shell {
	return &Shell{
		router:   router,
		renderer: &Renderer{},
		prompt:   fmt.Sprintf("%stokenly%s %s›%s ", colorBold, colorReset, colorBlue, colorReset),
	}
}

func Start() error {
	sh := NewShell(InitRouter())
	return sh.Run()
}

func (s *Shell) Run() error {
	s.renderer.PrintWelcomeBox(auth.HasCredentials())

	fd := int(os.Stdin.Fd())
	isTTY := term.IsTerminal(fd)

	var t *term.Terminal
	if isTTY {
		t = term.NewTerminal(stdReadWriter{}, s.prompt)
		t.AutoCompleteCallback = s.makeCompleter()
	}

	for {
		var input string

		if isTTY {
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
		} else {
			scanner := bufio.NewScanner(os.Stdin)
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					return err
				}
				break
			}
			input = scanner.Text()
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
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
					fmt.Printf("\n%sBye!%s\n\n", colorDim, colorReset)
					return nil
				}
			}
		} else {
			if strings.HasPrefix(cmdName, "/") {
				fmt.Printf("\n%s✖%s Unrecognized command: %s. Type %s/help%s for available commands.\n\n", colorRed, colorReset, cmdName, colorBold, colorReset)
			} else {
				fmt.Printf("\nYou typed: %s %s(Type /help to see available commands)%s\n\n", input, colorDim, colorReset)
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

func (r *Renderer) PrintWelcomeBox(hasCredentials bool) {
	fmt.Println()
	fmt.Printf("%s┌────────────────────────────────────────────────────────────┐%s\n", colorBlue, colorReset)

	titleText := "  Tokenly v0.1.0"
	titlePadding := 60 - utf8.RuneCountInString(titleText)
	fmt.Printf("%s│%s%s%s%s%s│%s%s\n",
		colorBlue, colorReset,
		titleText[:2], colorBold, titleText[2:], colorReset+strings.Repeat(" ", titlePadding),
		colorBlue, colorReset)

	fmt.Printf("%s│%s%s│%s%s\n", colorBlue, colorReset, strings.Repeat(" ", 60), colorBlue, colorReset)

	var statusText string
	var symbol string
	var symbolCol string
	if !hasCredentials {
		symbol = "ℹ"
		symbolCol = colorCyan
		statusText = "No credentials configured. Run /init to get started."
	} else {
		symbol = "✔"
		symbolCol = colorGreen
		statusText = "Credentials loaded from system keychain."
	}

	textLen := 2 + 1 + 2 + utf8.RuneCountInString(statusText)
	padding := 60 - textLen
	if padding < 0 {
		padding = 0
	}

	fmt.Printf("%s│%s  %s%s%s  %s%s%s%s│%s\n",
		colorBlue, colorReset,
		symbolCol, symbol, colorReset,
		colorDim, statusText, colorReset+strings.Repeat(" ", padding),
		colorBlue, colorReset,
	)

	fmt.Printf("%s└────────────────────────────────────────────────────────────┘%s\n", colorBlue, colorReset)
	fmt.Println()
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
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for {
			select {
			case <-s.stopChan:
				fmt.Print("\r\033[K")
				return
			default:
				fmt.Printf("\r%s%s%s %s", colorBlue, frames[i], colorReset, msg)
				i = (i + 1) % len(frames)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

func (s *Spinner) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}