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
	if !strings.Contains(ac[rsIdx:], "| File | Local edit source | What diverged | Recommendation |") {
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
	const note = "Scan walks canon→target only. Files in target with no canon counterpart are not enumerated here except via per-file `Coupled local-only files` sub-bullets."
	if !strings.Contains(ac, note) {
		t.Errorf("AC missing asymmetry note, got:\n%s", ac)
	}
	if !strings.Contains(out, note) {
		t.Errorf("console report missing same verbatim asymmetry note, got:\n%s", out)
	}
}

// AT-A1: Coupled local-only files are listed for divergent files using the
// directory-sibling rule. cmd/rel/helper.go is local-only and must be
// surfaced under cmd/rel/main.go's per-file block.
func TestCoupledLocalOnlyFiles_DirectorySiblings(t *testing.T) {
	dir := docFixture(t)
	mustWrite(t, filepath.Join(dir, "cmd/rel/main.go"), "package main\n// local divergent version\n")
	mustWrite(t, filepath.Join(dir, "cmd/rel/helper.go"), "package main\n// local-only sibling\n")
	gitAddCommit(t, dir, "AC1: cmd/rel divergence")

	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	matches := findStagedACs(t, dir)
	ac := mustRead(t, matches[0])

	if !strings.Contains(ac, "Coupled local-only files: cmd/rel/helper.go") {
		t.Errorf("expected helper.go listed under Coupled local-only files for cmd/rel/main.go, got:\n%s", ac)
	}
}

// AT-A2: Shell→binary coupling groups rel.sh and cmd/rel/main.go into one
// Director Review entry. The single regex over `*.sh` must resolve `go run
// ./cmd/rel` to a divergent file in cmd/rel/.
func TestCoupledLocalOnlyFiles_ShellBinary(t *testing.T) {
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

	// One Director Review line must mention both files together.
	drStart := strings.Index(ac, "## Director Review")
	if drStart < 0 {
		t.Fatalf("no Director Review section, got:\n%s", ac)
	}
	dr := ac[drStart:]
	found := false
	for line := range strings.SplitSeq(dr, "\n") {
		if strings.Contains(line, "`cmd/rel/main.go`") && strings.Contains(line, "`rel.sh`") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected one Director Review line mentioning both cmd/rel/main.go and rel.sh, got:\n%s", dr)
	}
	if !strings.Contains(dr, "(coupled — must route together)") {
		t.Errorf("expected coupled-group framing, got:\n%s", dr)
	}
}

// AT-A3: Coupled files appear in a single Director Review entry, not
// multiple. This guards against accidentally emitting one question per
// coupled file in addition to the grouped question.
func TestDirectorReview_GroupsCoupledFiles(t *testing.T) {
	dir := docFixture(t)
	mustWrite(t, filepath.Join(dir, "cmd/rel/main.go"), "package main\n// local\n")
	mustWrite(t, filepath.Join(dir, "rel.sh"), "#!/usr/bin/env bash\nexec go run ./cmd/rel \"$@\"\n")
	gitAddCommit(t, dir, "AC1: cmd/rel + rel.sh")

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

	// Coupled files must NOT appear as separate single-file entries.
	// A single-file entry has the form "N. Should `<one path>` be synced..."
	// without the "(coupled — must route together)" parenthetical.
	for _, rel := range []string{"cmd/rel/main.go", "rel.sh"} {
		soloPattern := "Should `" + rel + "` be synced"
		if strings.Contains(dr, soloPattern) {
			t.Errorf("coupled file %q must not appear as a separate single-file entry; got:\n%s", rel, dr)
		}
	}
}

// QA-3 + QA-4: enumerateLocalOnlySiblings filters OS/editor noise files
// (.DS_Store, Thumbs.db, *.swp, *~) and symlinks (e.g., CLAUDE.md →
// AGENTS.md). Only real local-only files surface in CoupledLocalOnly.
func TestEnumerateLocalOnlySiblings_FiltersNoiseAndSymlinks(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"real.go",
		".DS_Store",
		"Thumbs.db",
		"main.go.swp",
		"backup~",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("real.go", filepath.Join(dir, "alias.md")); err != nil {
		t.Skipf("symlink unsupported on this filesystem: %v", err)
	}
	canonPaths := map[string]bool{"main.go": true}
	siblings := enumerateLocalOnlySiblings(dir, "main.go", canonPaths)
	if len(siblings) != 1 || siblings[0] != "real.go" {
		t.Errorf("expected [real.go] (noise + symlink filtered), got %v", siblings)
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

// QA-6: For divergent files with no local-only siblings, the per-file
// block must render `Coupled local-only files: None` (not omit the line).
func TestPerFileBlock_CoupledLocalOnlyNoneRendered(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	matches := findStagedACs(t, dir)
	ac := mustRead(t, matches[0])
	// docs/ac-template.md is in target's docs/ alongside only per-AC files
	// (filtered) — so its CoupledLocalOnly is empty and the line must read
	// `None`.
	if !strings.Contains(ac, "Coupled local-only files: None") {
		t.Errorf("expected `Coupled local-only files: None` line for at least one divergent file, got:\n%s", ac)
	}
}

// QA-7: Three or more divergent files unioned via shared coupled-local-only
// siblings collapse into a single routing group.
func TestComputeRoutingGroups_MultiFileTransitive(t *testing.T) {
	dir := t.TempDir()
	files := []FileResult{
		{Relpath: "cmd/a/main.go", Classification: ClassAmbiguity, CoupledLocalOnly: []string{"cmd/a/helper.go"}},
		{Relpath: "cmd/b/main.go", Classification: ClassAmbiguity, CoupledLocalOnly: []string{"cmd/b/helper.go"}},
		// shared.go pulls both packages into one group via overlapping
		// coupled-local-only entries.
		{Relpath: "shared.go", Classification: ClassAmbiguity, CoupledLocalOnly: []string{"cmd/a/helper.go", "cmd/b/helper.go"}},
	}
	groups := computeRoutingGroups(files, dir)
	if len(groups) != 1 {
		t.Fatalf("expected 1 routing group (3 files transitively unioned), got %d: %v", len(groups), groups)
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
