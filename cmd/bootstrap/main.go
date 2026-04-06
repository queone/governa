package main

import (
	"fmt"
	"os"

	"repo-governance-template/internal/bootstrap"
)

func main() {
	cfg, err := bootstrap.ParseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := bootstrap.Run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
