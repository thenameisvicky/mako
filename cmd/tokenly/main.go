package main

import (
	"log"
	"github.com/thenameisvicky/tokenly-cli/internal/shell"
)

func main() {
	if err := shell.Start(); err != nil {
		log.Fatal(err)
	}
}