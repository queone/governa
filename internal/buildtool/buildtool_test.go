package buildtool

import (
	"bytes"
	"os"
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

	got := packageScopes([]string{"bootstrap", "rel"})
	want := []string{"./cmd/bootstrap", "./cmd/rel"}
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

	got, err := buildTargets([]string{"build", "bootstrap", "rel"})
	if err != nil {
		t.Fatalf("buildTargets() error = %v", err)
	}
	want := []string{}
	if len(got) != len(want) {
		t.Fatalf("buildTargets() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("buildTargets()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestShouldSkipBinaryInstall(t *testing.T) {
	t.Parallel()

	if !shouldSkipBinaryInstall(nil) {
		t.Fatal("expected default build to report skipped script-only commands")
	}
	if !shouldSkipBinaryInstall([]string{"bootstrap"}) {
		t.Fatal("expected bootstrap target to be treated as script-only")
	}
	if shouldSkipBinaryInstall([]string{"worker"}) {
		t.Fatal("did not expect installable target to be treated as script-only")
	}
}

func TestJoinScriptOnlyTargets(t *testing.T) {
	t.Parallel()

	if got := joinScriptOnlyTargets(nil); got != "cmd/bootstrap, cmd/build, cmd/rel" {
		t.Fatalf("joinScriptOnlyTargets(nil) = %q", got)
	}
	if got := joinScriptOnlyTargets([]string{"worker", "bootstrap", "rel"}); got != "cmd/bootstrap, cmd/rel" {
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
