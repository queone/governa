package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/queone/governa/internal/color"
	"github.com/queone/governa/internal/governance"
	"github.com/queone/governa/internal/templates"
)

const programVersion = "0.58.0"

const sourceRepo = "github.com/queone/governa"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	subcmd := os.Args[1]
	args := os.Args[2:]

	switch subcmd {
	case "version", "ver":
		fmt.Printf("governa v%s (template %s)\nsource: %s\n", programVersion, templates.TemplateVersion, sourceRepo)
		return
	case "examples":
		if len(args) > 0 && (args[0] == "-h" || args[0] == "--help" || args[0] == "-?") {
			fmt.Fprint(os.Stderr, color.FormatUsage("governa examples", nil,
				"Render both CODE and DOC overlays to /tmp/governa-examples/ for inspection or testing."))
			return
		}
		if err := runExamples(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	case "sync":
		// handled below
	case "new":
		fmt.Fprintf(os.Stderr, "unknown command: new (use \"governa sync\")\n")
		os.Exit(2)
	case "adopt":
		fmt.Fprintf(os.Stderr, "unknown command: adopt (use \"governa sync\")\n")
		os.Exit(2)
	case "enhance", "ack":
		fmt.Fprintf(os.Stderr, "command removed in v0.50.0: %q (see CHANGELOG)\n", subcmd)
		os.Exit(2)
	case "-h", "--help", "-?", "help", "h":
		printUsage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", subcmd)
		printUsage()
		os.Exit(2)
	}

	mode := governance.Mode(subcmd)

	cfg, help, err := governance.ParseModeArgs(mode, args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if help {
		return
	}

	// Start version check in background.
	versionNotice := make(chan string, 1)
	go checkLatestVersion(versionNotice)

	// Fail-safe: refuse to sync into the governa repo itself. The check looks at
	// the target path (so syncing from inside the governa repo to an *external*
	// dir via -t is fine — only writing the template onto the template source
	// is the forbidden case).
	if target, _ := filepath.Abs(cfg.Target); target != "" {
		if _, err := detectGovernaCheckoutAt(target); err == nil {
			fmt.Fprintln(os.Stderr, "error: cannot run sync against the governa repo itself — sync is for consumer repos")
			os.Exit(1)
		}
	}

	var tfs fs.FS = templates.EmbeddedFS
	if err := governance.RunWithFS(tfs, "", cfg); err != nil {
		if errors.Is(err, governance.ErrConflictsPresent) {
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Wait for version check (bounded by the 2-second HTTP timeout).
	if notice := <-versionNotice; notice != "" {
		fmt.Fprintln(os.Stderr, notice)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "%s v%s\n", color.BoldW("governa"), programVersion)
	fmt.Fprintln(os.Stderr, color.Gra(fmt.Sprintf("Repo governance templates — %s", sourceRepo)))
	fmt.Fprint(os.Stderr, color.FormatUsage("governa <command> [options]", []color.UsageLine{
		{Flag: "sync", Desc: "bootstrap or update governance in a repo"},
		{Flag: "examples", Desc: "render example repos to /tmp/governa-examples/"},
		{Flag: "version, ver", Desc: "print version and source info"},
		{Flag: "help, h", Desc: "show this help"},
	}, "Run 'governa sync --help' for sync-specific flags."))
}

const examplesOutputDir = "/tmp/governa-examples"

// runExamples renders both CODE and DOC overlays to /tmp/governa-examples/.
func runExamples() error {
	if err := os.RemoveAll(examplesOutputDir); err != nil {
		return fmt.Errorf("clean output dir: %w", err)
	}

	type target struct {
		subdir string
		cfg    governance.Config
		module string
	}
	targets := []target{
		{
			subdir: "code",
			cfg: governance.Config{
				Mode:      governance.ModeSync,
				RepoName:  "example-code",
				Type:      governance.RepoTypeCode,
				Stack:     "Go",
				AssumeYes: true,
			},
			module: "github.com/queone/governa/examples/code",
		},
		{
			subdir: "doc",
			cfg: governance.Config{
				Mode:               governance.ModeSync,
				RepoName:           "example-doc",
				Type:               governance.RepoTypeDoc,
				PublishingPlatform: "Hugo",
				Style:              "house voice",
				AssumeYes:          true,
			},
			module: "github.com/queone/governa/examples/doc",
		},
	}

	var tfs fs.FS = templates.EmbeddedFS
	for _, t := range targets {
		dir := filepath.Join(examplesOutputDir, t.subdir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}

		gomod := fmt.Sprintf("module %s\n\ngo 1.23\n", t.module)
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
			return fmt.Errorf("seed go.mod in %s: %w", dir, err)
		}

		t.cfg.Target = dir
		if err := governance.RunWithFS(tfs, "", t.cfg); err != nil {
			return fmt.Errorf("render %s overlay: %w", t.subdir, err)
		}
	}

	fmt.Println(examplesOutputDir)
	return nil
}

// detectGovernaCheckoutAt reports whether `dir` looks like a governa checkout.
// Used by the sync fail-safe to refuse writing the template over its own
// source.
func detectGovernaCheckoutAt(dir string) (string, error) {
	gomod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("no go.mod found")
	}
	if !strings.Contains(string(gomod), "module github.com/queone/governa") {
		return "", fmt.Errorf("go.mod module is not github.com/queone/governa")
	}
	if _, err := os.Stat(filepath.Join(dir, "internal", "templates", "base")); err != nil {
		return "", fmt.Errorf("internal/templates/base not found")
	}
	return dir, nil
}

var semverRe = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)`)

type semver struct {
	major, minor, patch int
}

func parseSemver(s string) (semver, bool) {
	m := semverRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return semver{}, false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	return semver{major, minor, patch}, true
}

func (a semver) newerThan(b semver) bool {
	if a.major != b.major {
		return a.major > b.major
	}
	if a.minor != b.minor {
		return a.minor > b.minor
	}
	return a.patch > b.patch
}

func checkLatestVersion(result chan<- string) {
	defer close(result)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://raw.githubusercontent.com/queone/governa/main/TEMPLATE_VERSION", nil)
	if err != nil {
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}

	buf := make([]byte, 64)
	n, _ := resp.Body.Read(buf)
	remote, ok := parseSemver(string(buf[:n]))
	if !ok {
		return
	}
	local, ok := parseSemver(templates.TemplateVersion)
	if !ok {
		return
	}

	if remote.newerThan(local) {
		remoteStr := fmt.Sprintf("%d.%d.%d", remote.major, remote.minor, remote.patch)
		result <- fmt.Sprintf("%s governa v%s available (you have v%s) — go install %s/cmd/governa@latest",
			color.Yel("notice:"), remoteStr, templates.TemplateVersion, sourceRepo)
	}
}
