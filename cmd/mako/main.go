package main

import (
	"log"
	"github.com/thenameisvicky/tokenly-mako/internal/shell"
)

func main() {
	if err := shell.Start(); err != nil {
		log.Fatal(err)
	}
}