// Package depscheck implements the `governa deps` subcommand.
package depscheck

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/queone/governa-color"
	"github.com/queone/governa/internal/emission"
)

const (
	ExitOK       = 0
	ExitEnvError = 1
	ExitUsage    = 2
)

type moduleUpdate struct {
	Path    string `json:"Path"`
	Version string `json:"Version"`
	Update  *struct {
		Version string `json:"Version"`
	} `json:"Update"`
	Indirect bool `json:"Indirect"`
	Main     bool `json:"Main"`
}

var runGoList = defaultGoList

// RunCLI executes the deps subcommand against the current working directory.
func RunCLI(args []string, out, errOut io.Writer) (int, error) {
	if helpRequested(args) {
		printUsage(errOut)
		return ExitOK, nil
	}
	fset := flag.NewFlagSet("governa deps", flag.ContinueOnError)
	fset.SetOutput(errOut)
	if err := fset.Parse(args); err != nil {
		printUsage(errOut)
		return ExitUsage, nil
	}
	if len(fset.Args()) > 0 {
		return ExitUsage, fmt.Errorf("governa deps: no positional arguments accepted; run from the consumer repo root (got: %v)", fset.Args())
	}

	cwd, err := os.Getwd()
	if err != nil {
		return ExitEnvError, fmt.Errorf("governa deps: get cwd: %w", err)
	}
	target, err := filepath.Abs(cwd)
	if err != nil {
		return ExitEnvError, fmt.Errorf("governa deps: resolve cwd: %w", err)
	}
	// `governa deps` is permitted against the governa source repo (so the
	// maintainer can audit governa's own deps); other consumer-run commands
	// still refuse the source via emission.RefuseGovernaSource.
	if !emission.IsGovernaCheckout(target) {
		if err := emission.RequireGovernaAdopted(target, "governa deps"); err != nil {
			return ExitEnvError, err
		}
	}
	if _, err := os.Stat(filepath.Join(target, "go.mod")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(errOut, "governa deps: no go.mod found — deps is CODE-only")
			return ExitOK, nil
		}
		return ExitEnvError, fmt.Errorf("governa deps: stat go.mod: %w", err)
	}

	raw, err := runGoList(target)
	if err != nil {
		return ExitEnvError, fmt.Errorf("governa deps: go list -m -u -json all: %w", err)
	}
	modules, err := parseModules(raw)
	if err != nil {
		return ExitEnvError, fmt.Errorf("governa deps: parse go list output: %w", err)
	}
	renderReport(out, modules)
	return ExitOK, nil
}

func defaultGoList(dir string) ([]byte, error) {
	cmd := exec.Command("go", "list", "-m", "-u", "-json", "all")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func parseModules(raw []byte) ([]moduleUpdate, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	var modules []moduleUpdate
	for {
		var mod moduleUpdate
		if err := dec.Decode(&mod); err != nil {
			if errors.Is(err, io.EOF) {
				return modules, nil
			}
			return nil, err
		}
		if mod.Main || mod.Indirect || strings.TrimSpace(mod.Path) == "" {
			continue
		}
		modules = append(modules, mod)
	}
}

func renderReport(out io.Writer, modules []moduleUpdate) {
	var helper, other []moduleUpdate
	for _, mod := range modules {
		if strings.HasPrefix(mod.Path, "github.com/queone/governa-") {
			helper = append(helper, mod)
		} else {
			other = append(other, mod)
		}
	}
	pathWidth, currentWidth, latestWidth := columnWidths(modules)
	fmt.Fprintln(out, "governa deps")
	if len(helper) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, color.Bold(color.Cya5("governa helper libraries")))
		renderRows(out, helper, pathWidth, currentWidth, latestWidth)
	}
	if len(other) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, color.Bold(color.Whi5("direct dependencies")))
		renderRows(out, other, pathWidth, currentWidth, latestWidth)
	}
	if len(modules) == 0 {
		fmt.Fprintln(out, "no direct dependencies found")
	}
}

func renderRows(out io.Writer, modules []moduleUpdate, pathWidth, currentWidth, latestWidth int) {
	for _, mod := range modules {
		latest := mod.Version
		status := color.Grn5("current")
		if mod.Update != nil && mod.Update.Version != "" && mod.Update.Version != mod.Version {
			latest = mod.Update.Version
			status = color.Yel5("update")
			if majorVersion(latest) > majorVersion(mod.Version) {
				status = color.Red5("major")
			}
			if strings.Contains(latest, "-") {
				status = color.Cya5("pre-release")
			}
		}
		fmt.Fprintf(out, "%-*s  %-*s  %-*s  %s\n", pathWidth, mod.Path, currentWidth, mod.Version, latestWidth, latest, status)
	}
}

func columnWidths(modules []moduleUpdate) (pathWidth, currentWidth, latestWidth int) {
	for _, mod := range modules {
		latest := mod.Version
		if mod.Update != nil && mod.Update.Version != "" && mod.Update.Version != mod.Version {
			latest = mod.Update.Version
		}
		pathWidth = max(pathWidth, len(mod.Path))
		currentWidth = max(currentWidth, len(mod.Version))
		latestWidth = max(latestWidth, len(latest))
	}
	return pathWidth, currentWidth, latestWidth
}

func majorVersion(v string) int {
	v = strings.TrimPrefix(v, "v")
	n := 0
	fmt.Sscanf(v, "%d", &n)
	return n
}

func helpRequested(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" || arg == "-?" {
			return true
		}
	}
	return false
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, color.FormatUsage("governa deps", nil, "Report direct Go dependency freshness for an adopted CODE consumer repo or the governa source repo."))
}
