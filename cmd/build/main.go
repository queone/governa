// build is based on an original build.sh Bash script from the source project
// that inspired this template.
//
// Thin wrapper. Logic lives in github.com/queone/governa-buildtool.
// governa's local copy registers a PostInstallHook for example-rendering
// (renderAndValidateExamples below); the consumer-facing overlay does NOT
// register a hook. Kept in-tree (not extracted to the library's cmd/) for the
// same reason as cmd/rel: extraction would move version pinning into build.sh
// files. See AC102 reasoning.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/queone/governa-buildtool"
	"github.com/queone/governa-color"
)

func main() {
	cfg, help, err := buildtool.ParseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if help {
		fmt.Print(buildtool.Usage())
		return
	}
	cfg.PostInstallHook = renderAndValidateExamples(cfg)
	if err := buildtool.Run(cfg, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// renderAndValidateExamples renders governa's CODE and DOC overlays to
// /tmp/governa-examples and validates each with go mod tidy + go vet + go test.
// Returns a hook function compatible with buildtool.Config.PostInstallHook so
// example validation runs between build/install and the next-tag suggestion.
// Specific to governa (governa is the template repo); other consumers of
// governa-buildtool leave PostInstallHook nil.
func renderAndValidateExamples(cfg buildtool.Config) func(out, errOut io.Writer) error {
	return func(out, errOut io.Writer) error {
		fmt.Fprintln(out, "\n"+color.Yel("==> Render example repos and validate"))
		exDir := "/tmp/governa-examples"
		if err := runStreaming(out, errOut, "go", "run", "./cmd/governa", "examples"); err != nil {
			return fmt.Errorf("governa examples: %w", err)
		}
		exCodeDir := filepath.Join(exDir, "code")
		for _, sub := range []string{"code", "doc"} {
			subDir := filepath.Join(exDir, sub)
			if _, err := os.Stat(subDir); err != nil {
				return fmt.Errorf("example dir missing: %s", subDir)
			}
			if err := runStreamingInDir(subDir, out, errOut, "go", "mod", "tidy"); err != nil {
				return fmt.Errorf("go mod tidy in %s: %w", subDir, err)
			}
		}
		if output, failed := runCapturedCheckInDir(exCodeDir, "go", "vet", "./..."); failed {
			writeIndented(out, output)
			return fmt.Errorf("go vet failed on rendered code example")
		}
		fmt.Fprintln(out, "    go vet examples/code: ok")
		exTestArgs := []string{"test"}
		if cfg.Verbose {
			exTestArgs = append(exTestArgs, "-v")
		}
		exTestArgs = append(exTestArgs, "./...")
		if err := runStreamingInDir(exCodeDir, out, errOut, "go", exTestArgs...); err != nil {
			return fmt.Errorf("go test failed on rendered code example: %w", err)
		}
		exDocDir := filepath.Join(exDir, "doc")
		if output, failed := runCapturedCheckInDir(exDocDir, "go", "vet", "./..."); failed {
			writeIndented(out, output)
			return fmt.Errorf("go vet failed on rendered doc example")
		}
		fmt.Fprintln(out, "    go vet examples/doc: ok")
		exDocTestArgs := []string{"test"}
		if cfg.Verbose {
			exDocTestArgs = append(exDocTestArgs, "-v")
		}
		exDocTestArgs = append(exDocTestArgs, "./...")
		if err := runStreamingInDir(exDocDir, out, errOut, "go", exDocTestArgs...); err != nil {
			return fmt.Errorf("go test failed on rendered doc example: %w", err)
		}
		os.RemoveAll(exDir)
		fmt.Fprintln(out, "    example validation passed; cleaned up "+exDir)
		return nil
	}
}

func runStreaming(out, errOut io.Writer, name string, args ...string) error {
	command := strings.TrimSpace(name + " " + strings.Join(args, " "))
	fmt.Fprintf(out, "    %s\n", color.Grn(command))
	cmd := exec.Command(name, args...)
	cmd.Stdout = out
	cmd.Stderr = errOut
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func runStreamingInDir(dir string, out, errOut io.Writer, name string, args ...string) error {
	command := strings.TrimSpace(name + " " + strings.Join(args, " "))
	fmt.Fprintf(out, "    %s (in %s)\n", color.Grn(command), dir)
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = errOut
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func runCapturedCheckInDir(dir, name string, args ...string) (string, bool) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	return string(output), err != nil
}

func writeIndented(out io.Writer, s string) {
	for line := range strings.SplitSeq(s, "\n") {
		fmt.Fprintln(out, "    "+line)
	}
}
