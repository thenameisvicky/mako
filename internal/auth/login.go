package auth

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/thenameisvicky/tokenly-mako/internal/clientconfig"
	"github.com/thenameisvicky/tokenly-mako/internal/keychain"
	"github.com/thenameisvicky/tokenly-mako/pkg/models"
	"golang.org/x/term"
)

const (
	colorReset = "\033[0m"
	colorDim   = "\033[38;5;244m"
	colorBold  = "\033[1m"
	colorRed   = "\033[38;5;203m"
	colorGreen = "\033[38;5;84m"
	colorTan   = "\033[38;5;215m"
)

// Login prompts for credentials on the terminal and authenticates.
func Login() error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("\n  %sLog in to the Mako proxy%s\n\n", colorBold, colorReset)

	fmt.Printf("  %s>%s %sUsername%s ", colorTan, colorReset, colorBold, colorReset)
	username, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	username = strings.TrimSpace(username)

	fmt.Printf("  %s>%s %sPassword%s ", colorTan, colorReset, colorBold, colorReset)
	password, err := readPassword(reader)
	if err != nil {
		return err
	}

	return Authenticate(username, password)
}

// readPassword reads a password without echoing it.
//
// Echo suppression needs a terminal. When stdin is a pipe - a script, a CI job,
// a provisioning step - term.ReadPassword fails with "inappropriate ioctl for
// device", so fall back to a plain line read rather than dying. The input is not
// masked in that case, but it was never on screen to begin with.
func readPassword(reader *bufio.Reader) (string, error) {
	if term.IsTerminal(int(syscall.Stdin)) {
		raw, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(raw)), nil
	}

	line, err := reader.ReadString('\n')
	fmt.Println()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// Authenticate exchanges credentials for a Mako token and stores it.
//
// Exported so non-interactive callers (`mako login`) share exactly this path.
func Authenticate(username, password string) error {
	if username == "" || password == "" {
		return fmt.Errorf("username and password cannot be empty")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	payload, err := json.Marshal(models.LoginRequest{
		Username: username,
		Password: password,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal login request: %w", err)
	}

	proxyURL := clientconfig.ProxyURL()
	resp, err := client.Post(proxyURL+"/api/v1/login", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to connect to proxy at %s: %w", proxyURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp models.LoginResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil && errResp.Message != "" {
			return fmt.Errorf("proxy error: %s", errResp.Message)
		}
		return fmt.Errorf("proxy authentication failed (HTTP %d)", resp.StatusCode)
	}

	var loginResp models.LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return fmt.Errorf("failed to decode login response: %w", err)
	}

	if loginResp.Token == "" {
		return fmt.Errorf("proxy did not return an authentication token")
	}

	return keychain.Save(username, loginResp.Token)
}

func HasCredentials() bool {
	return keychain.Exists()
}

func Logout() error {
	return keychain.Delete()
}

func GetUsername() string {
	email, _, err := keychain.Load()
	if err == nil && email != "" {
		parts := strings.Split(email, "@")
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return ""
}
