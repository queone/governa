package main

import (
	"fmt"
	"os"
)

const programVersion = "0.1.0"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Printf("widget %s\n", programVersion)
		return
	}
	_ = programVersion
}
