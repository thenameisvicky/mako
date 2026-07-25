package auth

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/thenameisvicky/tokenly-cli/internal/keychain"
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

// Init prompts the user for their credentials and saves them to the system keychain.
func Init() error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("\n  %sConfigure Tokenly credentials%s\n\n", colorBold, colorReset)

	fmt.Printf("  %s>%s %sEmail%s    ", colorTan, colorReset, colorBold, colorReset)
	email, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	email = strings.TrimSpace(email)

	fmt.Printf("  %s>%s %sPassword%s ", colorTan, colorReset, colorBold, colorReset)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return err
	}
	fmt.Println()
	password := strings.TrimSpace(string(bytePassword))

	if email == "" || password == "" {
		return fmt.Errorf("email and password cannot be empty")
	}

	if err := keychain.Save(email, password); err != nil {
		return err
	}

	return nil
}

// HasCredentials checks if credentials exist in the system keychain.
func HasCredentials() bool {
	return keychain.Exists()
}

// Logout deletes credentials from the system keychain.
func Logout() error {
	return keychain.Delete()
}

// GetUsername returns the username part of the stored email, if available.
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