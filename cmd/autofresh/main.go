package main

import (
	"fmt"
	"os"

	"autofresh/internal/app"
	"autofresh/internal/cli"
)

func main() {
	service, err := app.NewDefaultService()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := cli.Run(os.Args[1:], cli.Dependencies{
		App:    service,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
