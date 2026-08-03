package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func governaBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "governa")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = filepath.Join(testRepoRoot(t), "cmd", "governa")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build governa binary: %v\n%s", err, out)
	}
	return bin
}

func governaCmd(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(governaBinary(t), args...)
	cmd.Env = append(os.Environ(), "GOVERNA_NO_UPDATE_CHECK=1")
	return cmd
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	// cmd/governa is two levels below the repo root.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(wd, "..", "..")
}

func TestCLIHelpAlias(t *testing.T) {
	t.Parallel()
	for _, arg := range []string{"help", "h", "-h", "--help", "-?"} {
		out, _ := governaCmd(t, arg).CombinedOutput()
		output := string(out)
		if !strings.Contains(output, "governa v") {
			t.Errorf("governa %s: output should contain version header, got:\n%s", arg, output)
		}
		if !strings.Contains(output, "help, h") {
			t.Errorf("governa %s: output should list 'help, h', got:\n%s", arg, output)
		}
		if !strings.Contains(output, "Repo governance templates") {
			t.Errorf("governa %s: output should contain description, got:\n%s", arg, output)
		}
	}
}

// AT for cmd/governa/main_test.go subcommand registration coverage:
// drift-scan appears in printUsage() output.
func TestDriftScanSubcommandListed(t *testing.T) {
	t.Parallel()
	out, _ := governaCmd(t, "help").CombinedOutput()
	if !strings.Contains(string(out), "drift-scan") {
		t.Errorf("expected 'drift-scan' in help output, got:\n%s", out)
	}
}

// drift-scan dispatches to the drift-scan handler (not "unknown command").
// Note: dispatch with no args reaches the drift-scan handler, then fails the
// governa-adoption check (the binary's own cwd at test time is the governa
// source tree, but the cwd of the spawned process is the test's TempDir or
// the test working dir; either way, the failure is from drift-scan, not from
// the top-level unknown-command path).
func TestDriftScanDispatch(t *testing.T) {
	t.Parallel()
	out, _ := governaCmd(t, "drift-scan").CombinedOutput()
	if strings.Contains(string(out), "unknown command") {
		t.Errorf("drift-scan should not be unknown, got:\n%s", out)
	}
}

// `governa drift-scan -h` prints drift-scan-specific help.
func TestDriftScanHelp(t *testing.T) {
	t.Parallel()
	out, _ := governaCmd(t, "drift-scan", "-h").CombinedOutput()
	if !strings.Contains(string(out), "Scan an adopted-governa repo") {
		t.Errorf("drift-scan help should describe the command, got:\n%s", out)
	}
	for _, want := range []string{
		"-f, --flavor code|doc",
		"-s, --stack <name>",
		"-j, --json",
		"-l, --diff-lines <N>",
		"-n, --repo-name <name>",
		"CODE stack",
		"inferred from manifests",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("drift-scan help missing %q:\n%s", want, out)
		}
	}
}

func TestDriftScanSelectsRustBeforeManifest(t *testing.T) {
	t.Parallel()
	bin := governaBinary(t)
	dir := t.TempDir()
	files := map[string]string{
		"AGENTS.md":              "# AGENTS.md\n",
		"CHANGELOG.md":           "| Version | Summary |\n|---|---|\n| Unreleased | |\n",
		"governa/ac-template.md": "# AC template\n",
	}
	for rel, content := range files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rustBuild, err := os.ReadFile(
		filepath.Join(
			"..",
			"..",
			"internal",
			"templates",
			"overlays",
			"code",
			"stacks",
			"rust",
			"build.sh.tmpl",
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "build.sh"), rustBuild, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"git", "init", "-q"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "add", "-A"},
		{"git", "commit", "-qm", "initial"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	cmd := exec.Command(
		bin,
		"drift-scan",
		"--flavor",
		"code",
		"--stack",
		"Rust",
	)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOVERNA_NO_UPDATE_CHECK=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pre-manifest Rust drift-scan: %v\n%s", err, out)
	}
	matches, err := filepath.Glob(filepath.Join(dir, "governa", "ac*-drift-scan-v*.md"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("emitted ACs: %v err=%v output=%s", matches, err, out)
	}
	stub, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stub), "`build.sh` —") {
		t.Fatalf("canonical Rust build.sh was reported as drift:\n%s", stub)
	}
}

// AT13: drift-scan rejects positional arguments — no <repo-path> accepted.
func TestDriftScanRejectsPositionalArg(t *testing.T) {
	t.Parallel()
	out, _ := governaCmd(t, "drift-scan", "/some/path").CombinedOutput()
	if !strings.Contains(string(out), "no positional arguments accepted") {
		t.Errorf("expected positional-arg rejection, got:\n%s", out)
	}
}

func TestRMAndDepsListed(t *testing.T) {
	t.Parallel()
	out, _ := governaCmd(t, "help").CombinedOutput()
	for _, want := range []string{"rm", "deps"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("help missing %q:\n%s", want, out)
		}
	}
}

func TestRMAndDepsHelpFlags(t *testing.T) {
	t.Parallel()
	for _, subcmd := range []string{"rm", "deps"} {
		for _, flag := range []string{"-h", "--help", "-?"} {
			out, err := governaCmd(t, subcmd, flag).CombinedOutput()
			if err != nil {
				t.Fatalf("governa %s %s failed: %v\n%s", subcmd, flag, err, out)
			}
			if !strings.Contains(string(out), "Usage:") {
				t.Fatalf("governa %s %s missing Usage:\n%s", subcmd, flag, out)
			}
		}
	}
}

func TestRemoveAliasRejected(t *testing.T) {
	t.Parallel()
	out, err := governaCmd(t, "remove").CombinedOutput()
	if err == nil || !strings.Contains(string(out), "unknown command: remove") {
		t.Fatalf("expected remove alias rejection, err=%v out=%s", err, out)
	}
}

func TestUpdateCheckRunsOnNonZeroReturn(t *testing.T) {
	bin := governaBinary(t)
	cacheRoot := t.TempDir()
	cachePath := filepath.Join(cacheRoot, "governa", "last-check")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{
		"checked_at":     time.Now().UTC(),
		"latest_version": "v9.9.9",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, body, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "drift-scan")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "XDG_CACHE_HOME="+cacheRoot)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero drift-scan exit, output:\n%s", out)
	}
	if !strings.Contains(string(out), "governa v9.9.9 available") {
		t.Fatalf("expected deferred update notice on non-zero path, got:\n%s", out)
	}
}

func TestRenderCanonInfersRustAndAcceptsStackOverride(t *testing.T) {
	t.Parallel()
	bin := governaBinary(t)

	rustDir := t.TempDir()
	cargo := "[package]\nname = \"example\"\nversion = \"0.1.0\"\n"
	if err := os.WriteFile(filepath.Join(rustDir, "Cargo.toml"), []byte(cargo), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "inferred")
	cmd := exec.Command(bin, "render-canon", "--flavor", "code", target)
	cmd.Dir = rustDir
	cmd.Env = append(os.Environ(), "GOVERNA_NO_UPDATE_CHECK=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("inferred Rust render failed: %v\n%s", err, out)
	}
	build, err := os.ReadFile(filepath.Join(target, "build.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(build), "cargo clippy") {
		t.Fatal("inferred Rust canon did not emit Rust build.sh")
	}
	metadata, err := os.ReadFile(filepath.Join(target, "governa", "metadata.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(metadata); !strings.Contains(got, "repo_type = CODE\n") ||
		!strings.Contains(got, "code_stack = Rust\n") ||
		!strings.Contains(got, "governa_version = v") {
		t.Fatalf("rendered CODE metadata is incomplete: %q", got)
	}
	if _, err := os.Stat(filepath.Join(target, "governa", "repo-type.txt")); !os.IsNotExist(err) {
		t.Fatalf("rendered CODE canon retained retired marker: %v", err)
	}
	for _, want := range []string{
		"Usage: build [target ...] [-v|--verbose]",
		"_build_scoped_phases",
		"--no-track",
	} {
		if !strings.Contains(string(build), want) {
			t.Errorf("inferred Rust canon missing scoped-build marker %q", want)
		}
	}
	buildCLI, err := os.ReadFile(filepath.Join(target, "tests", "build_cli.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"test_compiled_version_output", "test_prep_no_build_rejection"} {
		if !strings.Contains(string(buildCLI), want) {
			t.Errorf("inferred Rust canon build CLI suite missing %q", want)
		}
	}

	goDir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(goDir, "go.mod"),
		[]byte("module example.com/go-consumer\n\ngo 1.25\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"render-canon", "--flavor", "code", "-s", "Rust", filepath.Join(t.TempDir(), "short")},
		{"render-canon", "--flavor", "code", "--stack", "Rust", filepath.Join(t.TempDir(), "long")},
	} {
		cmd = exec.Command(bin, args...)
		cmd.Dir = goDir
		cmd.Env = append(os.Environ(), "GOVERNA_NO_UPDATE_CHECK=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Rust override failed: %v\n%s", err, out)
		}
		rendered, err := os.ReadFile(filepath.Join(args[len(args)-1], "build.sh"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(rendered), "cargo test") {
			t.Fatal("explicit Rust override did not win over go.mod")
		}
		if !strings.Contains(string(rendered), "_build_scoped_phases") {
			t.Fatal("explicit Rust override omitted scoped-build routing")
		}
		if _, err := os.Stat(filepath.Join(args[len(args)-1], "tests", "build_cli.sh")); err != nil {
			t.Fatalf("explicit Rust override omitted build CLI suite: %v", err)
		}
	}
}

func TestRenderCanonInfersSwiftAndAcceptsCaseInsensitiveOverride(t *testing.T) {
	t.Parallel()
	bin := governaBinary(t)
	swiftDir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(swiftDir, "Package.swift"),
		[]byte("// swift-tools-version: 6.0\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"render-canon", "--flavor", "code", filepath.Join(t.TempDir(), "inferred")},
		{"render-canon", "--flavor", "code", "--stack", "sWiFt", filepath.Join(t.TempDir(), "explicit")},
	} {
		cmd := exec.Command(bin, args...)
		cmd.Dir = swiftDir
		cmd.Env = append(os.Environ(), "GOVERNA_NO_UPDATE_CHECK=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Swift render failed: %v\n%s", err, out)
		}
		build, err := os.ReadFile(filepath.Join(args[len(args)-1], "build.sh"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(build), "swift format lint --strict") {
			t.Fatal("Swift canon did not emit Swift build.sh")
		}
	}
}

func TestRenderCanonStackHelpAndFlavorValidation(t *testing.T) {
	t.Parallel()
	bin := governaBinary(t)
	cmd := exec.Command(bin, "render-canon", "--help")
	cmd.Env = append(os.Environ(), "GOVERNA_NO_UPDATE_CHECK=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("help failed: %v\n%s", err, out)
	}
	for _, want := range []string{"-s, --stack <name>", "-m, --module-path <path>"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("help missing %q:\n%s", want, out)
		}
	}

	docDir := t.TempDir()
	cmd = exec.Command(
		bin,
		"render-canon",
		"--flavor",
		"doc",
		"--stack",
		"Rust",
		filepath.Join(t.TempDir(), "doc"),
	)
	cmd.Dir = docDir
	cmd.Env = append(os.Environ(), "GOVERNA_NO_UPDATE_CHECK=1")
	out, err = cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "--stack applies only to CODE canon") {
		t.Fatalf("DOC stack rejection: err=%v out=%s", err, out)
	}
	docTarget := filepath.Join(t.TempDir(), "doc-valid")
	cmd = exec.Command(bin, "render-canon", "--flavor", "doc", docTarget)
	cmd.Dir = docDir
	cmd.Env = append(os.Environ(), "GOVERNA_NO_UPDATE_CHECK=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("DOC render failed: %v\n%s", err, out)
	}
	docMetadata, err := os.ReadFile(filepath.Join(docTarget, "governa", "metadata.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(docMetadata); !strings.Contains(got, "repo_type = DOC\n") ||
		strings.Contains(got, "code_stack") || !strings.Contains(got, "governa_version = v") {
		t.Fatalf("rendered DOC metadata is incomplete: %q", got)
	}
	if _, err := os.Stat(filepath.Join(docTarget, "governa", "repo-type.txt")); !os.IsNotExist(err) {
		t.Fatalf("rendered DOC canon retained retired marker: %v", err)
	}

	for name, args := range map[string][]string{
		"Rust CODE": {
			"render-canon",
			"--flavor",
			"code",
			"--stack",
			"Rust",
			"--module-path",
			"example.com/wrong",
			filepath.Join(t.TempDir(), "rust"),
		},
		"DOC": {
			"render-canon",
			"--flavor",
			"doc",
			"--module-path",
			"example.com/wrong",
			filepath.Join(t.TempDir(), "doc-module"),
		},
	} {
		cmd = exec.Command(bin, args...)
		cmd.Dir = docDir
		cmd.Env = append(os.Environ(), "GOVERNA_NO_UPDATE_CHECK=1")
		out, err = cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "--module-path applies only to Go CODE canon") {
			t.Errorf("%s module-path rejection: err=%v out=%s", name, err, out)
		}
	}
}
