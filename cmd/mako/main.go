package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/thenameisvicky/tokenly-mako/internal/auth"
	"github.com/thenameisvicky/tokenly-mako/internal/clientconfig"
	"github.com/thenameisvicky/tokenly-mako/internal/keychain"
	"github.com/thenameisvicky/tokenly-mako/internal/shell"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
)

func main() {
	proxyFlag := &cli.StringFlag{
		Name:    "proxy",
		Aliases: []string{"g"},
		Usage:   "proxy `URL` to talk to, e.g. https://abc123.ngrok-free.app",
		EnvVars: []string{"MAKO_PROXY_URL"},
	}

	backendFlag := &cli.StringFlag{
		Name:    "credentials-backend",
		Usage:   "where to store credentials: `keyring` or file (default: auto-detect)",
		EnvVars: []string{"MAKO_CREDENTIALS_BACKEND"},
	}

	// Applies the global flags before any command runs, so both `mako` and
	// `mako start` honour them.
	setup := func(c *cli.Context) error {
		if v := c.String("credentials-backend"); v != "" {
			if err := os.Setenv("MAKO_CREDENTIALS_BACKEND", v); err != nil {
				return err
			}
		}
		return clientconfig.SetProxyURL(c.String("proxy"))
	}

	app := &cli.App{
		Name:  "mako",
		Usage: "Mako CLI client",
		Flags: []cli.Flag{proxyFlag, backendFlag},
		Before: func(c *cli.Context) error {
			return setup(c)
		},
		Action: func(c *cli.Context) error {
			return shell.Start()
		},
		Commands: []*cli.Command{
			{
				Name:  "start",
				Usage: "Start the mako interactive shell",
				Flags: []cli.Flag{proxyFlag, backendFlag},
				Action: func(c *cli.Context) error {
					if err := setup(c); err != nil {
						return err
					}
					return shell.Start()
				},
			},
			{
				Name:  "login",
				Usage: "Authenticate without the interactive shell (for scripted setup)",
				Flags: []cli.Flag{
					proxyFlag, backendFlag,
					&cli.StringFlag{Name: "username", Aliases: []string{"u"}, Usage: "account `NAME`", Required: true},
					&cli.BoolFlag{Name: "password-stdin", Usage: "read the password from standard input"},
				},
				Action: func(c *cli.Context) error {
					if err := setup(c); err != nil {
						return err
					}

					password := ""
					if c.Bool("password-stdin") {
						// Read the whole of stdin so a piped heredoc or a file
						// redirect both work, then trim the trailing newline.
						raw, err := io.ReadAll(os.Stdin)
						if err != nil {
							return fmt.Errorf("failed to read the password from stdin: %w", err)
						}
						password = strings.TrimRight(string(raw), "\r\n")
					} else {
						fmt.Print("Password: ")
						raw, err := term.ReadPassword(int(syscall.Stdin))
						fmt.Println()
						if err != nil {
							return fmt.Errorf("failed to read the password: %w (use --password-stdin when there is no terminal)", err)
						}
						password = string(raw)
					}

					if err := auth.Authenticate(c.String("username"), password); err != nil {
						return err
					}
					fmt.Printf("Signed in as %s. Token stored in the %s backend.\n",
						c.String("username"), keychain.CredentialsBackend())
					return nil
				},
			},
			{
				Name:  "env",
				Usage: "Print the resolved proxy URL and credential store, then exit",
				Flags: []cli.Flag{proxyFlag, backendFlag},
				Action: func(c *cli.Context) error {
					if err := setup(c); err != nil {
						return err
					}
					fmt.Printf("proxy                %s\n", clientconfig.ProxyURL())
					fmt.Printf("credentials backend  %s\n", keychain.CredentialsBackend())
					if keychain.UsingFileFallback() {
						path, _ := keychain.CredentialsPath()
						fmt.Printf("credentials file     %s\n", path)
					}
					fmt.Printf("signed in            %t\n", keychain.Exists())
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
