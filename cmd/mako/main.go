package main

import (
	"log"
	"os"

	"github.com/thenameisvicky/tokenly-gateway/internal/shell"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "mako",
		Usage: "Tokenly CLI client",
		Action: func(c *cli.Context) error {
			return shell.Start()
		},
		Commands: []*cli.Command{
			{
				Name:  "start",
				Usage: "Start the mako interactive shell",
				Action: func(c *cli.Context) error {
					return shell.Start()
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
