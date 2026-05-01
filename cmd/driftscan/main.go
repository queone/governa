// Standalone entry point for drift-scan. The same logic is also reachable as
// `governa drift-scan` via cmd/governa/main.go's dispatch switch; both call
// into internal/driftscan.
package main

import (
	"os"

	"github.com/queone/governa/internal/driftscan"
	"github.com/queone/governa/internal/templates"
)

// programVersion is read by buildtool's prep version-bump scan; staticcheck
// would otherwise flag it as unused since the binary delegates everything to
// driftscan.RunCLI.
const programVersion = "0.101.0"

func main() {
	args := os.Args[1:]
	for _, a := range args {
		if a == "-V" || a == "--version" {
			println("governa-driftscan v" + programVersion)
			return
		}
	}
	exit, err := driftscan.RunCLI(args, templates.EmbeddedFS)
	// C3: print Run errors to stderr so the user sees the failure, not just
	// the exit code. RunCLI swallows ParseArgs errors itself; Run errors
	// reach here and need surfacing.
	if err != nil {
		println(err.Error())
	}
	os.Exit(exit)
}
