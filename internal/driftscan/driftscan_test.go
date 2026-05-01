package driftscan

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitInit initializes a git repo at dir with one commit so `git log` works.
// Uses a fixed name/email so tests don't depend on user git config.
func gitInit(t *testing.T, dir string) {
	t.Helper()
	for _, cmd := range [][]string{
		{"git", "init", "-q"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "commit.gpgsign", "false"},
	} {
		c := exec.Command(cmd[0], cmd[1:]...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", strings.Join(cmd, " "), err, out)
		}
	}
}

func gitAddCommit(t *testing.T, dir, msg string) {
	t.Helper()
	for _, cmd := range [][]string{
		{"git", "add", "-A"},
		{"git", "commit", "-q", "-m", msg, "--allow-empty"},
	} {
		c := exec.Command(cmd[0], cmd[1:]...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", strings.Join(cmd, " "), err, out)
		}
	}
}

// docFixture creates a doc-flavor target dir with valid scaffolding, an
// existing IE list, and a one-commit history.
func docFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS.md\n")
	mustWrite(t, filepath.Join(dir, "plan.md"), "# Plan\n\n## Ideas To Explore\n\n- IE1: existing\n- IE2: another\n")
	mustWrite(t, filepath.Join(dir, "CHANGELOG.md"), "# Changelog\n\n| Version | Summary |\n|---|---|\n| Unreleased | |\n| 0.1.0 | initial |\n")
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "docs/ac-template.md"), "# AC template\n")
	gitInit(t, dir)
	gitAddCommit(t, dir, "initial")
	return dir
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// AT9 (subset) — Auto-staging when prereqs are present.
func TestStagingHappy(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0", Invocation: "governa drift-scan " + dir}

	out := captureOut(t, func(f *os.File) {
		exit, _ := Run(cfg, EmbeddedFS, f)
		if exit != ExitOK {
			t.Errorf("expected ExitOK, got %d", exit)
		}
	})
	if !strings.Contains(out, "# Drift-Scan Report") {
		t.Errorf("report header missing, got:\n%s", out[:min(len(out), 400)])
	}

	// AC stub written.
	matches, _ := filepath.Glob(filepath.Join(dir, "docs/ac*-drift-scan-from-*.md"))
	if len(matches) != 1 {
		t.Fatalf("expected 1 staged AC, got %d", len(matches))
	}
	acContent := mustRead(t, matches[0])
	for _, want := range []string{
		"# AC1 Drift-Scan from governa @",
		"## Summary\n\n<!-- TBD by Operator -->",
		"## Objective Fit\n\n<!-- TBD by Operator -->",
		"## Director Review\n\nNone.",
		"## Status\n\n`PENDING` — awaiting Director critique.",
		"### Post-merge coherence audit\n\n<!-- TBD by Operator -->",
	} {
		if !strings.Contains(acContent, want) {
			t.Errorf("AC content missing %q", want)
		}
	}

	// plan.md modified.
	plan := mustRead(t, filepath.Join(dir, "plan.md"))
	if !strings.Contains(plan, "- IE3: drift-scan against governa @ ") {
		t.Errorf("plan.md missing shape-(b) IE3, got:\n%s", plan)
	}
	// Single-IE contract: no shape-(a) per-ambiguity IE entries are emitted —
	// the AC carries per-file detail under ## Implementation Notes.
	if strings.Contains(plan, "drift-scan ambiguity in ") {
		t.Errorf("plan.md must not carry shape-(a) ambiguity IEs, got:\n%s", plan)
	}
	// AT count: exactly one IE-grep AT (the shape-(b) one); no per-ambiguity ATs.
	atIECount := strings.Count(acContent, "rg -q '^- IE")
	if atIECount != 1 {
		t.Errorf("expected exactly 1 IE-grep AT in AC, got %d:\n%s", atIECount, acContent)
	}
}

// AT10 — Auto-staging skipped when prereqs are missing.
func TestStagingMissingPrereqs(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS.md\n")
	gitInit(t, dir)
	gitAddCommit(t, dir, "initial")

	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}

	out := captureOut(t, func(f *os.File) {
		exit, _ := Run(cfg, EmbeddedFS, f)
		if exit != ExitEnvError {
			t.Errorf("expected ExitEnvError, got %d", exit)
		}
	})
	if !strings.Contains(out, "staging skipped") {
		t.Errorf("output missing staging-skipped marker:\n%s", out)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "docs/ac*-drift-scan-from-*.md"))
	if len(matches) != 0 {
		t.Errorf("expected no staged AC, got %d", len(matches))
	}
}

// AT11 — Auto-staging refuses to overwrite for current canon SHA.
func TestStagingNoOverwrite(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}

	// First run.
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	// Second run should error.
	out := captureOut(t, func(f *os.File) {
		exit, _ := Run(cfg, EmbeddedFS, f)
		if exit != ExitEnvError {
			t.Errorf("expected ExitEnvError on second run, got %d", exit)
		}
	})
	if !strings.Contains(out, "already exists") && !strings.Contains(out, "prior drift-scan AC") {
		t.Errorf("expected prior-staging error, got:\n%s", out)
	}
}

// AT12 — Fail-safe against governa-self target.
func TestRefuseGovernaSelf(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module github.com/queone/governa\n")
	if err := os.MkdirAll(filepath.Join(dir, "internal", "templates", "base"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Target: dir, DiffLines: 50}

	exit, err := Run(cfg, EmbeddedFS, devNull(t))
	if exit != ExitEnvError {
		t.Errorf("expected ExitEnvError, got %d", exit)
	}
	if err == nil || !strings.Contains(err.Error(), "governa checkout") {
		t.Errorf("expected governa-self error, got: %v", err)
	}
}

// AT14 — Malformed plan.md is rejected.
func TestMalformedPlan(t *testing.T) {
	dir := docFixture(t)
	// Overwrite plan.md with no Ideas To Explore section.
	mustWrite(t, filepath.Join(dir, "plan.md"), "# Plan\n\n## Other\n\nstuff\n")

	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	out := captureOut(t, func(f *os.File) {
		exit, _ := Run(cfg, EmbeddedFS, f)
		if exit != ExitEnvError {
			t.Errorf("expected ExitEnvError, got %d", exit)
		}
	})
	if !strings.Contains(out, "Ideas To Explore") {
		t.Errorf("expected error to mention Ideas To Explore, got:\n%s", out)
	}
}

// AT15 — Orphaned-IE detection.
func TestOrphanedIE(t *testing.T) {
	dir := docFixture(t)
	// Overwrite plan.md with a shape-(b) IE pointing at a non-existent AC.
	mustWrite(t, filepath.Join(dir, "plan.md"),
		"# Plan\n\n## Ideas To Explore\n\n"+
			"- IE1: existing\n"+
			"- IE2: drift-scan against governa @ deadbee → docs/ac99-drift-scan-from-deadbee.md\n",
	)

	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	out := captureOut(t, func(f *os.File) {
		exit, _ := Run(cfg, EmbeddedFS, f)
		if exit != ExitEnvError {
			t.Errorf("expected ExitEnvError, got %d", exit)
		}
	})
	if !strings.Contains(out, "orphan") && !strings.Contains(out, "deleted") {
		t.Errorf("expected orphaned-IE error, got:\n%s", out)
	}
}

// AT18 — `(none active)` plan.md is replaced.
func TestPlanNoneActive(t *testing.T) {
	dir := docFixture(t)
	mustWrite(t, filepath.Join(dir, "plan.md"),
		"# Plan\n\n## Ideas To Explore\n\nIE docstring\n\n(none active)\n",
	)

	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) {
		exit, _ := Run(cfg, EmbeddedFS, f)
		if exit != ExitOK {
			t.Errorf("expected ExitOK, got %d", exit)
		}
	})
	plan := mustRead(t, filepath.Join(dir, "plan.md"))
	if strings.Contains(plan, "(none active)") {
		t.Errorf("expected (none active) replaced, got:\n%s", plan)
	}
	if !strings.Contains(plan, "- IE1: drift-scan against governa @") {
		t.Errorf("expected IE1 inserted, got:\n%s", plan)
	}
}

// AT19 — Repo-name override.
func TestRepoNameResolution(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0", RepoName: "my-override"}

	out := captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	if !strings.Contains(out, "Repo name: my-override") {
		t.Errorf("expected override to apply, got:\n%s", out)
	}
}

// AT4 (subset) — Numbering computation.
func TestNumbering(t *testing.T) {
	dir := docFixture(t)
	// Add a higher-numbered AC file and a commit referencing AC42.
	mustWrite(t, filepath.Join(dir, "docs/ac10-something.md"), "# AC10 something\n")
	gitAddCommit(t, dir, "AC42 prior work")

	n, _ := nextACNumber(dir)
	if n != 43 {
		t.Errorf("expected next AC = 43, got %d", n)
	}
	ie, _ := nextIENumber(dir)
	if ie != 3 {
		t.Errorf("expected next IE = 3, got %d", ie)
	}
}

// AT2 (subset) — Preserve marker detection.
func TestPreserveMarker(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "CHANGELOG.md"),
		"| 0.1.0 | preserve docs/foo.md customization |\n",
	)
	hits := grepPreserveMarkers(dir, "docs/foo.md")
	if len(hits) != 1 {
		t.Errorf("expected 1 hit, got %d: %v", len(hits), hits)
	}
	if !strings.Contains(hits[0], "preserve docs/foo.md customization") {
		t.Errorf("hit content unexpected: %q", hits[0])
	}
}

// Pure-function test for IE insertion (existing IEs).
func TestInsertIEsAfterLast(t *testing.T) {
	plan := "# Plan\n\n## Ideas To Explore\n\n- IE1: a\n- IE2: b\n"
	out, err := insertIEsIntoPlan(plan, []string{"- IE3: x", "- IE4: y"})
	if err != nil {
		t.Fatal(err)
	}
	wantSeq := []string{"- IE1: a\n", "- IE2: b\n- IE3: x\n", "- IE4: y\n"}
	for _, want := range wantSeq {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q, got:\n%s", want, out)
		}
	}
}

// Pure-function test for IE insertion ((none active)).
func TestInsertIEsNoneActive(t *testing.T) {
	plan := "# Plan\n\n## Ideas To Explore\n\n(none active)\n"
	out, err := insertIEsIntoPlan(plan, []string{"- IE1: x"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "(none active)") {
		t.Errorf("expected (none active) replaced, got:\n%s", out)
	}
	if !strings.Contains(out, "- IE1: x") {
		t.Errorf("expected IE1 present, got:\n%s", out)
	}
}

// AT6 — JSON output equivalence (basic structural check).
func TestJSONOutput(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, JSON: true, OverrideSHA: "abcdef0"}
	out := captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	var r Report
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if r.Header.Flavor != "doc" {
		t.Errorf("expected flavor=doc, got %q", r.Header.Flavor)
	}
	if len(r.Files) == 0 {
		t.Errorf("expected files in JSON, got none")
	}
}

// Helper: capture stdout of a function that takes *os.File.
func captureOut(t *testing.T, fn func(*os.File)) string {
	t.Helper()
	tmp, err := os.CreateTemp("", "drift-out-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	fn(tmp)
	tmp.Close()
	data, err := os.ReadFile(tmp.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func mustRead(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func devNull(t *testing.T) *os.File {
	t.Helper()
	f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

// T5 — C1 regression: target without .git/ must error, not silently
// classify divergent files as clear-sync.
func TestNoGitWorktree(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS.md\n")
	mustWrite(t, filepath.Join(dir, "plan.md"), "# Plan\n\n## Ideas To Explore\n\n- IE1: x\n")
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	// No gitInit — target has no .git/.

	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	exit, err := Run(cfg, EmbeddedFS, devNull(t))
	if exit != ExitEnvError {
		t.Errorf("expected ExitEnvError, got %d", exit)
	}
	if err == nil || !strings.Contains(err.Error(), "not a git worktree") {
		t.Errorf("expected git-worktree error, got: %v", err)
	}
}

// T6 — C2 regression: prose containing the preserve phrase mid-sentence
// must NOT be classified as a preserve marker.
func TestPreserveMarkerNotInProse(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "CHANGELOG.md"),
		"In some future state we should preserve docs/foo.md customization where appropriate.\n",
	)
	hits := grepPreserveMarkers(dir, "docs/foo.md")
	if len(hits) != 0 {
		t.Errorf("expected 0 hits for prose, got %d: %v", len(hits), hits)
	}

	// Sanity: the same phrase as a CHANGELOG table row IS detected.
	mustWrite(t, filepath.Join(dir, "CHANGELOG.md"),
		"| 0.1.0 | preserve docs/foo.md customization |\n",
	)
	hits = grepPreserveMarkers(dir, "docs/foo.md")
	if len(hits) != 1 {
		t.Errorf("expected 1 hit for table row, got %d: %v", len(hits), hits)
	}

	// Sanity: as a CHANGELOG cell separator (`; preserve <path>`).
	mustWrite(t, filepath.Join(dir, "CHANGELOG.md"),
		"| 0.1.0 | AC1: did stuff; preserve docs/foo.md customization |\n",
	)
	hits = grepPreserveMarkers(dir, "docs/foo.md")
	if len(hits) != 1 {
		t.Errorf("expected 1 hit for cell separator, got %d: %v", len(hits), hits)
	}
}

// T7 — H1 regression: non-existent target must error, not produce a
// successful-looking report with all-missing classifications.
func TestNonexistentTarget(t *testing.T) {
	cfg := Config{Target: "/no/such/dir", Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	exit, err := Run(cfg, EmbeddedFS, devNull(t))
	if exit != ExitEnvError {
		t.Errorf("expected ExitEnvError, got %d", exit)
	}
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected does-not-exist error, got: %v", err)
	}
}

// H2 regression: plan.md content divergence is expected and must classify
// as match (not ambiguity / clear-sync).
func TestPlanMdAlwaysMatches(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	out := captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	// plan.md should appear with classification `match` (per-repo content).
	if !strings.Contains(out, "### `plan.md` — match") {
		t.Errorf("expected plan.md classified as match, got:\n%s", out)
	}
}

// Sanity check that the report is valid markdown headed by the right title.
func TestReportShape(t *testing.T) {
	r := Report{
		Header: ReportHeader{Invocation: "test", CanonSHA: "abcdef0", Target: "/tmp/x", Flavor: "doc", RepoName: "x"},
		NextAC: 4,
		NextIE: 9,
	}
	var buf bytes.Buffer
	tmp, err := os.CreateTemp("", "ds-*.md")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	writeReport(tmp, r, false)
	tmp.Seek(0, 0)
	data, _ := os.ReadFile(tmp.Name())
	buf.Write(data)
	if !strings.HasPrefix(buf.String(), "# Drift-Scan Report") {
		t.Errorf("expected report header, got: %s", buf.String()[:50])
	}
}
