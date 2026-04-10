package main

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kquo/repokit/internal/bootstrap"
	"github.com/kquo/repokit/internal/color"
	"github.com/kquo/repokit/internal/templates"
)

const programVersion = "0.6.3"

const sourceRepo = "github.com/kquo/repokit"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	subcmd := os.Args[1]
	args := os.Args[2:]

	switch subcmd {
	case "version", "ver":
		fmt.Printf("repokit v%s (template %s)\nsource: %s\n", programVersion, templates.TemplateVersion, sourceRepo)
		return
	case "new", "adopt", "enhance":
		// handled below
	case "-h", "--help", "-?", "help":
		printUsage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", subcmd)
		printUsage()
		os.Exit(2)
	}

	mode := bootstrap.Mode(subcmd)

	cfg, help, err := bootstrap.ParseModeArgs(mode, args)
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

	var tfs fs.FS
	var repoRoot string

	if mode == bootstrap.ModeEnhance {
		root, err := detectRepokitCheckout()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		repoRoot = root

		if cfg.Reference == "" {
			// Self-review: compare on-disk templates against embedded baseline.
			deltas, err := bootstrap.RunSelfReview(templates.EmbeddedFS, templates.DiskFS(root), templates.TemplateVersion)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			bootstrap.PrintSelfReview(deltas, templates.TemplateVersion)
		} else {
			tfs = templates.DiskFS(root)
			if err := bootstrap.RunWithFS(tfs, repoRoot, cfg); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
	} else {
		tfs = templates.EmbeddedFS
		if err := bootstrap.RunWithFS(tfs, repoRoot, cfg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	// Wait for version check (bounded by the 2-second HTTP timeout).
	if notice := <-versionNotice; notice != "" {
		fmt.Fprintln(os.Stderr, notice)
	}
}

func printUsage() {
	fmt.Fprint(os.Stderr, color.FormatUsage("repokit <command> [options]", []color.UsageLine{
		{Flag: "new", Desc: "bootstrap a new governed repo"},
		{Flag: "adopt", Desc: "adopt governance into an existing repo"},
		{Flag: "enhance", Desc: "review a reference repo for template improvements"},
		{Flag: "version, ver", Desc: "print version and source info"},
		{Flag: "help", Desc: "show this help"},
	}, "Run 'repokit <command> --help' for command-specific flags."))
}

// detectRepokitCheckout verifies the working directory is a repokit checkout.
func detectRepokitCheckout() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}

	gomod, err := os.ReadFile(filepath.Join(cwd, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("enhance must be run from inside a repokit checkout (no go.mod found)")
	}
	if !strings.Contains(string(gomod), "module github.com/kquo/repokit") {
		return "", fmt.Errorf("enhance must be run from inside a repokit checkout (go.mod module is not github.com/kquo/repokit)")
	}
	if _, err := os.Stat(filepath.Join(cwd, "internal", "templates", "base")); err != nil {
		return "", fmt.Errorf("enhance must be run from inside a repokit checkout (internal/templates/base not found)")
	}
	return cwd, nil
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
		"https://raw.githubusercontent.com/kquo/repokit/main/TEMPLATE_VERSION", nil)
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
		result <- fmt.Sprintf("%s repokit v%s available (you have v%s) — go install %s/cmd/repokit@latest",
			color.Yel("notice:"), remoteStr, templates.TemplateVersion, sourceRepo)
	}
}
