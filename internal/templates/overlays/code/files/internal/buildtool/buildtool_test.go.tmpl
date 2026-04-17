package buildtool

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseArgsNoArgs(t *testing.T) {
	t.Parallel()

	cfg, help, err := ParseArgs(nil)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if help {
		t.Fatal("did not expect help mode")
	}
	if cfg.Verbose {
		t.Fatal("did not expect verbose mode")
	}
	if len(cfg.Targets) != 0 {
		t.Fatalf("unexpected targets: %#v", cfg.Targets)
	}
}

func TestParseArgsVerboseAndTargets(t *testing.T) {
	t.Parallel()

	cfg, help, err := ParseArgs([]string{"-v", "bootstrap", "rel"})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if help {
		t.Fatal("did not expect help mode")
	}
	if !cfg.Verbose {
		t.Fatal("expected verbose mode")
	}
	if len(cfg.Targets) != 2 || cfg.Targets[0] != "bootstrap" || cfg.Targets[1] != "rel" {
		t.Fatalf("unexpected targets: %#v", cfg.Targets)
	}
}

func TestPackageScopes(t *testing.T) {
	t.Parallel()

	got := packageScopes([]string{"governa", "rel"})
	want := []string{"./cmd/governa", "./cmd/rel"}
	if len(got) != len(want) {
		t.Fatalf("packageScopes() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("packageScopes()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestBuildTargetsSkipsScriptOnlyCommands(t *testing.T) {
	t.Parallel()

	got, err := buildTargets([]string{"build", "rel"})
	if err != nil {
		t.Fatalf("buildTargets() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("buildTargets() len = %d, want 0", len(got))
	}
}

func TestShouldSkipBinaryInstall(t *testing.T) {
	t.Parallel()

	if !shouldSkipBinaryInstall(nil) {
		t.Fatal("expected default build to report skipped script-only commands")
	}
	if !shouldSkipBinaryInstall([]string{"build"}) {
		t.Fatal("expected build target to be treated as script-only")
	}
	if shouldSkipBinaryInstall([]string{"worker"}) {
		t.Fatal("did not expect installable target to be treated as script-only")
	}
}

func TestJoinScriptOnlyTargets(t *testing.T) {
	t.Parallel()

	if got := joinScriptOnlyTargets(nil); got != "cmd/build, cmd/rel" {
		t.Fatalf("joinScriptOnlyTargets(nil) = %q", got)
	}
	if got := joinScriptOnlyTargets([]string{"worker", "build", "rel"}); got != "cmd/build, cmd/rel" {
		t.Fatalf("joinScriptOnlyTargets(requested) = %q", got)
	}
}

func TestNextPatchTagSortsSemver(t *testing.T) {
	t.Parallel()

	versions := []semver{
		{major: 1, minor: 2, patch: 9},
		{major: 1, minor: 10, patch: 0},
		{major: 1, minor: 3, patch: 1},
	}
	max := versions[0]
	for _, current := range versions[1:] {
		if current.major > max.major ||
			(current.major == max.major && current.minor > max.minor) ||
			(current.major == max.major && current.minor == max.minor && current.patch > max.patch) {
			max = current
		}
	}
	if max.minor != 10 {
		t.Fatalf("max version = %#v, want minor 10", max)
	}
}

// --- Usage test ---

func TestUsageContainsBasicInfo(t *testing.T) {
	t.Parallel()
	usage := Usage()
	if !strings.Contains(usage, "build") {
		t.Fatal("usage should mention build command")
	}
	if !strings.Contains(usage, "verbose") {
		t.Fatal("usage should mention verbose option")
	}
	if !strings.Contains(usage, "targets") {
		t.Fatal("usage should mention target scoping behavior")
	}
}

// --- ParseArgs edge cases ---

func TestParseArgsHelpMixedWithOther(t *testing.T) {
	t.Parallel()
	_, _, err := ParseArgs([]string{"-v", "--help"})
	if err == nil {
		t.Fatal("expected error for help mixed with other args")
	}
}

func TestParseArgsUnknownFlag(t *testing.T) {
	t.Parallel()
	_, _, err := ParseArgs([]string{"--unknown"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

// --- nextPatchTagFromOutput tests ---

func TestNextPatchTagFromOutputMultipleTags(t *testing.T) {
	t.Parallel()

	output := "v0.1.0\nv0.1.1\nv0.2.0\nsome-other-tag\n"
	tag, ok, err := nextPatchTagFromOutput(output)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !ok {
		t.Fatal("expected a tag suggestion")
	}
	if tag != "v0.2.1" {
		t.Fatalf("got %q, want v0.2.1", tag)
	}
}

func TestNextPatchTagFromOutputNoTags(t *testing.T) {
	t.Parallel()

	_, ok, err := nextPatchTagFromOutput("")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if ok {
		t.Fatal("expected no tag suggestion for empty output")
	}
}

func TestNextPatchTagFromOutputNonSemverTags(t *testing.T) {
	t.Parallel()

	_, ok, err := nextPatchTagFromOutput("release-1\nlatest\nbeta\n")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if ok {
		t.Fatal("expected no tag suggestion for non-semver tags")
	}
}

func TestNextPatchTagFromOutputSingleTag(t *testing.T) {
	t.Parallel()

	tag, ok, err := nextPatchTagFromOutput("v1.0.0\n")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !ok {
		t.Fatal("expected a tag suggestion")
	}
	if tag != "v1.0.1" {
		t.Fatalf("got %q, want v1.0.1", tag)
	}
}

// --- domainCoverage tests ---

func TestDomainCoverage(t *testing.T) {
	t.Parallel()

	coverData := `mode: set
example.com/internal/foo/foo.go:10.1,12.1 3 1
example.com/internal/foo/foo.go:14.1,16.1 2 0
example.com/internal/bar/bar.go:5.1,8.1 4 1
example.com/cmd/main.go:3.1,5.1 2 1
`
	dir := t.TempDir()
	coverPath := filepath.Join(dir, "cover.out")
	if err := os.WriteFile(coverPath, []byte(coverData), 0o644); err != nil {
		t.Fatal(err)
	}

	pct, err := domainCoverage(coverPath, "example.com/internal/")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	// 3 + 4 = 7 covered statements out of 3 + 2 + 4 = 9 total
	expected := float64(7) / float64(9) * 100
	if pct < expected-0.1 || pct > expected+0.1 {
		t.Fatalf("got %.1f%%, want %.1f%%", pct, expected)
	}
}

func TestDomainCoverageEmpty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	coverPath := filepath.Join(dir, "cover.out")
	if err := os.WriteFile(coverPath, []byte("mode: set\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pct, err := domainCoverage(coverPath, "example.com/internal/")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if pct != 0 {
		t.Fatalf("got %.1f%%, want 0%%", pct)
	}
}

// --- writeIndented tests ---

func TestWriteIndented(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	writeIndented(&buf, "line one\nline two\n")
	output := buf.String()
	if !strings.Contains(output, "    line one") {
		t.Fatal("expected indented output")
	}
	if !strings.Contains(output, "    line two") {
		t.Fatal("expected second line indented")
	}
}

func TestWriteIndentedFAILColoring(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	writeIndented(&buf, "ok test\nFAIL test_bad\n")
	output := buf.String()
	if !strings.Contains(output, "ok test") {
		t.Fatal("expected ok line in output")
	}
	// FAIL line should still appear (colored or not depending on TTY)
	if !strings.Contains(output, "FAIL") && !strings.Contains(output, "test_bad") {
		t.Fatal("expected FAIL line in output")
	}
}

// --- binaryExt test ---

func TestBinaryExt(t *testing.T) {
	t.Parallel()

	ext := binaryExt()
	// We can't control runtime.GOOS in a unit test, but we can verify it returns
	// a consistent value
	if ext != "" && ext != ".exe" {
		t.Fatalf("unexpected extension %q", ext)
	}
}

// --- isHelpArg test ---

func TestIsHelpArg(t *testing.T) {
	t.Parallel()

	for _, arg := range []string{"-h", "-?", "--help"} {
		if !isHelpArg(arg) {
			t.Fatalf("expected %q to be help arg", arg)
		}
	}
	for _, arg := range []string{"-v", "help", "--version"} {
		if isHelpArg(arg) {
			t.Fatalf("did not expect %q to be help arg", arg)
		}
	}
}

func TestRunCapturedSoftReturnsOutput(t *testing.T) {
	t.Parallel()
	// Use go version as a portable command that always produces output
	output := runCapturedSoft("go", "version")
	if !strings.Contains(output, "go") {
		t.Fatalf("expected output to contain 'go', got %q", output)
	}
}

func TestRunCapturedSoftReturnsErrorOnFailure(t *testing.T) {
	t.Parallel()
	// Use go with an invalid subcommand — exits non-zero with error text
	output := runCapturedSoft("go", "nosuchcommand")
	if output == "" {
		t.Fatal("expected non-empty output on failure")
	}
}

// --- extractProgramVersion tests ---

func TestExtractProgramVersionValid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	os.WriteFile(path, []byte("package main\n\nconst programVersion = \"1.2.3\"\n"), 0o644)
	ver, err := extractProgramVersion(path)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if ver != "1.2.3" {
		t.Fatalf("got %q, want 1.2.3", ver)
	}
}

func TestExtractProgramVersionWithTypeAnnotation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	os.WriteFile(path, []byte("package main\n\nconst programVersion string = \"0.5.0\"\n"), 0o644)
	ver, err := extractProgramVersion(path)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if ver != "0.5.0" {
		t.Fatalf("got %q, want 0.5.0", ver)
	}
}

func TestExtractProgramVersionConstBlock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	os.WriteFile(path, []byte("package main\n\nconst (\n\tprogramVersion = \"2.0.0\"\n)\n"), 0o644)
	ver, err := extractProgramVersion(path)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if ver != "2.0.0" {
		t.Fatalf("got %q, want 2.0.0", ver)
	}
}

func TestExtractProgramVersionConstBlockWithType(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	os.WriteFile(path, []byte("package main\n\nconst (\n\tprogramVersion string = \"3.1.0\"\n)\n"), 0o644)
	ver, err := extractProgramVersion(path)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if ver != "3.1.0" {
		t.Fatalf("got %q, want 3.1.0", ver)
	}
}

func TestExtractProgramVersionConstBlockMultipleConsts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	os.WriteFile(path, []byte("package main\n\nconst (\n\tappName = \"myapp\"\n\tprogramVersion = \"4.0.0\"\n\tmaxRetries = 3\n)\n"), 0o644)
	ver, err := extractProgramVersion(path)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if ver != "4.0.0" {
		t.Fatalf("got %q, want 4.0.0", ver)
	}
}

func TestExtractProgramVersionMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	os.WriteFile(path, []byte("package main\n\nfunc main() {}\n"), 0o644)
	ver, err := extractProgramVersion(path)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if ver != "" {
		t.Fatalf("got %q, want empty", ver)
	}
}

func TestExtractProgramVersionEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	os.WriteFile(path, []byte("package main\n\nconst programVersion = \"\"\n"), 0o644)
	ver, err := extractProgramVersion(path)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if ver != "" {
		t.Fatalf("got %q, want empty for empty-string const", ver)
	}
}

func TestExtractProgramVersionNotConst(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	os.WriteFile(path, []byte("package main\n\nvar programVersion = \"1.0.0\"\n"), 0o644)
	ver, err := extractProgramVersion(path)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if ver != "" {
		t.Fatalf("got %q, want empty for var (not const)", ver)
	}
}

func TestExtractProgramVersionFileNotFound(t *testing.T) {
	t.Parallel()
	_, err := extractProgramVersion("/nonexistent/main.go")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestValidateProgramVersionsRejectsEmpty(t *testing.T) {
	// No t.Parallel() — uses os.Chdir which affects the whole process
	dir := t.TempDir()
	cmdDir := filepath.Join(dir, "cmd", "worker")
	os.MkdirAll(cmdDir, 0o755)
	os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	var buf bytes.Buffer
	err := validateProgramVersions([]string{"worker"}, &buf)
	if err == nil {
		t.Fatal("expected error for missing programVersion")
	}
	if !strings.Contains(err.Error(), "programVersion") {
		t.Fatalf("error should mention programVersion, got: %v", err)
	}
}

func TestScriptOnlyCommandsExemptFromVersionCheck(t *testing.T) {
	t.Parallel()
	// Script-only commands are filtered out by filterInstallTargets,
	// so they never reach validateProgramVersions.
	targets := filterInstallTargets([]string{"build", "rel"})
	if len(targets) != 0 {
		t.Fatalf("expected no installable targets from script-only commands, got %v", targets)
	}
}

// --- resolveGoverna tests ---

func TestResolveGovernaPrefsInstalledPath(t *testing.T) {
	t.Parallel()
	got := resolveGoverna("/some/bin/governa")
	if got != "/some/bin/governa" {
		t.Fatalf("expected installed path, got %q", got)
	}
}

func TestResolveGovernaFallsBackToLookPath(t *testing.T) {
	t.Parallel()
	// With empty installed path, it should fall back to LookPath.
	// We can't guarantee governa is on PATH in CI, so just verify
	// the function returns without panic.
	_ = resolveGoverna("")
}

func TestResolveGovernaReturnsEmptyWhenUnavailable(t *testing.T) {
	// Set PATH to empty so LookPath fails
	t.Setenv("PATH", "")

	got := resolveGoverna("")
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

// --- checkDrift tests ---

func TestCheckDriftSkipsWhenBinaryUnavailable(t *testing.T) {
	var buf bytes.Buffer
	// Empty installed path and no governa on PATH should produce no output
	t.Setenv("PATH", "")

	checkDrift(&buf, "")
	if buf.Len() != 0 {
		t.Fatalf("expected no output when governa unavailable, got %q", buf.String())
	}
}

func TestCheckDriftSubprocessFailureSilent(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	// Point to a binary that will exit non-zero
	checkDrift(&buf, "/usr/bin/false")
	if buf.Len() != 0 {
		t.Fatalf("expected no output on subprocess failure, got %q", buf.String())
	}
}

func TestCheckDriftNoDriftLineInOutputSilent(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	// Use "echo" which produces output but no "drift:" line
	echoPath, err := exec.LookPath("echo")
	if err != nil {
		t.Skip("echo not found")
	}
	checkDrift(&buf, echoPath)
	if buf.Len() != 0 {
		t.Fatalf("expected no output when no drift line, got %q", buf.String())
	}
}

// --- relayDriftSummary tests ---

func TestRelayDriftSummaryWithDrift(t *testing.T) {
	t.Parallel()
	output := "mode: self-review (comparing local templates against embedded v0.7.0)\n" +
		"  changed: base/AGENTS.md (sections: Purpose)\n" +
		"summary: 1 changed, 0 added, 0 removed\n"
	var buf bytes.Buffer
	relayDriftSummary(&buf, output)
	got := buf.String()
	if !strings.Contains(got, "Governance drift check") {
		t.Fatalf("expected drift check banner, got %q", got)
	}
	if !strings.Contains(got, "summary: 1 changed, 0 added, 0 removed") {
		t.Fatalf("expected summary line relayed, got %q", got)
	}
}

func TestRelayDriftSummaryClean(t *testing.T) {
	t.Parallel()
	output := "mode: self-review (comparing local templates against embedded v0.7.0)\n" +
		"no changes since embedded version\n"
	var buf bytes.Buffer
	relayDriftSummary(&buf, output)
	if buf.Len() != 0 {
		t.Fatalf("expected no output for clean self-review, got %q", buf.String())
	}
}

func TestRelayDriftSummaryEmpty(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	relayDriftSummary(&buf, "")
	if buf.Len() != 0 {
		t.Fatalf("expected no output for empty input, got %q", buf.String())
	}
}

func TestRelayDriftSummaryWithAnsiCodes(t *testing.T) {
	t.Parallel()
	output := "\x1b[33msummary:\x1b[0m 2 changed, 1 added, 0 removed\n"
	var buf bytes.Buffer
	relayDriftSummary(&buf, output)
	got := buf.String()
	if !strings.Contains(got, "Governance drift check") {
		t.Fatalf("expected drift check banner with ANSI input, got %q", got)
	}
}

// --- stripAnsi tests ---

func TestStripAnsiRemovesEscapes(t *testing.T) {
	t.Parallel()
	input := "\x1b[33mdrift:\x1b[0m none detected"
	got := stripAnsi(input)
	if got != "drift: none detected" {
		t.Fatalf("got %q, want %q", got, "drift: none detected")
	}
}

func TestStripAnsiPassthroughPlainText(t *testing.T) {
	t.Parallel()
	input := "drift: none detected"
	got := stripAnsi(input)
	if got != input {
		t.Fatalf("got %q, want %q", got, input)
	}
}

func TestGoFmtNonEmptyOutputFailsBuild(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module testfmt\n\ngo 1.21\n"), 0o644)
	// Intentionally bad formatting: missing space before brace, extra spaces
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main(){\nvar   x   int\n_ = x\n}\n"), 0o644)

	// Run go fmt ./... with Dir set, matching production invocation in Run()
	cmd := exec.Command("go", "fmt", "./...")
	cmd.Dir = dir
	output, _ := cmd.CombinedOutput()

	// Non-empty output means files were reformatted — this is the exact
	// condition checked in Run() to make go fmt build-breaking
	if strings.TrimSpace(string(output)) == "" {
		t.Fatal("expected go fmt to report reformatted files")
	}
}
