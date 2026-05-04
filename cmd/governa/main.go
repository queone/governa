package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/queone/governa-color"
	"github.com/queone/governa/internal/driftscan"
	"github.com/queone/governa/internal/governance"
	"github.com/queone/governa/internal/templates"
)

const programVersion = "0.111.0"

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
	case "apply":
		// handled below
	case "drift-scan":
		exit, err := driftscan.RunCLI(args, templates.EmbeddedFS)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(exit)
	case "sync":
		fmt.Fprintf(os.Stderr, "unknown command: sync (use \"governa apply\")\n")
		os.Exit(2)
	case "new":
		fmt.Fprintf(os.Stderr, "unknown command: new (use \"governa apply\")\n")
		os.Exit(2)
	case "adopt":
		fmt.Fprintf(os.Stderr, "unknown command: adopt (use \"governa apply\")\n")
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

	// Fail-safe: refuse to apply into the governa repo itself.
	if target, _ := filepath.Abs(cfg.Target); target != "" {
		if err := governance.DetectGovernaCheckoutAt(target); err == nil {
			fmt.Fprintln(os.Stderr, "error: cannot run apply against the governa repo itself — apply is for consumer repos")
			os.Exit(1)
		}
	}

	var tfs fs.FS = templates.EmbeddedFS
	if err := governance.RunWithFS(tfs, cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "%s v%s\n", color.BoldW("governa"), programVersion)
	fmt.Fprintln(os.Stderr, color.Gra(fmt.Sprintf("Repo governance templates — %s", sourceRepo)))
	fmt.Fprint(os.Stderr, color.FormatUsage("governa <command> [options]", []color.UsageLine{
		{Flag: "apply", Desc: "apply governance template to a repo"},
		{Flag: "drift-scan", Desc: "scan an adopted repo against governa canon"},
		{Flag: "examples", Desc: "render example repos to /tmp/governa-examples/"},
		{Flag: "version, ver", Desc: "print version and source info"},
		{Flag: "help, h", Desc: "show this help"},
	}, "Run 'governa <command> -h' for command-specific flags."))
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
				Mode:     governance.ModeApply,
				RepoName: "example-code",
				Type:     governance.RepoTypeCode,
				Stack:    "Go",
			},
			module: "github.com/queone/governa/examples/code",
		},
		{
			subdir: "doc",
			cfg: governance.Config{
				Mode:     governance.ModeApply,
				RepoName: "example-doc",
				Type:     governance.RepoTypeDoc,
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
		if err := governance.RunWithFS(tfs, t.cfg); err != nil {
			return fmt.Errorf("render %s overlay: %w", t.subdir, err)
		}
	}

	fmt.Println(examplesOutputDir)
	return nil
}
