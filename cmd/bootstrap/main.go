package main

import (
	"fmt"
	"os"

	"repokit/internal/bootstrap"
)

func main() {
	cfg, help, err := bootstrap.ParseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if help {
		return
	}
	if err := bootstrap.Run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
