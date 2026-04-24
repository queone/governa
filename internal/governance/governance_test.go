package governance

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

func TestParseFlagsSyncDefaults(t *testing.T) {
	t.Parallel()
	cfg, help, err := parseFlags(ModeSync, []string{"--target", "/tmp/nope"})
	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	if help {
		t.Fatal("unexpected help request")
	}
	if cfg.Mode != ModeSync {
		t.Errorf("Mode = %q; want %q", cfg.Mode, ModeSync)
	}
	if cfg.Target != "/tmp/nope" {
		t.Errorf("Target = %q; want /tmp/nope", cfg.Target)
	}
	if cfg.AssumeYes {
		t.Errorf("expected AssumeYes=false by default")
	}
}

// AC79 Part B AT8: `--no` flag is no longer recognized.
func TestParseFlagsRejectsNo(t *testing.T) {
	t.Parallel()
	_, _, err := parseFlags(ModeSync, []string{"--no", "--target", "/tmp/x"})
	if err == nil {
		t.Fatal("expected flag-parse error for removed --no flag; got nil")
	}
}

// AC79 Part B AT9: `--dry-run` flag is no longer recognized.
func TestParseFlagsRejectsDryRun(t *testing.T) {
	t.Parallel()
	_, _, err := parseFlags(ModeSync, []string{"--dry-run", "--target", "/tmp/x"})
	if err == nil {
		t.Fatal("expected flag-parse error for removed --dry-run flag; got nil")
	}
	_, _, err = parseFlags(ModeSync, []string{"-d", "--target", "/tmp/x"})
	if err == nil {
		t.Fatal("expected flag-parse error for removed -d shorthand; got nil")
	}
}

func TestParseFlagsAcceptsYes(t *testing.T) {
	t.Parallel()
	cfg, _, err := parseFlags(ModeSync, []string{"--yes", "--target", "/tmp/x"})
	if err != nil {
		t.Fatalf("parseFlags --yes: %v", err)
	}
	if !cfg.AssumeYes {
		t.Errorf("AssumeYes = false; want true after --yes")
	}
}

func TestModeHelpSyncDescribesReviewDoc(t *testing.T) {
	t.Parallel()
	help := ModeHelp(ModeSync)
	if help == "" {
		t.Fatal("ModeHelp returned empty")
	}
	if !strings.Contains(help, ".governa/sync-review.md") {
		t.Errorf("sync help missing review-doc reference: %q", help)
	}
}

// AC79 F-new-3: --yes must appear as its own flag-list row, not only in the
// footer prose. Regex-anchored so the assertion catches footer-only mentions.
func TestModeHelpSyncListsYesFlag(t *testing.T) {
	t.Parallel()
	help := ModeHelp(ModeSync)
	re := regexp.MustCompile(`(?m)^\s+(?:-\w,\s+)?--yes\s{2,}`)
	if !re.MatchString(help) {
		t.Errorf("sync help missing --yes as a flag-list row (regex %q); got:\n%s", re, help)
	}
}

// AC79 F-new-2: --dry-run must NOT appear as a flag-list row (it was retired).
func TestModeHelpSyncOmitsDryRun(t *testing.T) {
	t.Parallel()
	help := ModeHelp(ModeSync)
	if strings.Contains(help, "--dry-run") {
		t.Errorf("sync help still references --dry-run; should be removed per AC79. Got:\n%s", help)
	}
	re := regexp.MustCompile(`(?m)^\s+-d,`)
	if re.MatchString(help) {
		t.Errorf("sync help still references -d shorthand; should be removed. Got:\n%s", help)
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

func TestInferPurposeFromReadme(t *testing.T) {
	t.Parallel()
	dir := newFixtureTarget(t, map[string]string{
		"README.md": "# My Repo\n\nDoes a specific thing for specific users.\n",
	})
	got := inferPurpose(dir)
	want := "Does a specific thing for specific users."
	if got != want {
		t.Errorf("inferPurpose = %q; want %q", got, want)
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

func TestDetectSyncModeNewRepo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if got := detectSyncMode(dir); got != "new" {
		t.Errorf("detectSyncMode on fresh dir = %q; want new", got)
	}
}

func TestDetectSyncModeReSync(t *testing.T) {
	t.Parallel()
	dir := newFixtureTarget(t, map[string]string{
		".governa/manifest": "governa-manifest-v1\ntemplate-version: 0.50.0\n",
	})
	if got := detectSyncMode(dir); got != "re-sync" {
		t.Errorf("detectSyncMode with manifest = %q; want re-sync", got)
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
	// Symbols removed by AC79 Part B (enumerated in AT13's retiredSymbols
	// list). If any return, AT13 fails before tests even compile in most
	// cases.
	// If any of those return, this test file will need references updated
	// before re-passing — serves as a trip-wire, not a live assertion.
}

// AC79 Part A AT1: TEMPLATE_VERSION is always overwritten, even when the
// disk content differs from the current template version.
func TestRunSyncAlwaysWritesTemplateVersion(t *testing.T) {
	dir := t.TempDir()
	// Seed stale TEMPLATE_VERSION + minimal manifest to put target in re-sync state.
	if err := os.WriteFile(filepath.Join(dir, "TEMPLATE_VERSION"), []byte("0.0.1\n"), 0o644); err != nil {
		t.Fatalf("seed TEMPLATE_VERSION: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".governa"), 0o755); err != nil {
		t.Fatalf("mkdir .governa: %v", err)
	}
	manifestContent := "governa-manifest-v1\ntemplate-version: 0.0.1\nrepo-name: x\npurpose: x\ntype: CODE\nstack: Go\n"
	if err := os.WriteFile(filepath.Join(dir, ".governa", "manifest"), []byte(manifestContent), 0o644); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}

	cfg := Config{
		Mode:      ModeSync,
		Target:    dir,
		Type:      RepoTypeCode,
		RepoName:  "x",
		Purpose:   "x",
		Stack:     "Go",
		AssumeYes: false, // deliberately NOT --yes; bookkeeping writes should happen anyway.
	}
	if err := RunWithFS(templates.EmbeddedFS, "", cfg); err != nil && !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("RunWithFS: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "TEMPLATE_VERSION"))
	if err != nil {
		t.Fatalf("read post-sync TEMPLATE_VERSION: %v", err)
	}
	want := templates.TemplateVersion + "\n"
	// Some test setups might not have the trailing newline — accept either.
	if strings.TrimSpace(string(got)) != strings.TrimSpace(want) {
		t.Errorf("TEMPLATE_VERSION = %q; want %q", string(got), want)
	}
}

// AC79 Part A AT2: TEMPLATE_VERSION is never listed as a collision in the
// review doc (bookkeeping writes are exempt from the collision path).
func TestSyncReviewOmitsTemplateVersion(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "TEMPLATE_VERSION"), []byte("0.0.1\n"), 0o644); err != nil {
		t.Fatalf("seed TEMPLATE_VERSION: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".governa"), 0o755); err != nil {
		t.Fatalf("mkdir .governa: %v", err)
	}
	manifestContent := "governa-manifest-v1\ntemplate-version: 0.0.1\nrepo-name: x\npurpose: x\ntype: CODE\nstack: Go\n"
	if err := os.WriteFile(filepath.Join(dir, ".governa", "manifest"), []byte(manifestContent), 0o644); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}

	cfg := Config{Mode: ModeSync, Target: dir, Type: RepoTypeCode, RepoName: "x", Purpose: "x", Stack: "Go"}
	if err := RunWithFS(templates.EmbeddedFS, "", cfg); err != nil && !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("RunWithFS: %v", err)
	}

	review, err := os.ReadFile(filepath.Join(dir, syncReviewFile))
	if err != nil {
		t.Fatalf("read sync-review.md: %v", err)
	}
	if strings.Contains(string(review), "TEMPLATE_VERSION") {
		t.Errorf("sync-review.md must not list TEMPLATE_VERSION (it's bookkeeping); got:\n%s", string(review))
	}
}

// AC79 Part B AT7: review-doc shape — header, per-collision section, diff preview.
func TestRenderSyncReviewShape(t *testing.T) {
	t.Parallel()
	records := []collisionRecord{
		{
			path:     "/tmp/t/AGENTS.md",
			existing: "existing line 1\nexisting line 2\n",
			proposed: "template line 1\ntemplate line 2\n",
		},
	}
	out := renderSyncReview("/tmp/t", "0.49.0", "0.51.0", records)
	mustContain(t, out, "# Governa Sync Review")
	mustContain(t, out, "Template version: 0.49.0 → 0.51.0")
	mustContain(t, out, "1 file(s) need review")
	mustContain(t, out, "### `AGENTS.md`")
	mustContain(t, out, "```diff")
	mustContain(t, out, "-existing line 1")
	mustContain(t, out, "+template line 1")
}

// AC79 Part B AT5: zero-collision case writes a summary-only review-doc.
func TestRenderSyncReviewZeroCollisions(t *testing.T) {
	t.Parallel()
	out := renderSyncReview("/tmp/t", "0.50.0", "0.51.0", nil)
	mustContain(t, out, "# Governa Sync Review")
	mustContain(t, out, "0 files need review")
	if strings.Contains(out, "## Collisions") {
		t.Errorf("zero-collision review-doc must not include a ## Collisions section; got:\n%s", out)
	}
}

// AC79 Part B: diff preview truncates at maxLines with a "more lines" marker.
func TestUnifiedDiffPreviewTruncation(t *testing.T) {
	t.Parallel()
	var eLines []string
	for i := range 100 {
		eLines = append(eLines, fmt.Sprint("existing ", i))
	}
	existing := strings.Join(eLines, "\n") + "\n"
	out := unifiedDiffPreview(existing, "", 10)
	if !strings.Contains(out, "more lines") {
		t.Errorf("expected truncation marker in preview; got:\n%s", out)
	}
	// Count lines — should be ≤ 11 (10 diff lines + 1 truncation marker).
	if got := strings.Count(out, "\n"); got > 11 {
		t.Errorf("preview line count = %d; want ≤ 11", got)
	}
}

// Helper that calls t.Errorf with the full string if assertion fails.
func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("missing substring %q in:\n%s", needle, haystack)
	}
}

// Compile-time probe to ensure fmt is imported where test helpers need it.
var _ = fmt.Sprint
