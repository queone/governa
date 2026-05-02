package driftscan

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// findStagedACs returns the AC file paths under dir/docs/ matching the
// drift-scan staging pattern, excluding the `-diffs.md` sister files
// introduced in AC105 Part B.
func findStagedACs(t *testing.T, dir string) []string {
	t.Helper()
	all, _ := filepath.Glob(filepath.Join(dir, "docs/ac*-drift-scan-from-*.md"))
	var acs []string
	for _, m := range all {
		if strings.HasSuffix(m, "-diffs.md") {
			continue
		}
		acs = append(acs, m)
	}
	return acs
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
	matches := findStagedACs(t, dir)
	if len(matches) != 1 {
		t.Fatalf("expected 1 staged AC, got %d", len(matches))
	}
	acContent := mustRead(t, matches[0])
	for _, want := range []string{
		"# AC1 Drift-Scan from governa @",
		"## Summary\n\n<!-- TBD by Operator -->",
		"## Objective Fit\n\n<!-- TBD by Operator -->",
		"## Status\n\n`PENDING` — awaiting Director critique.",
		"### Post-merge coherence audit\n\n<!-- TBD by Operator -->",
		"Counts: ",
	} {
		if !strings.Contains(acContent, want) {
			t.Errorf("AC content missing %q", want)
		}
	}

	// plan.md modified.
	plan := mustRead(t, filepath.Join(dir, "plan.md"))
	if !strings.Contains(plan, "- IE3: drift-scan against governa @ ") {
		t.Errorf("plan.md missing AC-pointer IE3, got:\n%s", plan)
	}
	// Single-IE contract: no pre-rubric per-ambiguity IE entries are emitted —
	// the AC carries per-file detail under ## Implementation Notes.
	if strings.Contains(plan, "drift-scan ambiguity in ") {
		t.Errorf("plan.md must not carry pre-rubric ambiguity IEs, got:\n%s", plan)
	}
	// AT scope (Part B): tool no longer emits AT-for-IE-pointer or
	// AT-for-preserve-marker — those verified scaffolding placed by earlier
	// ACs / by this scan's staging step. ATs cover only In Scope work.
	if strings.Contains(acContent, "rg -qF -- '- IE") {
		t.Errorf("Part B: AC must not carry AT-for-IE-pointer; got:\n%s", acContent)
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
	matches := findStagedACs(t, dir)
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
	// Overwrite plan.md with an AC-pointer IE pointing at a non-existent AC.
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
// as expected-divergence (not ambiguity / clear-sync, and not the misleading
// `match` label used pre-AC4-fixes).
func TestPlanMdExpectedDivergence(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	out := captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	if !strings.Contains(out, "### `plan.md` — expected-divergence") {
		t.Errorf("expected plan.md classified as expected-divergence, got:\n%s", out)
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

// Counts: "X expected-divergence, Y ambiguity, Z missing-in-target" tally line
// must appear in both the terminal report header and the AC's Implementation Notes.
func TestCountsTallyLine(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}

	out := captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	if !strings.Contains(out, "- Counts: ") {
		t.Errorf("terminal report missing Counts line, got:\n%s", out)
	}

	matches := findStagedACs(t, dir)
	if len(matches) != 1 {
		t.Fatalf("expected 1 staged AC, got %d", len(matches))
	}
	ac := mustRead(t, matches[0])
	if !strings.Contains(ac, "Counts: ") {
		t.Errorf("AC missing Counts line in Implementation Notes, got:\n%s", ac)
	}
	if !strings.Contains(ac, "missing-in-target") {
		t.Errorf("AC Counts line missing missing-in-target, got:\n%s", ac)
	}
}

// Pure-function test for tallyClassifications.
func TestTallyClassifications(t *testing.T) {
	files := []FileResult{
		{Classification: ClassMatch},
		{Classification: ClassMatch},
		{Classification: ClassPreserve},
		{Classification: ClassAmbiguity},
	}
	got := tallyClassifications(files)
	want := "2 match, 1 preserve, 1 ambiguity"
	if got != want {
		t.Errorf("tally got %q, want %q", got, want)
	}
	if tallyClassifications(nil) != "0 files" {
		t.Errorf("empty tally should be %q, got %q", "0 files", tallyClassifications(nil))
	}
}

// Pure-function test for previewCanonContent.
func TestPreviewCanonContent(t *testing.T) {
	short := "a\nb\nc\n"
	if got := previewCanonContent(short, 30); got != "a\nb\nc" {
		t.Errorf("short preview should be returned unchanged, got %q", got)
	}
	long := strings.Repeat("x\n", 50)
	got := previewCanonContent(long, 10)
	if !strings.Contains(got, "[... 40 more lines truncated ...]") {
		t.Errorf("long preview missing truncation marker, got %q", got)
	}
	if got, want := strings.Count(got, "x"), 10; got != want {
		t.Errorf("long preview should have %d kept lines, got %d", want, got)
	}
}

// annotateCommit suffixes adoption-style commits with `(adoption)`.
func TestAnnotateCommit(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"abc1234 AC1: adopt Governa v0.90.0 DOC overlay", "abc1234 AC1: adopt Governa v0.90.0 DOC overlay (adoption)"},
		{"def5678 govern tips repo and add about + new entries", "def5678 govern tips repo and add about + new entries (adoption)"},
		{"ghi9012 AC3: cmd/rel limit 60→80", "ghi9012 AC3: cmd/rel limit 60→80"},
	}
	for _, c := range cases {
		if got := annotateCommit(c.in); got != c.want {
			t.Errorf("annotateCommit(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// Missing-in-target with non-empty canon must route into ## In Scope as
// `create from canon`, AND get a detail subsection with content preview.
func TestMissingInTargetCreateRouting(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}

	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	matches := findStagedACs(t, dir)
	ac := mustRead(t, matches[0])

	// .gitignore is in canon (non-empty) but not in the docFixture target —
	// must be routed into In Scope as a create candidate.
	if !strings.Contains(ac, "- `.gitignore` — create from canon") {
		t.Errorf("expected .gitignore in In Scope as create-from-canon, got:\n%s", ac)
	}
	if !strings.Contains(ac, "### Missing in target (create candidates)") {
		t.Errorf("expected Missing-in-target subsection, got:\n%s", ac)
	}
	if !strings.Contains(ac, "Canon content (preview):") {
		t.Errorf("expected canon content preview block, got:\n%s", ac)
	}
}

// Ambiguities must auto-populate Director Review with one numbered routing
// question per file. No ambiguities → Director Review stays as `None.`.
func TestDirectorReviewAutoPopulate(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}

	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	matches := findStagedACs(t, dir)
	ac := mustRead(t, matches[0])

	// docFixture's AGENTS.md and docs/ac-template.md exist with stub content +
	// one commit → both classify as ambiguity. Plus plan.md is expected-divergence
	// (no Director Review entry). So Director Review should have 2 numbered entries.
	if !strings.Contains(ac, "## Director Review\n\n1. Should `") {
		t.Errorf("expected Director Review to start with numbered routing question, got:\n%s", ac)
	}
	if !strings.Contains(ac, "synced to canon, preserved with a marker") {
		t.Errorf("expected sync/preserve/defer routing prompt, got:\n%s", ac)
	}
}

// AT-B1: Routing summary table is the first sub-subsection of `##
// Implementation Notes` (after the Counts line, asymmetry note, and
// sister-file cross-ref). It must precede `### Match evidence`.
func TestImplementationNotes_RoutingSummaryTableFirst(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	matches := findStagedACs(t, dir)
	ac := mustRead(t, matches[0])

	rsIdx := strings.Index(ac, "### Routing summary")
	meIdx := strings.Index(ac, "### Match evidence")
	if rsIdx < 0 {
		t.Fatalf("expected `### Routing summary` sub-subsection, got:\n%s", ac)
	}
	if meIdx > 0 && meIdx < rsIdx {
		t.Errorf("`### Routing summary` must precede `### Match evidence`; rsIdx=%d meIdx=%d", rsIdx, meIdx)
	}
	// Table header must follow.
	if !strings.Contains(ac[rsIdx:], "| File | Local edit source | What diverged | Operator lean (as of staging) |") {
		t.Errorf("expected routing summary table header, got:\n%s", ac[rsIdx:])
	}
}

// AT-B2: AC per-file blocks under `### Divergent files` carry no `diff -u`
// hunks AND do carry the verbatim commit list. The sister-file cross-ref
// line appears at the top of `## Implementation Notes`.
func TestStagedAC_NoDiffsKeepCommits(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	matches := findStagedACs(t, dir)
	ac := mustRead(t, matches[0])

	// No diff fences in the AC body. (Sister carries diffs.)
	if strings.Contains(ac, "```diff") {
		t.Errorf("AC must not carry diff hunks; got ```diff fence in:\n%s", ac)
	}
	// Commit list line must remain in per-file blocks for at least one
	// ambiguity file (docFixture has multiple ambiguity files with commits).
	if !strings.Contains(ac, "Local commits (`git log -n 5 --follow`):") {
		t.Errorf("AC must carry commit lists in per-file blocks, got:\n%s", ac)
	}
	// Sister-file cross-ref line is in Implementation Notes.
	if !strings.Contains(ac, "Per-file diffs: `docs/ac1-drift-scan-from-abcdef0-diffs.md`") {
		t.Errorf("AC missing sister-file cross-ref line, got:\n%s", ac)
	}
}

// AT-B3: Sister file is staged alongside the AC. Title points back at the
// parent AC; one `## <relpath>` section per divergent file with the
// verbatim diff hunk; matches files have no section in the sister.
func TestStagedSisterFile(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	sisterPath := filepath.Join(dir, "docs", "ac1-drift-scan-from-abcdef0-diffs.md")
	if _, err := os.Stat(sisterPath); err != nil {
		t.Fatalf("sister file not staged at %s: %v", sisterPath, err)
	}
	sister := mustRead(t, sisterPath)
	if !strings.HasPrefix(sister, "# Diffs for AC1 (drift-scan from governa @ abcdef0)") {
		t.Errorf("sister file missing expected title, got:\n%s", sister[:min(len(sister), 200)])
	}
	// Sister must contain at least one ## relpath section + diff fence.
	if !strings.Contains(sister, "## `") {
		t.Errorf("sister file missing per-file section, got:\n%s", sister)
	}
	if !strings.Contains(sister, "```diff") {
		t.Errorf("sister file missing diff fence, got:\n%s", sister)
	}
	// Match-classified files (e.g., AGENTS.md when target is byte-equal)
	// would have no section in sister. docFixture's AGENTS.md is ambiguity,
	// so it should appear. There are no byte-equal files in docFixture, so
	// instead verify sister's section count matches the divergent count.
	// Expect at least one section.
	if strings.Count(sister, "## `") < 1 {
		t.Errorf("sister file should have ≥ 1 per-file section, got:\n%s", sister)
	}
}

// AT-B4: AT generation is scoped to In Scope only. With no In Scope items,
// AT body is the literal `None — ...` line. With In Scope items, ATs cover
// only those — no preserve-marker or IE-pointer ATs.
func TestAcceptanceTests_OnlyForInScope(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	matches := findStagedACs(t, dir)
	ac := mustRead(t, matches[0])

	// docFixture has missing-in-target create candidates (.gitignore etc.),
	// so `## In Scope` is non-empty and AT body has scaffolds.
	if !strings.Contains(ac, "**AT1** [Automated]") {
		t.Errorf("expected AT1 scaffold for In Scope items, got:\n%s", ac)
	}
	// No preserve-marker AT.
	if strings.Contains(ac, "rg -qF -- '|") {
		t.Errorf("AT must not include preserve-marker check; got:\n%s", ac)
	}
	// No IE-pointer AT.
	if strings.Contains(ac, "rg -qF -- '- IE") {
		t.Errorf("AT must not include IE-pointer check; got:\n%s", ac)
	}
}

// AT-B5: Asymmetry note appears in AC's `## Implementation Notes` opening
// and in the console report header.
func TestImplementationNotes_AsymmetryNote(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	out := captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	matches := findStagedACs(t, dir)
	ac := mustRead(t, matches[0])

	// AC105 Part B: same verbatim string in both AC and console.
	const note = "Scan walks canon→target only. Files in target with no canon counterpart surface under `### Files in target without canon` (when present in the other flavor's canon) or via name-reference body scan."
	if !strings.Contains(ac, note) {
		t.Errorf("AC missing asymmetry note, got:\n%s", ac)
	}
	if !strings.Contains(out, note) {
		t.Errorf("console report missing same verbatim asymmetry note, got:\n%s", out)
	}
}

// AC106 Part C: Shell→binary coupling surfaces both files in the
// `Coupled-with:` annotation under each one's per-file block. Q-per-file
// emission means each file gets its own Director Review Q (the prior
// "must route together" framing was dropped per class G).
func TestCoupling_ShellBinary(t *testing.T) {
	dir := docFixture(t)
	mustWrite(t, filepath.Join(dir, "cmd/rel/main.go"), "package main\n// local\n")
	mustWrite(t, filepath.Join(dir, "rel.sh"), "#!/usr/bin/env bash\nset -euo pipefail\nexec go run ./cmd/rel \"$@\"\n")
	gitAddCommit(t, dir, "AC1: rel.sh + cmd/rel divergence")

	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	matches := findStagedACs(t, dir)
	ac := mustRead(t, matches[0])

	// AC106 class G: no "must route together" assertion anywhere in the AC.
	if strings.Contains(ac, "must route together") {
		t.Errorf("class G violation: AC must not contain `must route together`; got:\n%s", ac)
	}
	// AC106 Part B: Q-per-file emission. Each file gets its own Director
	// Review Q. Ambiguity files emit a Q with "Should `<path>` be synced"
	// shape.
	drStart := strings.Index(ac, "## Director Review")
	if drStart < 0 {
		t.Fatalf("no Director Review section, got:\n%s", ac)
	}
	dr := ac[drStart:]
	drEnd := strings.Index(dr, "\n## ")
	if drEnd > 0 {
		dr = dr[:drEnd]
	}
	for _, rel := range []string{"cmd/rel/main.go", "rel.sh"} {
		want := "Should `" + rel + "` be synced"
		if !strings.Contains(dr, want) {
			t.Errorf("Q-per-file: expected per-file Q for %q, got:\n%s", rel, dr)
		}
	}
}

// QA-5: When `## In Scope` is empty (every divergent file is preserve or
// ambiguity, none clear-sync, and no missing-in-target candidates), the AT
// body must be the literal `None — ...` line.
func TestAcceptanceTests_NoneWhenInScopeEmpty(t *testing.T) {
	report := &Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "abcdef0"},
		Files: []FileResult{
			{
				Relpath:        "foo.md",
				Classification: ClassPreserve,
				Markers:        []string{"| 0.1.0 | preserve foo.md customization |"},
				Diff:           "--- a\n+++ b\n",
				Commits:        []string{"abc subject"},
			},
		},
	}
	out := buildACStub(1, "abcdef0", report)
	const want = "None — this AC ships only the staged plan.md IE entry; nothing to verify in target."
	if !strings.Contains(out, want) {
		t.Errorf("expected `None — ...` AT body when In Scope is empty, got:\n%s", out)
	}
	// And no AT scaffolds.
	if strings.Contains(out, "**AT1** [Automated]") {
		t.Errorf("expected no AT scaffolds when In Scope is empty, got:\n%s", out)
	}
}

// AC106 Part B: For divergent files with no coupled siblings under the
// unified rule, the per-file block must render `Coupled-with: None`
// (not omit the line). Replaces the prior `Coupled local-only files`
// rendering — directory-sibling enumeration is no longer used.
func TestPerFileBlock_CoupledWithNoneRendered(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	matches := findStagedACs(t, dir)
	ac := mustRead(t, matches[0])
	if !strings.Contains(ac, "Coupled-with: None") {
		t.Errorf("expected `Coupled-with: None` line for at least one divergent file, got:\n%s", ac)
	}
}

// AC106 Part C: Three .go files in the same package + same directory
// transitively union into a single routing group via the Go same-package
// build-relationship signal. Replaces the prior directory-sibling test —
// directory-sibling enumeration is no longer used as a coupling proxy.
func TestComputeRoutingGroups_MultiFileTransitive(t *testing.T) {
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "cmd", "rel")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"main.go", "color.go", "main_test.go"} {
		if err := os.WriteFile(filepath.Join(pkgDir, f), []byte("package main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files := []FileResult{
		{Relpath: "cmd/rel/main.go", Classification: ClassAmbiguity},
		{Relpath: "cmd/rel/color.go", Classification: ClassAmbiguity},
		{Relpath: "cmd/rel/main_test.go", Classification: ClassAmbiguity},
	}
	groups := computeRoutingGroups(files, dir)
	if len(groups) != 1 {
		t.Fatalf("expected 1 routing group (3 .go files in same package), got %d: %v", len(groups), groups)
	}
	if len(groups[0]) != 3 {
		t.Errorf("expected 3 files in the unioned group, got %d: %v", len(groups[0]), groups[0])
	}
}

// QA-8: When the scan finds zero divergent files, the sister diffs file
// is still staged and carries the "No divergent files." body.
func TestBuildSisterDiffs_NoDivergent(t *testing.T) {
	report := &Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "abcdef0"},
		Files: []FileResult{
			{Relpath: "foo.md", Classification: ClassMatch},
		},
	}
	out := buildSisterDiffs(1, "abcdef0", report)
	if !strings.HasPrefix(out, "# Diffs for AC1 (drift-scan from governa @ abcdef0)") {
		t.Errorf("sister missing title, got:\n%s", out)
	}
	if !strings.Contains(out, "No divergent files.") {
		t.Errorf("expected `No divergent files.` body, got:\n%s", out)
	}
}

// QA-9: Commit subjects containing `|` would otherwise break the routing
// summary markdown table; the cell renderer must escape them.
func TestRoutingSummary_EscapesPipeInCommitSubject(t *testing.T) {
	report := &Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "abcdef0"},
		Files: []FileResult{
			{
				Relpath:        "foo.md",
				Classification: ClassAmbiguity,
				Commits:        []string{"abcdef AC1: tweaked | the | thing"},
			},
		},
	}
	out := buildACStub(1, "abcdef0", report)
	if !strings.Contains(out, `AC1: tweaked \| the \| thing`) {
		t.Errorf("expected `|` escaped as `\\|` in routing summary cell, got:\n%s", out)
	}
}

// AT-C4: Operator-lean placeholder in `## Director Review` uses
// `<!-- TBD by Operator -->`, not the legacy `<TBD>` form, so the convention
// is uniform with the other Operator-fill placeholders.
func TestDirectorReview_LeanPlaceholderMarker(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	matches := findStagedACs(t, dir)
	ac := mustRead(t, matches[0])

	drStart := strings.Index(ac, "## Director Review")
	if drStart < 0 {
		t.Fatalf("no Director Review section, got:\n%s", ac)
	}
	dr := ac[drStart:]
	drEnd := strings.Index(dr, "\n## ")
	if drEnd > 0 {
		dr = dr[:drEnd]
	}
	if !strings.Contains(dr, "Operator lean: <!-- TBD by Operator -->") {
		t.Errorf("expected Operator-lean placeholder `<!-- TBD by Operator -->`, got:\n%s", dr)
	}
	// Legacy `<TBD>` must not appear.
	if strings.Contains(dr, "Operator lean: <TBD>") {
		t.Errorf("legacy `<TBD>` placeholder must be removed; got:\n%s", dr)
	}
}

// canonSHAFromSourceCheckout returns the short SHA of HEAD when the source
// file is in a git worktree. Smoke test — runs against the live repo.
func TestCanonSHAFromSourceCheckout(t *testing.T) {
	sha, err := canonSHAFromSourceCheckout()
	if err != nil {
		t.Fatalf("canonSHAFromSourceCheckout failed in source tree: %v", err)
	}
	if len(sha) != 7 {
		t.Errorf("expected 7-char SHA, got %q", sha)
	}
}

// AC106 Class C: format-defining files in the registry are auto-routed
// to ## In Scope as sync (regardless of raw classification), suppressed
// from Director Review, and named under ### Format-defining file routing
// with rationale.
func TestClassC_FormatDefiningHardRoutes(t *testing.T) {
	dir := docFixture(t)
	// Make docs/ac-template.md divergent (already exists in fixture as
	// a stub; just modify content so canon vs target differ).
	mustWrite(t, filepath.Join(dir, "docs/ac-template.md"), "# AC template\n\nlocal divergence here\n")
	gitAddCommit(t, dir, "AC1: ac-template.md tweak")

	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	matches := findStagedACs(t, dir)
	ac := mustRead(t, matches[0])

	// In Scope must list ac-template.md as sync (with format-defining note).
	if !strings.Contains(ac, "`docs/ac-template.md` — sync to canon (format-defining; hard-routed") {
		t.Errorf("expected docs/ac-template.md hard-routed to In Scope as sync, got:\n%s", ac)
	}
	// ### Format-defining file routing block must name it.
	if !strings.Contains(ac, "### Format-defining file routing") {
		t.Errorf("expected `### Format-defining file routing` sub-subsection, got:\n%s", ac)
	}
	// Director Review must NOT carry a Q for ac-template.md.
	drStart := strings.Index(ac, "## Director Review")
	if drStart < 0 {
		t.Fatalf("no Director Review section, got:\n%s", ac)
	}
	dr := ac[drStart:]
	if strings.Contains(dr, "Should `docs/ac-template.md`") {
		t.Errorf("class C violation: Director Review must not emit a Q for format-defining file; got:\n%s", dr)
	}
}

// AC106 Class E: canon-coherence precondition fires hard on deliberate
// AGENTS.md ↔ ac-template.md drift. Drift-scan exits non-zero, writes a
// structured stdout report (H1: # Canon-Coherence Precondition Failed),
// and stages no files in the target.
func TestClassE_CanonCoherenceHardFail(t *testing.T) {
	// Use the live canon — it's coherent post-AC106 reconciliation.
	// To exercise the hard-fail path, install a coherence rule with a
	// pattern guaranteed not to match.
	saved := coherenceRules
	defer func() { coherenceRules = saved }()
	coherenceRules = []coherenceRule{
		{
			Name:           "Synthetic-test rule",
			AuthorityPath:  "AGENTS.md",
			AuthorityRegex: regexp.MustCompile(`__pattern_that_will_not_match_anything_in_AGENTS_md__`),
		},
	}

	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	out := captureOut(t, func(f *os.File) {
		exit, _ := Run(cfg, EmbeddedFS, f)
		if exit == ExitOK {
			t.Errorf("class E: expected non-zero exit on canon-coherence failure, got ExitOK")
		}
	})
	if !strings.Contains(out, "# Canon-Coherence Precondition Failed") {
		t.Errorf("class E: expected `# Canon-Coherence Precondition Failed` H1 on stdout, got:\n%s", out)
	}
	if !strings.Contains(out, "**governa-side**") {
		t.Errorf("class E: expected governa-side framing in report, got:\n%s", out)
	}
	// No target writes.
	staged := findStagedACs(t, dir)
	if len(staged) > 0 {
		t.Errorf("class E: expected no staged ACs after hard-fail, got %v", staged)
	}
}

// AC106 Class G discipline (descriptive-not-prescriptive coupling
// language) is enforced by negative-matching the regex list below
// across the full staged-AC output. The list is heuristic, not
// exhaustive: extend it whenever a new prescriptive coupling phrasing
// is observed in emission output (during code review of any drift-scan
// emission change, or when a consumer agent surfaces a routing-language
// complaint). An unrevised list ages into a false-pass surface —
// failing tests are the only signal that the discipline holds, so keep
// them honest.
var classGProhibitedPhrases = []string{
	"must route together",
	"route together",
	"route as a group",
	"route as a unit",
	"should be routed",
	"consider as a unit",
}

// AC106 Class G: no prescriptive coupling language survives in the
// staged AC. Tested globally across the full AC body so the discipline
// holds even if a future emission change moves prescriptive language
// into a different section.
func TestClassG_NoPrescriptiveCouplingLanguage(t *testing.T) {
	dir := docFixture(t)
	mustWrite(t, filepath.Join(dir, "cmd/rel/main.go"), "package main\n// local\n")
	mustWrite(t, filepath.Join(dir, "rel.sh"), "#!/usr/bin/env bash\nset -euo pipefail\nexec go run ./cmd/rel \"$@\"\n")
	gitAddCommit(t, dir, "AC1: shell-binary coupling fixture")

	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	matches := findStagedACs(t, dir)
	ac := mustRead(t, matches[0])

	for _, banned := range classGProhibitedPhrases {
		if strings.Contains(ac, banned) {
			t.Errorf("class G violation: AC body contains prescriptive phrase %q. Update emission or add new phrasing to classGProhibitedPhrases.", banned)
		}
	}
	// Coupled sets reading aid heading verbatim with qualifier.
	if !strings.Contains(ac, "### Coupled sets (informational — routing decisions per Q above)") {
		t.Errorf("class G: expected `### Coupled sets (informational — routing decisions per Q above)` heading verbatim, got:\n%s", ac)
	}
}

// AC106 Class I: `target-has-no-canon` files emit a Director Review Q
// with keep/delete/migrate-to-canon options. Closes the decision-surface
// coverage gap — every non-terminal classification must pair with a Q.
func TestClassI_TargetHasNoCanonGetsQ(t *testing.T) {
	// Use a doc-flavor fixture, but place a file that exists only in the
	// CODE overlay's canon — so it's target-has-no-canon for doc flavor.
	dir := docFixture(t)
	mustWrite(t, filepath.Join(dir, "docs/development-cycle.md"), "# Development cycle\n\nlocal version\n")
	gitAddCommit(t, dir, "AC1: development-cycle.md added (only in code overlay)")

	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	matches := findStagedACs(t, dir)
	ac := mustRead(t, matches[0])

	// Director Review must carry a Q for the target-has-no-canon file
	// with keep/delete/migrate-to-canon options.
	drStart := strings.Index(ac, "## Director Review")
	if drStart < 0 {
		t.Fatalf("no Director Review section, got:\n%s", ac)
	}
	dr := ac[drStart:]
	if !strings.Contains(dr, "(target-has-no-canon)") {
		t.Errorf("class I: expected target-has-no-canon Q in Director Review, got:\n%s", dr)
	}
	if !strings.Contains(dr, "kept as a per-repo addition, deleted, or migrated into canon") {
		t.Errorf("class I: expected keep/delete/migrate-to-canon options in Q text, got:\n%s", dr)
	}
}

// AC106 AT4: Routing summary table carries the staging-time stamp
// directly under the heading. Class B discipline against staleness.
func TestRoutingSummary_StagingStamp(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	matches := findStagedACs(t, dir)
	ac := mustRead(t, matches[0])

	const stamp = "_Operator lean below reflects staging-time analysis. Director-resolved routing lives in the Director Review section below; this table does not auto-update on resolution._"
	if !strings.Contains(ac, stamp) {
		t.Errorf("AC106 AT4: expected staging stamp under `### Routing summary`, got:\n%s", ac)
	}
}

// AC106 AT5 (with-content variant): when In Scope has body content AND
// Director Review has open Qs, the header note appears at the top of
// In Scope, before the body items.
func TestInScopeHeaderNote_WithContent(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	matches := findStagedACs(t, dir)
	ac := mustRead(t, matches[0])

	// docFixture has missing-in-target candidates and ambiguity files,
	// so In Scope has body and Director Review has Qs — the header note
	// must appear.
	const noteFragment = "_In Scope expands as Director resolves Q1–Q"
	if !strings.Contains(ac, noteFragment) {
		t.Errorf("AC106 AT5 (with-content): expected In Scope header note when Director Review has open Qs, got:\n%s", ac)
	}
}

// AC106 AT5 (None-replacement variant): when In Scope is otherwise
// `None` and Director Review has open Qs, the header note replaces
// the `None` body.
func TestInScopeHeaderNote_NoneReplacement(t *testing.T) {
	// Construct a report where every divergent file is preserve (no
	// In Scope body) but ambiguity exists in the Q list. We can do this
	// with a hand-built Report passed to buildACStub directly.
	report := &Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "abcdef0"},
		Files: []FileResult{
			{Relpath: "preserved.md", Classification: ClassPreserve, Markers: []string{"| 0.1.0 | preserve preserved.md customization |"}},
			{Relpath: "ambiguity.md", Classification: ClassAmbiguity, Commits: []string{"abc subject"}},
		},
	}
	out := buildACStub(1, "abcdef0", report)

	// In Scope body should be the header note, not "None.".
	const noteFragment = "_In Scope expands as Director resolves Q1–Q"
	if !strings.Contains(out, noteFragment) {
		t.Errorf("AC106 AT5 (None-replacement): expected In Scope header note as body, got:\n%s", out)
	}
	// The literal "None." should not appear as the In Scope body when
	// the header note replaces it.
	inScopeIdx := strings.Index(out, "## In Scope")
	if inScopeIdx < 0 {
		t.Fatalf("missing ## In Scope section, got:\n%s", out)
	}
	tail := out[inScopeIdx:]
	bodyEnd := strings.Index(tail, "## Out Of Scope")
	if bodyEnd < 0 {
		bodyEnd = len(tail)
	}
	body := tail[:bodyEnd]
	if strings.Contains(body, "\nNone.\n") {
		t.Errorf("AC106 AT5 (None-replacement): `None.` must not appear as body when header note replaces it, got body:\n%s", body)
	}
}

// AC106 AT8 / Class H: Resolution protocol section is present in
// docs/drift-scan.md and enumerates sync/preserve/defer transitions.
func TestClassH_ResolutionProtocolDocumented(t *testing.T) {
	data, err := os.ReadFile("../../docs/drift-scan.md")
	if err != nil {
		t.Fatalf("read drift-scan.md: %v", err)
	}
	doc := string(data)
	if !strings.Contains(doc, "## Resolution protocol") {
		t.Error("AC106 AT8: drift-scan.md missing `## Resolution protocol` section")
	}
	for _, transition := range []string{"`sync` resolution", "`preserve` resolution", "`defer` resolution"} {
		if !strings.Contains(doc, transition) {
			t.Errorf("AC106 AT8: drift-scan.md `## Resolution protocol` missing %s transition", transition)
		}
	}
}

// AC106 Class A negative case: heterogeneous content at any depth
// (process docs alongside reference docs under docs/, multiple
// unrelated subcommands under cmd/) does not couple by directory
// alone. Only build-relationship signal (Go same-package) or
// name-reference body scan unions files into a group.
func TestClassA_DepthNHeterogeneousNoFalseCoupling(t *testing.T) {
	dir := t.TempDir()
	// Two unrelated cmd/ subtrees with different packages.
	if err := os.MkdirAll(filepath.Join(dir, "cmd", "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "cmd", "beta"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "cmd/alpha/main.go"), "package alpha\n")
	mustWrite(t, filepath.Join(dir, "cmd/beta/main.go"), "package beta\n")
	// Two unrelated docs/ files (no name-reference between them).
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "docs/process.md"), "# Process\n\nworkflow doc\n")
	mustWrite(t, filepath.Join(dir, "docs/reference.md"), "# Reference\n\nlookup doc\n")

	files := []FileResult{
		{Relpath: "cmd/alpha/main.go", Classification: ClassAmbiguity},
		{Relpath: "cmd/beta/main.go", Classification: ClassAmbiguity},
		{Relpath: "docs/process.md", Classification: ClassAmbiguity},
		{Relpath: "docs/reference.md", Classification: ClassAmbiguity},
	}
	groups := computeRoutingGroups(files, dir)
	// Each file is its own group — no directory-sibling coupling at any depth.
	if len(groups) != 4 {
		t.Errorf("AC106 Class A: heterogeneous depth-N content must not couple via directory; expected 4 groups, got %d: %v", len(groups), groups)
	}
}
