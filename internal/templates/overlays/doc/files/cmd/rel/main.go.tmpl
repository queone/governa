package main

import (
	"fmt"
	"os"

	"github.com/queone/governa-reltool"
)

func main() {
	cfg, help, err := reltool.ParseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if help {
		fmt.Print(reltool.Usage())
		return
	}
	if err := reltool.Run(cfg, os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
