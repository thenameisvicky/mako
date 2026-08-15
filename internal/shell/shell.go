package shell

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/thenameisvicky/tokenly-gateway/internal/auth"
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
	colorTan = "\033[38;5;215m"
)

const (
	symbolInfo    = ""
	symbolSuccess = ""
	symbolError   = ""
	symbolBullet  = ""
)

// spinnerFrames mirrors the pulsing star Claude Code shows while it works.
var spinnerFrames = []string{"·", "✢", "✳", "∗", "✽", "✽", "✽", "∗", "✳", "✢"}

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

func termWidth() int {
	width := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		width = w
	}
	return width
}

func printBoxTop() {
	width := termWidth()
	if width < 4 {
		width = 4
	}
	fmt.Printf("%s╭%s╮%s\n", colorDim, strings.Repeat("─", width-2), colorReset)
}

func printBoxBottom() {
	width := termWidth()
	if width < 4 {
		width = 4
	}
	fmt.Printf("%s╰%s╯%s\n", colorDim, strings.Repeat("─", width-2), colorReset)
}

func NewShell(router *Router) *Shell {
	return &Shell{
		router:   router,
		renderer: &Renderer{},
		prompt:   "  > ",
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
		fmt.Printf("\n%sInterrupted%s\n", colorBold, colorReset)
		os.Exit(0)
	}()

	var t *term.Terminal
	if isTTY {
		t = term.NewTerminal(stdReadWriter{}, s.prompt)
		t.AutoCompleteCallback = s.MakeCompleter()
	}

	upperLineNeeded := true
	lastWidth := termWidth()

	for {
		var input string

		if isTTY {
			currentWidth := termWidth()
			if currentWidth != lastWidth {
				fmt.Print("\033[H\033[2J")
				s.renderer.PrintWelcomeBox(auth.HasCredentials())
				upperLineNeeded = true
				lastWidth = currentWidth
			}

			if upperLineNeeded {
				printBoxTop()
				fmt.Println()
				printBoxBottom()
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

			printBoxBottom()
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

		if cmdName == "/" {
			cmdName = "/help"
		}

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
					fmt.Printf("\n  %sSee you next time.%s\n\n", colorDim, colorReset)
					return nil
				}
				fmt.Printf("\n%s%s%s %s%s%s\n\n", colorRed, symbolError, colorReset, colorRed, cmdErr.Error(), colorReset)
			}
		} else {
			if strings.HasPrefix(cmdName, "/") {
				fmt.Printf("\n%s%s%s %sUnknown command %s%s%s. Run %s/help%s to see what's available.%s\n\n", colorRed, symbolError, colorReset, colorDim, colorReset, cmdName, colorDim, colorGreen, colorDim, colorReset)
			} else {
				fmt.Printf("\n  %sTokenly runs on slash commands. Try %s/help%s to get started.%s\n\n", colorDim, colorGreen, colorDim, colorReset)
			}
		}
	}
	return nil
}

func (s *Shell) MakeCompleter() func(line string, pos int, key rune) (newLine string, newPos int, ok bool) {
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

func boxLine(colored, raw string, innerWidth int) string {
	pad := innerWidth - visualWidth(raw)
	if pad < 0 {
		pad = 0
	}
	return fmt.Sprintf("%s│%s%s%s%s│%s", colorDim, colorReset, colored, strings.Repeat(" ", pad), colorDim, colorReset)
}

func (r *Renderer) PrintWelcomeBox(hasCredentials bool) {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "~"
	}
	if home, err := os.UserHomeDir(); err == nil {
		if strings.HasPrefix(cwd, home) {
			cwd = "~" + strings.TrimPrefix(cwd, home)
		}
	}

	innerWidth := 62
	if w := termWidth() - 2; w < innerWidth {
		innerWidth = w
	}
	if innerWidth < 24 {
		innerWidth = 24
	}

	accountRaw := "  Not signed in — run /init to configure credentials"
	accountColored := "  " + colorDim + "Not signed in — run " + colorReset + colorGreen + "/init" + colorReset + colorDim + " to configure credentials" + colorReset
	if hasCredentials {
		if u := auth.GetUsername(); u != "" {
			accountRaw = "  Signed in as " + u
			accountColored = "  " + colorDim + "Signed in as " + colorReset + u
		} else {
			accountRaw = "  Signed in"
			accountColored = "  " + colorDim + "Signed in" + colorReset
		}
	}

	if visualWidth(accountRaw) > innerWidth {
		if hasCredentials {
			accountRaw = "  Signed in"
			accountColored = "  " + colorDim + "Signed in" + colorReset
		} else {
			accountRaw = "  Not signed in — run /init"
			accountColored = "  " + colorDim + "Not signed in — run " + colorReset + colorGreen + "/init" + colorReset
			if visualWidth(accountRaw) > innerWidth {
				accountRaw = "  Not signed in"
				accountColored = "  " + colorDim + "Not signed in" + colorReset
			}
		}
	}
	if visualWidth(accountRaw) > innerWidth {
		keep := innerWidth - 3
		if keep < 0 {
			keep = 0
		}
		accountRaw = accountRaw[:keep]
		accountColored = accountRaw
	}

	cwdRaw := "  " + cwd
	if visualWidth(cwdRaw) > innerWidth {
		keep := innerWidth - 5
		if keep < 1 {
			keep = 1
		}
		if keep > len(cwd) {
			keep = len(cwd)
		}
		cwd = "…" + cwd[len(cwd)-keep:]
		cwdRaw = "  " + cwd
	}
	if visualWidth(cwdRaw) > innerWidth {
		keep := innerWidth - 3
		if keep < 0 {
			keep = 0
		}
		cwdRaw = cwdRaw[:keep]
	}

	title := "  Welcome"
	titleColored := "  " + colorBold + "Welcome" + colorReset

	helpRaw := "  /help for help, /status for your current setup"
	helpColored := "  " + colorGreen + "/help" + colorReset + colorDim + " for help, " + colorReset + colorGreen + "/status" + colorReset + colorDim + " for your current setup" + colorReset

	// Make help line responsive
	if visualWidth(helpRaw) > innerWidth {
		helpRaw = "  /help for help, /status for setup"
		helpColored = "  " + colorGreen + "/help" + colorReset + colorDim + " for help, " + colorReset + colorGreen + "/status" + colorReset + colorDim + " for setup" + colorReset
		if visualWidth(helpRaw) > innerWidth {
			helpRaw = "  /help for help"
			helpColored = "  " + colorGreen + "/help" + colorReset + colorDim + " for help" + colorReset
			if visualWidth(helpRaw) > innerWidth {
				helpRaw = "  /help"
				helpColored = "  " + colorGreen + "/help" + colorReset
			}
		}
	}
	if visualWidth(helpRaw) > innerWidth {
		keep := innerWidth - 3
		if keep < 0 {
			keep = 0
		}
		helpRaw = helpRaw[:keep]
		helpColored = helpRaw
	}

	rows := [][2]string{
		{titleColored, title},
		{"", ""},
		{accountColored, accountRaw},
		{"", ""},
		{helpColored, helpRaw},
		{"", ""},
		{colorDim + cwdRaw + colorReset, cwdRaw},
	}

	fmt.Println()
	fmt.Printf("%s╭%s╮%s\n", colorDim, strings.Repeat("─", innerWidth), colorReset)
	for _, row := range rows {
		fmt.Println(boxLine(row[0], row[1], innerWidth))
	}
	fmt.Printf("%s╰%s╯%s\n", colorDim, strings.Repeat("─", innerWidth), colorReset)
	fmt.Printf("\n  %s? for shortcuts%s\n", colorDim, colorReset)
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
		start := time.Now()
		i := 0
		for {
			select {
			case <-s.stopChan:
				fmt.Print("\r\033[K")
				return
			default:
				elapsed := int(time.Since(start).Seconds())
				frame := spinnerFrames[i%len(spinnerFrames)]
				fmt.Printf("\r\033[K%s%s%s %s%s%s %s(%ds · esc to interrupt)%s",
					colorTan, frame, colorReset,
					colorBold, msg, colorReset,
					colorDim, elapsed, colorReset)
				i++
				time.Sleep(120 * time.Millisecond)
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