package main

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/queone/governa-color"
	"github.com/queone/governa/internal/depscheck"
	"github.com/queone/governa/internal/driftscan"
	"github.com/queone/governa/internal/governance"
	"github.com/queone/governa/internal/governarm"
	"github.com/queone/governa/internal/templates"
	"github.com/queone/governa/internal/updatecheck"
)

const programVersion = "0.139.0"

const sourceRepo = "github.com/queone/governa"

func main() {
	os.Exit(run())
}

func run() int {
	defer updatecheck.Check(programVersion)

	if len(os.Args) < 2 {
		printUsage()
		return 2
	}

	subcmd := os.Args[1]
	args := os.Args[2:]

	switch subcmd {
	case "version", "ver":
		fmt.Printf("governa v%s (template %s)\nsource: %s\n", programVersion, templates.TemplateVersion, sourceRepo)
		return 0
	case "render-canon":
		if err := runRenderCanon(args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "apply":
		// handled below
	case "drift-scan":
		exit, err := driftscan.RunCLI(args, templates.EmbeddedFS)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		return exit
	case "rm":
		exit, err := governarm.RunCLI(args, templates.EmbeddedFS, os.Stdout, os.Stderr)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		return exit
	case "deps":
		exit, err := depscheck.RunCLI(args, os.Stdout, os.Stderr)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		return exit
	case "sync":
		fmt.Fprintf(os.Stderr, "unknown command: sync (use \"governa apply\")\n")
		return 2
	case "new":
		fmt.Fprintf(os.Stderr, "unknown command: new (use \"governa apply\")\n")
		return 2
	case "adopt":
		fmt.Fprintf(os.Stderr, "unknown command: adopt (use \"governa apply\")\n")
		return 2
	case "enhance", "ack":
		fmt.Fprintf(os.Stderr, "command removed in v0.50.0: %q (see CHANGELOG)\n", subcmd)
		return 2
	case "-h", "--help", "-?", "help", "h":
		printUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", subcmd)
		printUsage()
		return 2
	}

	mode := governance.Mode(subcmd)

	cfg, help, err := governance.ParseModeArgs(mode, args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if help {
		return 0
	}

	// Fail-safe: refuse to apply into the governa repo itself.
	if target, _ := filepath.Abs(cfg.Target); target != "" {
		if err := governance.DetectGovernaCheckoutAt(target); err == nil {
			fmt.Fprintln(os.Stderr, "error: cannot run apply against the governa repo itself — apply is for consumer repos")
			return 1
		}
	}

	var tfs fs.FS = templates.EmbeddedFS
	if err := governance.RunWithFS(tfs, cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "%s v%s\n", color.Bold(color.Whi5("governa")), programVersion)
	fmt.Fprintln(os.Stderr, color.Gra5(fmt.Sprintf("Repo governance templates — %s", sourceRepo)))
	fmt.Fprint(os.Stderr, color.FormatUsage("governa <command> [options]", []color.UsageLine{
		{Flag: "apply", Desc: "apply governance template to a repo"},
		{Flag: "drift-scan", Desc: "scan an adopted repo against governa canon"},
		{Flag: "rm", Desc: "emit cleanup AC for removing Governa canon"},
		{Flag: "deps", Desc: "report direct dependency freshness for adopted CODE repos or governa source"},
		{Flag: "render-canon", Desc: "render flavor-specific canon files into a target directory"},
		{Flag: "version, ver", Desc: "print version and source info"},
		{Flag: "help, h", Desc: "show this help"},
	}, "Run 'governa <command> -h' for command-specific flags."))
}

// runRenderCanon renders flavor-specific canon files into <target>/, with
// flat repo-relative layout (e.g. <target>/AGENTS.md, <target>/governa/ac-template.md).
// Canon files only — no adoption record, no go.mod seed, no symlinks. Smoke
// harnesses (build.sh) seed go.mod and create symlinks separately as needed.
// Flavor defaults to driftscan.DetectFlavor on cwd; --flavor flag overrides.
// For CODE flavor, {{MODULE_PATH}} substitution reads the module path from
// the cwd's go.mod (the consumer's actual repo), or from the --module-path
// flag when set. For DOC flavor, no Go module path is needed.
func runRenderCanon(args []string) error {
	flavor := ""
	target := ""
	modulePath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help", "-?":
			fmt.Fprint(os.Stderr, color.FormatUsage("governa render-canon [--flavor code|doc] [--module-path <path>] <target>", []color.UsageLine{
				{Flag: "-f, --flavor code|doc", Desc: "select consumer flavor (default: inferred from cwd via driftscan.DetectFlavor)"},
				{Flag: "-m, --module-path <path>", Desc: "module path for CODE-flavor {{MODULE_PATH}} substitution (default: read from cwd's go.mod)"},
			}, "Render canon files into <target>/ in flat repo-relative layout. Canon files only — no adoption record. Target is not pre-cleaned; remove or empty it beforehand if you need a fresh tree."))
			return nil
		case "-f", "--flavor":
			if i+1 >= len(args) {
				return fmt.Errorf("--flavor requires a value")
			}
			flavor = args[i+1]
			i++
		case "-m", "--module-path":
			if i+1 >= len(args) {
				return fmt.Errorf("--module-path requires a value")
			}
			modulePath = args[i+1]
			i++
		default:
			if target != "" {
				return fmt.Errorf("unexpected argument: %s (target already set to %q)", args[i], target)
			}
			target = args[i]
		}
	}
	if target == "" {
		return fmt.Errorf("render-canon requires a positional <target> argument")
	}
	if flavor != "" && flavor != "code" && flavor != "doc" {
		return fmt.Errorf("invalid --flavor: %q (must be 'code' or 'doc')", flavor)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}
	if flavor == "" {
		inferred, err := driftscan.DetectFlavor(cwd)
		if err != nil {
			return fmt.Errorf("infer flavor from cwd: %w (use --flavor to override)", err)
		}
		flavor = inferred
	}
	if flavor == "code" && modulePath == "" {
		modulePath = governance.ReadModulePath(cwd)
		if modulePath == "" {
			return fmt.Errorf("could not read module path from cwd's go.mod (cwd=%s); pass --module-path to override", cwd)
		}
	}

	absTarget, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolve target: %w", err)
	}
	if err := os.MkdirAll(absTarget, 0o755); err != nil {
		return fmt.Errorf("create target %s: %w", absTarget, err)
	}

	// RepoName tracks the consumer's repo identity per governa/development-
	// guidelines.md Template Placeholder Guidance: for CODE flavor, the
	// module path's final component (e.g., module `example.com/consumer` →
	// `consumer`); for DOC flavor, the cwd's basename (the consumer's
	// actual repo dir). Never the scratch target's basename — that would
	// leak the scratch dir name into the rendered AGENTS.md / README.
	repoName := ""
	if modulePath != "" {
		repoName = path.Base(modulePath)
	} else {
		repoName = filepath.Base(cwd)
	}
	cfg := governance.Config{
		Mode:       governance.ModeApply,
		Target:     absTarget,
		RepoName:   repoName,
		ModulePath: modulePath,
	}
	if flavor == "code" {
		cfg.Type = governance.RepoTypeCode
		cfg.Stack = "Go"
	} else {
		cfg.Type = governance.RepoTypeDoc
	}

	canon, err := governance.RenderCanonicalFiles(templates.EmbeddedFS, cfg, absTarget)
	if err != nil {
		return fmt.Errorf("render canon: %w", err)
	}

	for rel, content := range canon {
		dest := filepath.Join(absTarget, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(dest), err)
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(rel, ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(dest, []byte(content), mode); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
	}

	fmt.Println(absTarget)
	return nil
}
