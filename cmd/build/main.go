// build is based on an original build.sh Bash script from the source project
// that inspired this template.
//
// Thin wrapper. Logic lives in github.com/queone/governa-buildtool.
// governa's local copy registers a PostInstallHook for example-rendering
// (renderAndValidateExamples below); the consumer-facing overlay does NOT
// register a hook. Kept in-tree (not extracted to the library's cmd/) for the
// same reason as cmd/rel: extraction would move version pinning into build.sh
// files.
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
//
// Per AC139, governa render-canon is canon-only: this hook seeds go.mod and
// creates the CLAUDE.md → AGENTS.md symlink externally for each flavor.
func renderAndValidateExamples(cfg buildtool.Config) func(out, errOut io.Writer) error {
	return func(out, errOut io.Writer) error {
		fmt.Fprintln(out, "\n"+color.Yel5("==> Render example repos and validate"))
		exDir := "/tmp/governa-examples"
		if err := os.RemoveAll(exDir); err != nil {
			return fmt.Errorf("clean %s: %w", exDir, err)
		}
		exCodeDir := filepath.Join(exDir, "code")
		exDocDir := filepath.Join(exDir, "doc")
		for _, t := range []struct {
			dir, flavor, module string
		}{
			{exCodeDir, "code", "github.com/queone/governa/examples/code"},
			{exDocDir, "doc", "github.com/queone/governa/examples/doc"},
		} {
			// Smoke render: pass --module-path explicitly so {{MODULE_PATH}}
			// substitution uses the example module path, not the governa
			// source's cwd module (which is `github.com/queone/governa`).
			args := []string{"run", "./cmd/governa", "render-canon", "--flavor", t.flavor}
			if t.flavor == "code" {
				args = append(args, "--module-path", t.module)
			}
			args = append(args, t.dir)
			if err := runStreaming(out, errOut, "go", args...); err != nil {
				return fmt.Errorf("governa render-canon --flavor %s %s: %w", t.flavor, t.dir, err)
			}
			// Seed go.mod and CLAUDE.md symlink after render so `go mod tidy`
			// and the smoke runs have what they need; render-canon emits
			// canon files only.
			gomod := fmt.Sprintf("module %s\n\ngo 1.23\n", t.module)
			if err := os.WriteFile(filepath.Join(t.dir, "go.mod"), []byte(gomod), 0o644); err != nil {
				return fmt.Errorf("seed go.mod in %s: %w", t.dir, err)
			}
			if err := os.Symlink("AGENTS.md", filepath.Join(t.dir, "CLAUDE.md")); err != nil && !os.IsExist(err) {
				return fmt.Errorf("symlink CLAUDE.md in %s: %w", t.dir, err)
			}
			if err := runStreamingInDir(t.dir, out, errOut, "go", "mod", "tidy"); err != nil {
				return fmt.Errorf("go mod tidy in %s: %w", t.dir, err)
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

		// smoke test DOC overlay's no-go.mod adoption path.
		// render-canon is canon-only (no go.mod), so this dir exercises the
		// fresh content-repo scenario where the consumer hasn't bootstrapped a
		// Go module. A future regression that re-introduces a module-mode
		// dependency in the DOC rel surface fails here.
		fmt.Fprintln(out, "\n"+color.Yel5("==> Smoke test DOC overlay (no-go.mod adoption)"))
		smokeDir := "/tmp/governa-doc-smoke"
		if err := os.RemoveAll(smokeDir); err != nil {
			return fmt.Errorf("clean %s: %w", smokeDir, err)
		}
		if err := runStreaming(out, errOut, "go", "run", "./cmd/governa", "render-canon", "--flavor", "doc", smokeDir); err != nil {
			return fmt.Errorf("DOC smoke render: %w", err)
		}
		if _, err := os.Stat(filepath.Join(smokeDir, "go.mod")); err == nil {
			return fmt.Errorf("DOC smoke: go.mod was seeded into %s — adoption-time bug check is masked", smokeDir)
		}
		if err := runStreamingInDir(smokeDir, out, errOut, "./rel.sh"); err != nil {
			return fmt.Errorf("DOC smoke: rel.sh failed in no-go.mod dir (this is the no-go.mod-adoption bug — DOC overlay's rel surface requires module mode): %w", err)
		}
		os.RemoveAll(smokeDir)
		fmt.Fprintln(out, "    DOC smoke passed; cleaned up "+smokeDir)
		return nil
	}
}

func runStreaming(out, errOut io.Writer, name string, args ...string) error {
	command := strings.TrimSpace(name + " " + strings.Join(args, " "))
	fmt.Fprintf(out, "    %s\n", color.Grn5(command))
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
	fmt.Fprintf(out, "    %s (in %s)\n", color.Grn5(command), dir)
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
