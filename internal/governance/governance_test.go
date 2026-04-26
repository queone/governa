package governance

import (
	"os"
	"path/filepath"
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

// AC79 Part B AT8: `--no` flag is no longer recognized.
func TestParseFlagsRejectsNo(t *testing.T) {
	t.Parallel()
	_, _, err := parseFlags(ModeApply, []string{"--no", "--target", "/tmp/x"})
	if err == nil {
		t.Fatal("expected flag-parse error for removed --no flag; got nil")
	}
}

// AC79 Part B AT9: `--dry-run` flag is no longer recognized.
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

// AC88: --yes flag is removed (no collision negotiation).
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

// AC88: help text describes consumer ownership, not collision/review.
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

// AC88: --yes must NOT appear in help (removed).
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

// AC79 F-new-2: --dry-run must NOT appear as a flag-list row (it was retired).
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
	err := RunWithFS(templates.EmbeddedFS, "", Config{Mode: Mode("enhance")})
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

func TestDetectApplyModeNewRepo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if got := detectApplyMode(dir); got != "new" {
		t.Errorf("detectApplyMode on fresh dir = %q; want new", got)
	}
}

func TestDetectApplyModeReApply(t *testing.T) {
	t.Parallel()
	dir := newFixtureTarget(t, map[string]string{
		".governa/manifest": "governa-manifest-v1\ntemplate-version: 0.50.0\n",
	})
	if got := detectApplyMode(dir); got != "re-apply" {
		t.Errorf("detectApplyMode with manifest = %q; want re-apply", got)
	}
}

func TestBuildManifestMinimalShape(t *testing.T) {
	t.Parallel()
	m := buildManifest("1.2.3", ManifestParams{RepoName: "x", Type: "CODE"})
	if m.TemplateVersion != "1.2.3" {
		t.Errorf("TemplateVersion = %q; want 1.2.3", m.TemplateVersion)
	}
	if m.Params.RepoName != "x" {
		t.Errorf("Params.RepoName = %q; want x", m.Params.RepoName)
	}
}

// AC79 Part B AT10: removed-symbol trip-wire. Absence is asserted at
// compile time — if the deleted surfaces come back, other tests stop
// compiling. AC80 AT13's `TestRetiredSymbolsNotPresent` is the active
// regression guard; this test is retained as a named anchor for the
// AC79 retirement set.
func TestAC79RemovedSymbols(t *testing.T) {
	t.Parallel()
}

// AC88 AT1: TEMPLATE_VERSION is always overwritten on apply.
func TestRunApplyAlwaysWritesTemplateVersion(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "TEMPLATE_VERSION"), []byte("0.0.1\n"), 0o644); err != nil {
		t.Fatalf("seed TEMPLATE_VERSION: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".governa"), 0o755); err != nil {
		t.Fatalf("mkdir .governa: %v", err)
	}
	manifestContent := "governa-manifest-v1\ntemplate-version: 0.0.1\nrepo-name: x\ntype: CODE\nstack: Go\n"
	if err := os.WriteFile(filepath.Join(dir, ".governa", "manifest"), []byte(manifestContent), 0o644); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}

	cfg := Config{
		Mode:     ModeApply,
		Target:   dir,
		Type:     RepoTypeCode,
		RepoName: "x",
		Stack:    "Go",
	}
	if err := RunWithFS(templates.EmbeddedFS, "", cfg); err != nil {
		t.Fatalf("RunWithFS: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "TEMPLATE_VERSION"))
	if err != nil {
		t.Fatalf("read post-apply TEMPLATE_VERSION: %v", err)
	}
	if strings.TrimSpace(string(got)) != strings.TrimSpace(templates.TemplateVersion) {
		t.Errorf("TEMPLATE_VERSION = %q; want %q", string(got), templates.TemplateVersion)
	}
}

// AC88: apply produces docs/ac1-governa-apply.md adoption record.
func TestRunApplyProducesAdoptionAC(t *testing.T) {
	dir := t.TempDir()

	cfg := Config{
		Mode:     ModeApply,
		Target:   dir,
		Type:     RepoTypeCode,
		RepoName: "test-repo",
		Stack:    "Go",
	}
	if err := RunWithFS(templates.EmbeddedFS, "", cfg); err != nil {
		t.Fatalf("RunWithFS: %v", err)
	}

	acPath := filepath.Join(dir, "docs", "ac1-governa-apply.md")
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
}

// AC88: renderApplyAC lists files from operations and marks consumer ownership.
func TestRenderApplyACShape(t *testing.T) {
	t.Parallel()
	ops := []operation{
		{kind: "write", path: "/tmp/t/AGENTS.md", note: "governance contract"},
		{kind: "symlink", path: "/tmp/t/CLAUDE.md", linkTo: "AGENTS.md"},
		{kind: "skip"},
	}
	out := renderApplyAC("0.60.0", Config{Type: RepoTypeCode, RepoName: "x"}, ops)
	mustContain(t, out, "# AC1 Governa Apply")
	mustContain(t, out, "0.60.0")
	mustContain(t, out, "AGENTS.md")
	mustContain(t, out, "CLAUDE.md")
	mustContain(t, out, "consumer-owned")
	mustContain(t, out, "## Acceptance Tests")
	if strings.Count(out, "skip") > 0 {
		lines := strings.SplitSeq(out, "\n")
		for l := range lines {
			if strings.HasPrefix(l, "- `skip`") {
				t.Errorf("skip operations should not appear in the file list; got line: %s", l)
			}
		}
	}
}

// Helper that calls t.Errorf with the full string if assertion fails.
func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("missing substring %q in:\n%s", needle, haystack)
	}
}
