package shell

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"github.com/thenameisvicky/tokenly-cli/internal/auth"
)

func Start() error {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("\nHello from tokenly!\n")

	if !auth.HasCredentials() {
		fmt.Println("Creds not found run /init to get started\n")
	} 

	for {
		fmt.Print("tokenly> ")
		
		if !scanner.Scan() {
			if err:= scanner.Err(); err != nil {
				return err
			}
		}

		input := strings.TrimSpace(scanner.Text())

		switch input {
		case "/exit":
			fmt.Println("\nGoodbye!")
			return nil
		case "/init":
			if err := auth.Init(); err != nil {
				return err
			}
            fmt.Println("\nCredentials saved successfully")
		case "/logout":
			if err := auth.Logout(); err != nil {
				return err
			}
			fmt.Println("Logged out successfully")
		case "":
			continue
		default:
			fmt.Printf("You typed: %s\n", input)
		}
	}
}