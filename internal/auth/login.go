package auth

import (
	"fmt"
	"bufio"
	"os"
	"github.com/thenameisvicky/tokenly-cli/internal/keychain"
)

func Init() error {
	fmt.Println("\nEnter same creds as AI agent you use\n")
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Email")
	email, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	fmt.Println("Password")
	password, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	if err := keychain.Save(email, password); err != nil {
		return err
	}

	return nil
}

func HasCredentials() bool {
	return keychain.Exists()
}

func Logout() error {
	return keychain.Delete()
}