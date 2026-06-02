package main

import (
	"fmt"
	"os"

	"ccoach/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:], cli.Dependencies{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
