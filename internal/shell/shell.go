package shell

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func Start() error {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("\nHello from tokenly!\n")
	fmt.Println("Type /exit to quit \n")

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
		case "":
			continue
		default:
			fmt.Printf("You typed: %s\n", input)
		}
	}
}