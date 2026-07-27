package governance

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/queone/governa/internal/templates"
)

// Helper: build a fixture target directory with the listed relative-path
// files and contents. Returns the absolute target path.
func newFixtureTarget(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		abs := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return dir
}

func TestParseFlagsApplyDefaults(t *testing.T) {
	t.Parallel()
	cfg, help, err := parseFlags(ModeApply, []string{"--target", "/tmp/nope"})
	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	if help {
		t.Fatal("unexpected help request")
	}
	if cfg.Mode != ModeApply {
		t.Errorf("Mode = %q; want %q", cfg.Mode, ModeApply)
	}
	if cfg.Target != "/tmp/nope" {
		t.Errorf("Target = %q; want /tmp/nope", cfg.Target)
	}
}

// AT8: `--no` flag is no longer recognized.
func TestParseFlagsRejectsNo(t *testing.T) {
	t.Parallel()
	_, _, err := parseFlags(ModeApply, []string{"--no", "--target", "/tmp/x"})
	if err == nil {
		t.Fatal("expected flag-parse error for removed --no flag; got nil")
	}
}

// AT9: `--dry-run` flag is no longer recognized.
func TestParseFlagsRejectsDryRun(t *testing.T) {
	t.Parallel()
	_, _, err := parseFlags(ModeApply, []string{"--dry-run", "--target", "/tmp/x"})
	if err == nil {
		t.Fatal("expected flag-parse error for removed --dry-run flag; got nil")
	}
	_, _, err = parseFlags(ModeApply, []string{"-d", "--target", "/tmp/x"})
	if err == nil {
		t.Fatal("expected flag-parse error for removed -d shorthand; got nil")
	}
}

// --yes flag is removed (no collision negotiation).
func TestParseFlagsRejectsYes(t *testing.T) {
	t.Parallel()
	_, _, err := parseFlags(ModeApply, []string{"--yes", "--target", "/tmp/x"})
	if err == nil {
		t.Fatal("expected flag-parse error for removed --yes flag; got nil")
	}
	_, _, err = parseFlags(ModeApply, []string{"-y", "--target", "/tmp/x"})
	if err == nil {
		t.Fatal("expected flag-parse error for removed -y shorthand; got nil")
	}
}

// help text describes consumer ownership, not collision/review.
func TestModeHelpApplyDescribesConsumerOwnership(t *testing.T) {
	t.Parallel()
	help := ModeHelp(ModeApply)
	if help == "" {
		t.Fatal("ModeHelp returned empty")
	}
	if !strings.Contains(help, "consumer-owned") {
		t.Errorf("apply help missing consumer-owned reference: %q", help)
	}
}

// --yes must NOT appear in help (removed).
func TestModeHelpApplyOmitsYesFlag(t *testing.T) {
	t.Parallel()
	help := ModeHelp(ModeApply)
	if strings.Contains(help, "--yes") {
		t.Errorf("apply help still references --yes; should be removed. Got:\n%s", help)
	}
	if strings.Contains(help, "-y,") {
		t.Errorf("apply help still references -y shorthand; should be removed. Got:\n%s", help)
	}
}

// Historical: --dry-run must NOT appear as a flag-list row (it was retired).
func TestModeHelpApplyOmitsDryRun(t *testing.T) {
	t.Parallel()
	help := ModeHelp(ModeApply)
	if strings.Contains(help, "--dry-run") {
		t.Errorf("apply help still references --dry-run; should be removed. Got:\n%s", help)
	}
}

func TestModeHelpRemovedModes(t *testing.T) {
	t.Parallel()
	if got := ModeHelp(Mode("enhance")); got != "" {
		t.Errorf("removed mode 'enhance' should have empty help; got %q", got)
	}
	if got := ModeHelp(Mode("ack")); got != "" {
		t.Errorf("removed mode 'ack' should have empty help; got %q", got)
	}
}

func TestRunWithFSRejectsUnsupportedMode(t *testing.T) {
	t.Parallel()
	err := RunWithFS(templates.EmbeddedFS, Config{Mode: Mode("enhance")})
	if err == nil || !strings.Contains(err.Error(), "unsupported mode") {
		t.Fatalf("expected unsupported-mode error; got %v", err)
	}
}

func TestInferStackFromGoMod(t *testing.T) {
	t.Parallel()
	dir := newFixtureTarget(t, map[string]string{
		"go.mod": "module x\n\ngo 1.25\n",
	})
	if got := inferStack(dir); got != "Go" {
		t.Errorf("inferStack = %q; want Go", got)
	}
}

func TestInferStackFromCargoManifest(t *testing.T) {
	t.Parallel()
	dir := newFixtureTarget(t, map[string]string{
		"Cargo.toml": "[package]\nname = \"example\"\nversion = \"0.1.0\"\n",
	})
	if got := inferStack(dir); got != "Rust" {
		t.Errorf("inferStack = %q; want Rust", got)
	}
}

func TestInferStackManifestPrecedenceRemainsStable(t *testing.T) {
	t.Parallel()
	dir := newFixtureTarget(t, map[string]string{
		"go.mod":     "module example.com/test\n\ngo 1.25\n",
		"Cargo.toml": "[package]\nname = \"example\"\nversion = \"0.1.0\"\n",
	})
	if got := inferStack(dir); got != "Go" {
		t.Errorf("inferStack = %q; want Go precedence", got)
	}
}

func TestInferStackFromTerraformLockFile(t *testing.T) {
	t.Parallel()
	dir := newFixtureTarget(t, map[string]string{
		".terraform.lock.hcl": "# This file is maintained automatically\n",
	})
	if got := inferStack(dir); got != "Terraform" {
		t.Errorf("inferStack = %q; want Terraform", got)
	}
}

func TestInferStackFromDotTfGlob(t *testing.T) {
	t.Parallel()
	dir := newFixtureTarget(t, map[string]string{
		"main.tf": "terraform {}\n",
	})
	if got := inferStack(dir); got != "Terraform" {
		t.Errorf("inferStack = %q; want Terraform", got)
	}
}

func TestInferStackEmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if got := inferStack(dir); got != "" {
		t.Errorf("inferStack on empty dir = %q; want empty string", got)
	}
}

// Go stack emits a build.sh rendered from the Go-stack template.
func TestGoStackEmitsBuildSh(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := Config{
		Mode:     ModeApply,
		Target:   dir,
		Type:     RepoTypeCode,
		RepoName: "test-repo",
		Stack:    "Go",
	}
	if err := RunWithFS(templates.EmbeddedFS, cfg); err != nil {
		t.Fatalf("RunWithFS: %v", err)
	}
	buildSh, err := os.ReadFile(filepath.Join(dir, "build.sh"))
	if err != nil {
		t.Fatalf("build.sh not emitted: %v", err)
	}
	if !strings.Contains(string(buildSh), "go mod tidy") {
		t.Error("Go stack build.sh should contain 'go mod tidy'")
	}
}

// Terraform stack emits a build.sh rendered from the Terraform-stack template.
func TestTerraformStackEmitsBuildSh(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := Config{
		Mode:     ModeApply,
		Target:   dir,
		Type:     RepoTypeCode,
		RepoName: "test-repo",
		Stack:    "Terraform",
	}
	if err := RunWithFS(templates.EmbeddedFS, cfg); err != nil {
		t.Fatalf("RunWithFS: %v", err)
	}
	buildSh, err := os.ReadFile(filepath.Join(dir, "build.sh"))
	if err != nil {
		t.Fatalf("build.sh not emitted: %v", err)
	}
	content := string(buildSh)
	if strings.Contains(content, "go mod tidy") {
		t.Error("Terraform stack build.sh must not contain 'go mod tidy'")
	}
	if !strings.Contains(content, "terraform fmt") {
		t.Error("Terraform stack build.sh should contain 'terraform fmt'")
	}
	if !strings.Contains(content, "terraform validate") {
		t.Error("Terraform stack build.sh should contain 'terraform validate'")
	}
}

// Terraform stack .gitignore includes Terraform-specific patterns.
func TestTerraformStackGitignoreBlock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := Config{
		Mode:     ModeApply,
		Target:   dir,
		Type:     RepoTypeCode,
		RepoName: "test-repo",
		Stack:    "Terraform",
	}
	if err := RunWithFS(templates.EmbeddedFS, cfg); err != nil {
		t.Fatalf("RunWithFS: %v", err)
	}
	gitignore, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf(".gitignore not emitted: %v", err)
	}
	content := string(gitignore)
	for _, want := range []string{".terraform/", "*.tfstate", "*.tfvars"} {
		if !strings.Contains(content, want) {
			t.Errorf(".gitignore missing %q for Terraform stack", want)
		}
	}
}

func TestRustStackEmitsBuildIgnoreAndGuidance(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := Config{
		Mode:     ModeApply,
		Target:   dir,
		Type:     RepoTypeCode,
		RepoName: "test-repo",
		Stack:    "Rust",
	}
	if err := RunWithFS(templates.EmbeddedFS, cfg); err != nil {
		t.Fatalf("RunWithFS: %v", err)
	}
	buildPath := filepath.Join(dir, "build.sh")
	info, err := os.Stat(buildPath)
	if err != nil {
		t.Fatalf("build.sh not emitted: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("build.sh mode %v is not executable", info.Mode())
	}
	build, err := os.ReadFile(buildPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(build)
	required := []string{
		"cargo fmt --check",
		"cargo clippy",
		"--all-targets --all-features -- -D warnings",
		"cargo test",
		"cargo build",
		"--release",
	}
	last := -1
	for _, want := range required {
		index := strings.Index(content, want)
		if index < 0 {
			t.Errorf("build.sh missing %q", want)
		}
		if index < last {
			t.Errorf("build.sh command marker %q appears out of order", want)
		}
		last = index
	}
	for _, unwanted := range []string{"go mod tidy", "terraform validate"} {
		if strings.Contains(content, unwanted) {
			t.Errorf("Rust build.sh contains %q", unwanted)
		}
	}

	gitignore, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	ignoreContent := string(gitignore)
	for _, want := range []string{"/target/", "**/*.rs.bk"} {
		if !strings.Contains(ignoreContent, want) {
			t.Errorf(".gitignore missing %q", want)
		}
	}
	for line := range strings.SplitSeq(ignoreContent, "\n") {
		if strings.TrimSpace(line) == "Cargo.lock" ||
			strings.TrimSpace(line) == "/Cargo.lock" {
			t.Error(".gitignore must not ignore Cargo.lock")
		}
	}

	guidelines, err := os.ReadFile(filepath.Join(dir, "governa", "development-guidelines.md"))
	if err != nil {
		t.Fatal(err)
	}
	guidelineContent := string(guidelines)
	boundaryMarker := "\n## Project Practices\n"
	boundary := strings.Index(guidelineContent, boundaryMarker)
	rustBlock := strings.Index(guidelineContent, "\n## Rust Practices\n")
	if rustBlock < 0 || boundary < 0 || rustBlock >= boundary {
		t.Fatalf("Rust guidance must precede Project Practices")
	}
	if strings.Count(guidelineContent, boundaryMarker) != 1 {
		t.Fatalf("Project Practices heading count = %d; want 1",
			strings.Count(guidelineContent, boundaryMarker))
	}
	if !strings.Contains(
		guidelineContent,
		"Sections above ## Project Practices are governa-maintained canon",
	) {
		t.Fatal("stack composition broke the Project Practices introduction")
	}
	rustFragment, err := templates.EmbeddedFS.ReadFile("stack-guidelines/rust.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(
		guidelineContent[rustBlock:boundary],
		strings.TrimSpace(string(rustFragment)),
	) {
		t.Error("rendered Rust guidance does not contain the complete embedded fragment")
	}
	if strings.Contains(guidelineContent, "## Go Practices") ||
		strings.Contains(guidelineContent, "programVersion") {
		t.Error("Rust guidelines contain Go-only guidance")
	}
}

func TestGoGuidanceMovesOutOfGenericCanon(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := Config{
		Mode:     ModeApply,
		Target:   dir,
		Type:     RepoTypeCode,
		RepoName: "test-repo",
		Stack:    "Go",
	}
	if err := RunWithFS(templates.EmbeddedFS, cfg); err != nil {
		t.Fatal(err)
	}
	contentBytes, err := os.ReadFile(filepath.Join(dir, "governa", "development-guidelines.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(contentBytes)
	block := strings.Index(content, "\n## Go Practices\n")
	boundary := strings.Index(content, "\n## Project Practices\n")
	if block < 0 || boundary < 0 || block >= boundary {
		t.Fatal("Go guidance must precede Project Practices")
	}
	goFragment, err := templates.EmbeddedFS.ReadFile("stack-guidelines/go.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content[block:boundary], strings.TrimSpace(string(goFragment))) {
		t.Error("rendered Go guidance does not contain the complete embedded fragment")
	}
}

func TestUnsupportedStackKeepsGenericCanon(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := Config{
		Mode:     ModeApply,
		Target:   dir,
		Type:     RepoTypeCode,
		RepoName: "test-repo",
		Stack:    "Unknown",
	}
	if err := RunWithFS(templates.EmbeddedFS, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "build.sh")); !os.IsNotExist(err) {
		t.Fatalf("unsupported stack build.sh stat error = %v; want not-exist", err)
	}
	contentBytes, err := os.ReadFile(filepath.Join(dir, "governa", "development-guidelines.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(contentBytes)
	if strings.Contains(content, "## Go Practices") ||
		strings.Contains(content, "## Rust Practices") {
		t.Error("unsupported stack received stack-specific guidance")
	}
}

func renderRustRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cfg := Config{
		Mode:     ModeApply,
		Target:   dir,
		Type:     RepoTypeCode,
		RepoName: "test-repo",
		Stack:    "Rust",
	}
	if err := RunWithFS(templates.EmbeddedFS, cfg); err != nil {
		t.Fatalf("RunWithFS: %v", err)
	}
	return dir
}

func writeFakeCargo(t *testing.T, dir string) (string, string) {
	t.Helper()
	binDir := filepath.Join(dir, "fake-bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "cargo.log")
	script := `#!/usr/bin/env bash
printf '%s\n' "$*" >>"$CARGO_LOG"
if [ "${CARGO_FAIL_MATCH:-}" != "" ] &&
   printf '%s' "$*" | grep -Fq "$CARGO_FAIL_MATCH"; then
  printf '%s\n' "${CARGO_FAIL_OUTPUT:-forced cargo failure}" >&2
  exit "${CARGO_FAIL_STATUS:-7}"
fi
if [ "${1:-}" = check ]; then
  printf '%s\n' 'refreshed lock' >Cargo.lock
fi
`
	path := filepath.Join(binDir, "cargo")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return binDir, logPath
}

func runRustBuild(t *testing.T, dir, binDir, logPath string, args ...string) (string, error) {
	t.Helper()
	commandArgs := append([]string{"./build.sh"}, args...)
	cmd := exec.Command("/bin/bash", commandArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+":"+os.Getenv("PATH"),
		"CARGO_LOG="+logPath,
		"NO_COLOR=1",
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestRustBuildScriptSyntaxAndCargoDispatch(t *testing.T) {
	t.Parallel()
	dir := renderRustRepo(t)
	syntax := exec.Command("/bin/bash", "-n", "./build.sh")
	syntax.Dir = dir
	if out, err := syntax.CombinedOutput(); err != nil {
		t.Fatalf("bash -n failed: %v\n%s", err, out)
	}
	binDir, logPath := writeFakeCargo(t, dir)
	if out, err := runRustBuild(t, dir, binDir, logPath); err != nil {
		t.Fatalf("normal build failed: %v\n%s", err, out)
	}
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(string(logBytes)), "\n")
	want := []string{
		"fmt --check",
		"clippy --all-targets --all-features -- -D warnings",
		"test --all-targets --all-features",
		"build --release",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("cargo calls = %#v; want %#v", got, want)
	}

	if err := os.Remove(logPath); err != nil {
		t.Fatal(err)
	}
	if out, err := runRustBuild(t, dir, binDir, logPath, "--verbose"); err != nil {
		t.Fatalf("verbose build failed: %v\n%s", err, out)
	}
	logBytes, err = os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	got = strings.Split(strings.TrimSpace(string(logBytes)), "\n")
	want = []string{
		"fmt --check",
		"clippy --verbose --all-targets --all-features -- -D warnings",
		"test --verbose --all-targets --all-features",
		"build --verbose --release",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("verbose cargo calls = %#v; want %#v", got, want)
	}
}

func TestRustBuildScriptHelpAndFailureGuidance(t *testing.T) {
	t.Parallel()
	dir := renderRustRepo(t)
	binDir, logPath := writeFakeCargo(t, dir)
	for _, flag := range []string{"-h", "-?", "--help"} {
		out, err := runRustBuild(t, dir, binDir, logPath, flag)
		if err != nil || !strings.Contains(out, "Usage: build") {
			t.Errorf("help %s: err=%v out=%q", flag, err, out)
		}
	}
	if out, err := runRustBuild(t, dir, binDir, logPath, "--help", "-v"); err == nil ||
		!strings.Contains(out, "help flags must be used by themselves") {
		t.Fatalf("mixed help: err=%v out=%q", err, out)
	}

	cmd := exec.Command("/bin/bash", "./build.sh")
	cmd.Dir = dir
	cmd.Env = []string{"PATH=" + filepath.Dir(findBash(t)), "NO_COLOR=1"}
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "install the Rust toolchain") {
		t.Fatalf("missing cargo: err=%v out=%s", err, out)
	}

	if err := os.Remove(logPath); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	fmtEnv := append(os.Environ(),
		"PATH="+binDir+":"+os.Getenv("PATH"),
		"CARGO_LOG="+logPath,
		"CARGO_FAIL_MATCH=fmt",
		"CARGO_FAIL_OUTPUT=no such command: fmt",
		"CARGO_FAIL_STATUS=8",
	)
	cmd = exec.Command("/bin/bash", "./build.sh")
	cmd.Dir = dir
	cmd.Env = fmtEnv
	out, err = cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "rustup component add rustfmt") {
		t.Fatalf("rustfmt failure: err=%v out=%s", err, out)
	}
	if err := os.Remove(logPath); err != nil {
		t.Fatal(err)
	}
	failEnv := append(os.Environ(),
		"PATH="+binDir+":"+os.Getenv("PATH"),
		"CARGO_LOG="+logPath,
		"CARGO_FAIL_MATCH=clippy",
		"CARGO_FAIL_OUTPUT=no such command: clippy",
		"CARGO_FAIL_STATUS=9",
	)
	cmd = exec.Command("/bin/bash", "./build.sh")
	cmd.Dir = dir
	cmd.Env = failEnv
	out, err = cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "rustup component add clippy") {
		t.Fatalf("clippy failure: err=%v out=%s", err, out)
	}
	logBytes, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if strings.Contains(string(logBytes), "\ntest ") ||
		strings.Contains(string(logBytes), "\nbuild ") {
		t.Fatalf("commands continued after Clippy failure:\n%s", logBytes)
	}
}

func findBash(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func initializeRustFixtureRepo(t *testing.T, dir, cargo string) {
	t.Helper()
	writeRepoFile(t, dir, "Cargo.toml", cargo)
	writeRepoFile(t, dir, "CHANGELOG.md", "| Version | Summary |\n|---|---|\n| Unreleased | |\n")
	writeRepoFile(t, dir, "plan.md", "## Product Direction\n\n## Ideas To Explore\n")
	mustRunRepoCommand(t, dir, "", "git", "init", "-q")
	mustRunRepoCommand(t, dir, "", "git", "config", "user.name", "Rust Test")
	mustRunRepoCommand(t, dir, "", "git", "config", "user.email", "rust-test@example.com")
	mustRunRepoCommand(t, dir, "", "git", "add", ".")
	mustRunRepoCommand(t, dir, "", "git", "commit", "-qm", "fixture")
}

func TestRustReleasePrepDryRunDoesNotWrite(t *testing.T) {
	t.Parallel()
	dir := renderRustRepo(t)
	cargo := "[package]\nname = \"example\"\nversion = \"0.1.0\"\n"
	initializeRustFixtureRepo(t, dir, cargo)
	before, err := os.ReadFile(filepath.Join(dir, "Cargo.toml"))
	if err != nil {
		t.Fatal(err)
	}
	binDir, logPath := writeFakeCargo(t, dir)
	out, err := runRustBuild(
		t,
		dir,
		binDir,
		logPath,
		"prep",
		"--dry-run",
		"v0.2.0",
		"Rust release",
	)
	if err != nil {
		t.Fatalf("dry-run prep failed: %v\n%s", err, out)
	}
	for _, want := range []string{
		"Cargo.toml [package].version: 0.1.0 -> 0.2.0",
		"./build.sh v0.2.0",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("dry-run output missing %q:\n%s", want, out)
		}
	}
	after, err := os.ReadFile(filepath.Join(dir, "Cargo.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(before, after) {
		t.Fatal("dry-run modified Cargo.toml")
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("dry-run invoked Cargo; log stat=%v", err)
	}
}

func TestRustReleasePrepUpdatesOwnedArtifacts(t *testing.T) {
	t.Parallel()
	dir := renderRustRepo(t)
	cargo := "[package]\nname = \"example\"\nversion  =  \"0.1.0\" # release version\n\n[dependencies]\nanyhow = \"1.0.0\"\n"
	initializeRustFixtureRepo(t, dir, cargo)
	writeRepoFile(t, dir, "governa/ac7-rust-release.md", "# fixture\n")
	writeRepoFile(
		t,
		dir,
		"plan.md",
		"## Product Direction\n\n## Ideas To Explore\n\n- IE7: ship → governa/ac7-rust-release.md\n",
	)
	binDir, logPath := writeFakeCargo(t, dir)
	out, err := runRustBuild(
		t,
		dir,
		binDir,
		logPath,
		"prep",
		"--no-build",
		"v0.2.0",
		"AC7: Rust release",
	)
	if err != nil {
		t.Fatalf("release prep failed: %v\n%s", err, out)
	}
	cargoBytes, err := os.ReadFile(filepath.Join(dir, "Cargo.toml"))
	if err != nil {
		t.Fatal(err)
	}
	cargoContent := string(cargoBytes)
	if !strings.Contains(cargoContent, "version  =  \"0.2.0\" # release version") ||
		!strings.Contains(cargoContent, "anyhow = \"1.0.0\"") {
		t.Fatalf("Cargo.toml version update changed unrelated content:\n%s", cargoContent)
	}
	lockBytes, err := os.ReadFile(filepath.Join(dir, "Cargo.lock"))
	if err != nil || !strings.Contains(string(lockBytes), "refreshed lock") {
		t.Fatalf("Cargo.lock not refreshed: err=%v content=%q", err, lockBytes)
	}
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(logBytes)) != "check --all-targets --all-features" {
		t.Fatalf("--no-build Cargo calls = %q", logBytes)
	}
	changelog, err := os.ReadFile(filepath.Join(dir, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(changelog), "| 0.2.0 | AC7: Rust release |") {
		t.Fatalf("CHANGELOG missing release row:\n%s", changelog)
	}
	if _, err := os.Stat(filepath.Join(dir, "governa", "ac7-rust-release.md")); !os.IsNotExist(err) {
		t.Fatalf("completed AC was not deleted: %v", err)
	}
	plan, err := os.ReadFile(filepath.Join(dir, "plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plan), "ac7-rust-release") {
		t.Fatalf("plan pointer was not removed:\n%s", plan)
	}
	if !strings.Contains(out, "./build.sh v0.2.0") {
		t.Fatalf("release command not emitted:\n%s", out)
	}
}

func TestRustReleasePrepRunsPreAndPostValidation(t *testing.T) {
	t.Parallel()
	dir := renderRustRepo(t)
	cargo := "[package]\nname = \"example\"\nversion = \"0.1.0\"\n"
	initializeRustFixtureRepo(t, dir, cargo)
	binDir, logPath := writeFakeCargo(t, dir)
	out, err := runRustBuild(
		t,
		dir,
		binDir,
		logPath,
		"prep",
		"v0.2.0",
		"Rust release",
	)
	if err != nil {
		t.Fatalf("release prep failed: %v\n%s", err, out)
	}
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(string(logBytes)), "\n")
	want := []string{
		"fmt --check",
		"clippy --all-targets --all-features -- -D warnings",
		"test --all-targets --all-features",
		"build --release",
		"check --all-targets --all-features",
		"fmt --check",
		"clippy --all-targets --all-features -- -D warnings",
		"test --all-targets --all-features",
		"build --release",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("prep Cargo calls = %#v; want %#v", got, want)
	}
}

func TestRustReleaseCancelsOrPushesLightweightTag(t *testing.T) {
	dir := renderRustRepo(t)
	cargo := "[package]\nname = \"example\"\nversion = \"0.1.0\"\n"
	initializeRustFixtureRepo(t, dir, cargo)
	branch := strings.TrimSpace(mustRunRepoCommand(
		t,
		dir,
		"",
		"git",
		"branch",
		"--show-current",
	))
	remote := filepath.Join(t.TempDir(), "remote.git")
	mustRunRepoCommand(t, dir, "", "git", "init", "--bare", "-q", remote)
	mustRunRepoCommand(t, dir, "", "git", "remote", "add", "origin", remote)
	mustRunRepoCommand(t, dir, "", "git", "push", "-q", "-u", "origin", branch)
	writeRepoFile(t, dir, "pending.md", "# Pending release\n")

	binDir, logPath := writeFakeCargo(t, dir)
	runRelease := func(input string) (string, error) {
		cmd := exec.Command("/bin/bash", "./build.sh", "v0.2.0", "Rust release")
		cmd.Dir = dir
		cmd.Stdin = strings.NewReader(input)
		cmd.Env = append(os.Environ(),
			"PATH="+binDir+":"+os.Getenv("PATH"),
			"CARGO_LOG="+logPath,
			"NO_COLOR=1",
		)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	before := strings.TrimSpace(mustRunRepoCommand(t, dir, "", "git", "rev-parse", "HEAD"))
	if out, err := runRelease("n\n"); err == nil {
		t.Fatalf("cancelled release exited successfully:\n%s", out)
	}
	afterCancel := strings.TrimSpace(mustRunRepoCommand(t, dir, "", "git", "rev-parse", "HEAD"))
	if afterCancel != before {
		t.Fatalf("cancelled release changed HEAD: got %s, want %s", afterCancel, before)
	}
	if _, err := runRepoCommand(t, dir, "", "git", "rev-parse", "-q", "--verify", "refs/tags/v0.2.0"); err == nil {
		t.Fatal("cancelled release created a tag")
	}

	if out, err := runRelease("y\n"); err != nil {
		t.Fatalf("confirmed release failed: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(mustRunRepoCommand(t, dir, "", "git", "cat-file", "-t", "v0.2.0")); got != "commit" {
		t.Fatalf("tag object type = %q; want lightweight commit reference", got)
	}
	localHead := strings.TrimSpace(mustRunRepoCommand(t, dir, "", "git", "rev-parse", "HEAD"))
	remoteHead := strings.TrimSpace(mustRunRepoCommand(
		t,
		dir,
		"",
		"git",
		"--git-dir",
		remote,
		"rev-parse",
		"refs/heads/"+branch,
	))
	if remoteHead != localHead {
		t.Fatalf("remote branch head = %s; want %s", remoteHead, localHead)
	}
	remoteTag := strings.TrimSpace(mustRunRepoCommand(
		t,
		dir,
		"",
		"git",
		"--git-dir",
		remote,
		"rev-parse",
		"refs/tags/v0.2.0",
	))
	if remoteTag != localHead {
		t.Fatalf("remote tag target = %s; want %s", remoteTag, localHead)
	}
}

func TestRustReleasePrepRejectsUnsupportedVersions(t *testing.T) {
	t.Parallel()
	fixtures := map[string]string{
		"virtual workspace": "[workspace]\nmembers = [\"crate\"]\n",
		"inherited version": "[package]\nname = \"example\"\nversion.workspace = true\n",
		"missing version":   "[package]\nname = \"example\"\n",
		"duplicate version": "[package]\nname = \"example\"\nversion = \"0.1.0\"\nversion = \"0.1.1\"\n",
		"nonliteral version": "[package]\nname = \"example\"\n" +
			"version = { workspace = true }\n",
	}
	for name, cargo := range fixtures {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := renderRustRepo(t)
			initializeRustFixtureRepo(t, dir, cargo)
			binDir, logPath := writeFakeCargo(t, dir)
			out, err := runRustBuild(
				t,
				dir,
				binDir,
				logPath,
				"prep",
				"--dry-run",
				"v0.2.0",
				"Rust release",
			)
			if err == nil || !strings.Contains(out, "prep:") {
				t.Fatalf("unsupported manifest accepted: err=%v out=%s", err, out)
			}
		})
	}
}

func TestDetectApplyModeNewRepo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if got := detectApplyMode(dir); got != "new" {
		t.Errorf("detectApplyMode on fresh dir = %q; want new", got)
	}
}

// detectApplyMode returns "existing" when AGENTS.md is present.
func TestDetectApplyModeExisting(t *testing.T) {
	t.Parallel()
	dir := newFixtureTarget(t, map[string]string{
		"AGENTS.md": "# AGENTS.md\n",
	})
	if got := detectApplyMode(dir); got != "existing" {
		t.Errorf("detectApplyMode with AGENTS.md = %q; want existing", got)
	}
}

// Removed-symbol trip-wire. Absence is asserted at compile time — if the
// deleted surfaces come back, other tests stop compiling.
// `TestRetiredSymbolsNotPresent` (in retired_symbols_test.go) is the active
// regression guard; this test is retained as a named anchor for the
// retired-symbols set.
func TestRetiredSymbolsAbsent(t *testing.T) {
	t.Parallel()
}

// apply no longer writes a .governa/ directory to consumer repos.
func TestRunApplyStateless(t *testing.T) {
	dir := t.TempDir()

	cfg := Config{
		Mode:     ModeApply,
		Target:   dir,
		Type:     RepoTypeCode,
		RepoName: "x",
		Stack:    "Go",
	}
	if err := RunWithFS(templates.EmbeddedFS, cfg); err != nil {
		t.Fatalf("RunWithFS: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".governa")); err == nil {
		t.Error(".governa/ directory should not be created in consumer repos")
	}
}

// apply produces governa/ac1-governa-apply.md adoption record.
func TestRunApplyProducesAdoptionAC(t *testing.T) {
	dir := t.TempDir()

	cfg := Config{
		Mode:     ModeApply,
		Target:   dir,
		Type:     RepoTypeCode,
		RepoName: "test-repo",
		Stack:    "Go",
	}
	if err := RunWithFS(templates.EmbeddedFS, cfg); err != nil {
		t.Fatalf("RunWithFS: %v", err)
	}

	acPath := filepath.Join(dir, "governa", "ac1-governa-apply.md")
	content, err := os.ReadFile(acPath)
	if err != nil {
		t.Fatalf("read adoption AC: %v", err)
	}
	text := string(content)
	mustContain(t, text, "# AC1 Governa Apply")
	mustContain(t, text, "## Summary")
	mustContain(t, text, "## In Scope")
	mustContain(t, text, "## Status")
	mustContain(t, text, "consumer-owned")
	// AT2: nested files appear as repo-relative slash paths
	// in the In Scope list, not as bare basenames.
	mustContain(t, text, "- `governa/development-cycle.md`")
}

// renderApplyAC lists files from operations and marks consumer ownership.
// list entries use repo-relative slash paths, not basenames.
func TestRenderApplyACShape(t *testing.T) {
	t.Parallel()
	const targetAbs = "/tmp/t"
	ops := []operation{
		{kind: "write", path: filepath.Join(targetAbs, "AGENTS.md"), note: "governance contract"},
		{kind: "symlink", path: filepath.Join(targetAbs, "CLAUDE.md"), linkTo: "AGENTS.md"},
		{kind: "write", path: filepath.Join(targetAbs, "governa", "roles.md"), note: "overlay file"},
		{kind: "skip"},
	}
	out := renderApplyAC("0.60.0", Config{Type: RepoTypeCode, RepoName: "x"}, ops, targetAbs)
	mustContain(t, out, "# AC1 Governa Apply")
	mustContain(t, out, "0.60.0")
	mustContain(t, out, "AGENTS.md")
	mustContain(t, out, "CLAUDE.md")
	mustContain(t, out, "consumer-owned")
	mustContain(t, out, "## Acceptance Tests")
	// AT1: nested files render as repo-relative slash paths,
	// never as basename-only.
	mustContain(t, out, "- `governa/roles.md`")
	for line := range strings.SplitSeq(out, "\n") {
		if line == "- `roles.md`" || strings.HasPrefix(line, "- `roles.md` (") {
			t.Errorf("nested entry should not render as basename-only; got line: %q", line)
		}
	}
	if strings.Count(out, "skip") > 0 {
		lines := strings.SplitSeq(out, "\n")
		for l := range lines {
			if strings.HasPrefix(l, "- `skip`") {
				t.Errorf("skip operations should not appear in the file list; got line: %s", l)
			}
		}
	}
}

func renderDocRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cfg := Config{
		Mode:     ModeApply,
		Target:   dir,
		Type:     RepoTypeDoc,
		RepoName: "docs-test",
	}
	if err := RunWithFS(templates.EmbeddedFS, cfg); err != nil {
		t.Fatalf("render DOC repo: %v", err)
	}
	return dir
}

func runRepoCommand(t *testing.T, dir, input, name string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "TERM=dumb")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func mustRunRepoCommand(t *testing.T, dir, input, name string, args ...string) string {
	t.Helper()
	out, err := runRepoCommand(t, dir, input, name, args...)
	if err != nil {
		t.Fatalf("run %s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return out
}

func writeRepoFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	abs := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func initRepoForShellTests(t *testing.T, dir string) string {
	t.Helper()
	mustRunRepoCommand(t, dir, "", "git", "init", "-q")
	mustRunRepoCommand(t, dir, "", "git", "config", "user.email", "docs-test@example.com")
	mustRunRepoCommand(t, dir, "", "git", "config", "user.name", "DOC Test")
	mustRunRepoCommand(t, dir, "", "git", "add", ".")
	mustRunRepoCommand(t, dir, "", "git", "commit", "-q", "-m", "initial")
	return strings.TrimSpace(mustRunRepoCommand(t, dir, "", "git", "rev-parse", "--abbrev-ref", "HEAD"))
}

func TestDocApplyEmitsShellTooling(t *testing.T) {
	dir := renderDocRepo(t)
	buildPath := filepath.Join(dir, "build.sh")
	info, err := os.Stat(buildPath)
	if err != nil {
		t.Fatalf("build.sh not emitted: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("build.sh mode %v is not executable", info.Mode())
	}
	content, err := os.ReadFile(buildPath)
	if err != nil {
		t.Fatalf("read build.sh: %v", err)
	}
	for _, want := range []string{"prep_main", "rel_run"} {
		mustContain(t, string(content), want)
	}
	if strings.Contains(string(content), "mdcheck") {
		t.Error("DOC build.sh contains content-validation tooling")
	}
	agents, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	mustContain(t, string(agents), "Complete any repo-owned validation before preparing any commit handoff.")
	usage := mustRunRepoCommand(t, dir, "", "./build.sh")
	mustContain(t, usage, "build prep")
	for _, rel := range []string{"rel.sh", "cmd/rel/main.go", "cmd/rel/color.go"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); !os.IsNotExist(err) {
			t.Errorf("retired DOC release path %s was emitted", rel)
		}
	}
}

func TestDocReleaseCancelsOrPushesAnnotatedTag(t *testing.T) {
	dir := renderDocRepo(t)
	branch := initRepoForShellTests(t, dir)
	remote := filepath.Join(t.TempDir(), "remote.git")
	mustRunRepoCommand(t, dir, "", "git", "init", "--bare", "-q", remote)
	mustRunRepoCommand(t, dir, "", "git", "remote", "add", "origin", remote)
	mustRunRepoCommand(t, dir, "", "git", "push", "-q", "-u", "origin", branch)

	writeRepoFile(t, dir, "pending.md", "# Pending release\n")
	before := strings.TrimSpace(mustRunRepoCommand(t, dir, "", "git", "rev-parse", "HEAD"))
	out, err := runRepoCommand(t, dir, "n\n", "./build.sh", "v1.2.3", "DOC release")
	if err == nil {
		t.Fatalf("cancelled release exited successfully:\n%s", out)
	}
	afterCancel := strings.TrimSpace(mustRunRepoCommand(t, dir, "", "git", "rev-parse", "HEAD"))
	if afterCancel != before {
		t.Errorf("cancelled release changed HEAD: got %s, want %s", afterCancel, before)
	}
	if _, err := runRepoCommand(t, dir, "", "git", "rev-parse", "-q", "--verify", "refs/tags/v1.2.3"); err == nil {
		t.Error("cancelled release created tag v1.2.3")
	}

	mustRunRepoCommand(t, dir, "y\n", "./build.sh", "v1.2.3", "DOC release")
	if got := strings.TrimSpace(mustRunRepoCommand(t, dir, "", "git", "cat-file", "-t", "v1.2.3")); got != "tag" {
		t.Errorf("tag object type = %q; want annotated tag object", got)
	}
	tagBody := mustRunRepoCommand(t, dir, "", "git", "for-each-ref", "--format=%(contents)", "refs/tags/v1.2.3")
	if strings.TrimSpace(tagBody) != "DOC release" {
		t.Errorf("annotated tag message = %q; want %q", strings.TrimSpace(tagBody), "DOC release")
	}
	localHead := strings.TrimSpace(mustRunRepoCommand(t, dir, "", "git", "rev-parse", "HEAD"))
	remoteHead := strings.TrimSpace(mustRunRepoCommand(t, dir, "", "git", "--git-dir", remote, "rev-parse", "refs/heads/"+branch))
	if remoteHead != localHead {
		t.Errorf("remote branch head = %s; want %s", remoteHead, localHead)
	}
	remoteTag := strings.TrimSpace(mustRunRepoCommand(t, dir, "", "git", "--git-dir", remote, "rev-parse", "refs/tags/v1.2.3^{}"))
	if remoteTag != localHead {
		t.Errorf("remote tag target = %s; want %s", remoteTag, localHead)
	}
}

func TestDocPrepStagesReleaseWithoutVersionBump(t *testing.T) {
	dir := renderDocRepo(t)
	initRepoForShellTests(t, dir)
	writeRepoFile(t, dir, "governa/ac42-doc-update.md", "# Documentation update\n")
	writeRepoFile(t, dir, "cmd/example/main.go", "package main\n\nconst programVersion = \"0.1.0\"\n")
	planPath := filepath.Join(dir, "plan.md")
	plan, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("read plan.md: %v", err)
	}
	writeRepoFile(t, dir, "plan.md", string(plan)+"\n- IE9: ship docs → governa/ac42-doc-update.md\n")
	mustRunRepoCommand(t, dir, "", "git", "add", ".")

	changelogPath := filepath.Join(dir, "CHANGELOG.md")
	beforeChangelog, err := os.ReadFile(changelogPath)
	if err != nil {
		t.Fatalf("read CHANGELOG.md: %v", err)
	}
	beforePlan, _ := os.ReadFile(planPath)
	for _, flag := range []string{"--dry-run", "-n"} {
		out := mustRunRepoCommand(t, dir, "", "./build.sh", "prep", flag, "v1.2.3", "AC42: docs")
		mustContain(t, out, "release command:")
		if got, _ := os.ReadFile(changelogPath); string(got) != string(beforeChangelog) {
			t.Errorf("%s modified CHANGELOG.md", flag)
		}
		if got, _ := os.ReadFile(planPath); string(got) != string(beforePlan) {
			t.Errorf("%s modified plan.md", flag)
		}
	}

	for _, args := range [][]string{
		{"prep", "bad", "message"},
		{"prep", "v1.2.3", "   "},
		{"prep", "v1.2.3", strings.Repeat("x", 81)},
		{"prep", "--no-build", "v1.2.3", "message"},
	} {
		if out, err := runRepoCommand(t, dir, "", "./build.sh", args...); err == nil {
			t.Errorf("invalid prep args succeeded: %v\n%s", args, out)
		}
	}

	out := mustRunRepoCommand(t, dir, "", "./build.sh", "prep", "v1.2.3", "AC42: docs")
	mustContain(t, out, "release command:")
	if strings.Contains(out, "check build") || strings.Contains(out, "validation") {
		t.Errorf("DOC prep ran or reported content validation:\n%s", out)
	}
	changelog, _ := os.ReadFile(changelogPath)
	mustContain(t, string(changelog), "| Unreleased |")
	mustContain(t, string(changelog), "| 1.2.3 | AC42: docs |")
	if strings.Index(string(changelog), "| Unreleased |") > strings.Index(string(changelog), "| 1.2.3 | AC42: docs |") {
		t.Error("release row was not inserted below the Unreleased row")
	}
	if _, err := os.Stat(filepath.Join(dir, "governa/ac42-doc-update.md")); !os.IsNotExist(err) {
		t.Error("prep did not delete the release-message AC file")
	}
	updatedPlan, _ := os.ReadFile(planPath)
	if strings.Contains(string(updatedPlan), "governa/ac42-doc-update.md") {
		t.Error("prep did not sweep the matching plan.md IE")
	}
	versionSource, _ := os.ReadFile(filepath.Join(dir, "cmd/example/main.go"))
	mustContain(t, string(versionSource), `programVersion = "0.1.0"`)

	stableChangelog := string(changelog)
	if duplicateOut, err := runRepoCommand(t, dir, "", "./build.sh", "prep", "v1.2.3", "AC42: docs"); err == nil {
		t.Errorf("duplicate CHANGELOG row succeeded:\n%s", duplicateOut)
	}
	if got, _ := os.ReadFile(changelogPath); string(got) != stableChangelog {
		t.Error("duplicate-row failure modified CHANGELOG.md")
	}
}

// Helper that calls t.Errorf with the full string if assertion fails.
func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("missing substring %q in:\n%s", needle, haystack)
	}
}
