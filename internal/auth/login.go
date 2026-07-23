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
	colorDim   = "\033[2m"
	colorBold  = "\033[1m"
	colorRed   = "\033[31m"
	colorGreen = "\033[32m"
)

// Init prompts the user for their credentials and saves them to the system keychain.
func Init() error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("%s?%s %sEmail:%s %s›%s ", colorGreen, colorReset, colorBold, colorReset, colorDim, colorReset)
	email, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	email = strings.TrimSpace(email)

	fmt.Printf("%s?%s %sPassword:%s %s›%s ", colorGreen, colorReset, colorBold, colorReset, colorDim, colorReset)
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