package driftscan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// sectionBody returns the body of the named `## <name>` section in the AC,
// anchored on heading-at-start-of-line. Necessary because rationale paragraphs
// (e.g. format-defining-routing, missing-in-target-routing) carry literal
// substrings like "`## Director Review`" or "`## In Scope`" as backticked
// section references; bare strings.Index would match inside those paragraphs.
// Returns ("", false) when the section is not found.
func sectionBody(body, name string) (string, bool) {
	heading := "\n## " + name + "\n"
	idx := strings.Index(body, heading)
	if idx < 0 {
		return "", false
	}
	tail := body[idx+1:]
	endIdx := strings.Index(tail[len("## "+name)+1:], "\n## ")
	if endIdx < 0 {
		return tail, true
	}
	return tail[:len("## "+name)+1+endIdx], true
}

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
		// AC114 Part A: Objective Fit pre-fills with target's local form
		// (canon-3-part fallback when target ac-template missing). For docFixture
		// the local ac-template is empty, so fallback fires.
		"## Objective Fit\n\n1. **Outcome** <!-- TBD by Operator -->",
		"## Status\n\n`PENDING` — awaiting Director critique.",
		// AC114 Parts B+C: post-merge audit pre-fills based on sync/preserve state.
		// docFixture has 1 missing-in-target (.gitignore) ambiguity files but no
		// preserve markers → vacuous preserve-empty body fires.
		"### Post-merge coherence audit",
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

	// AC111 Class X: Director Review opens with the bulleted routing-menu
	// block (when at least one Q exists), then numbered per-Q entries
	// leading with the file in backticks.
	dr, ok := sectionBody(ac, "Director Review")
	if !ok {
		t.Fatalf("missing Director Review section, got:\n%s", ac)
	}
	if !strings.Contains(dr, "**Routing menu** (pick one per Q):") {
		t.Errorf("expected bulleted routing-menu block at start of Director Review, got:\n%s", dr)
	}
	if !strings.Contains(dr, "1. **`") {
		t.Errorf("expected first per-Q entry to lead with `1. **`<file>`**`, got:\n%s", dr)
	}
}

// AC112 Class Y (formerly AT-B1, inverted) — Routing summary table is no
// longer emitted. `What diverged` lives in per-file Divergent files blocks;
// leans live only in Director Review per-Q. Single source of truth.
func TestClassY_NoRoutingSummaryTable(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	matches := findStagedACs(t, dir)
	ac := mustRead(t, matches[0])

	if strings.Contains(ac, "### Routing summary") {
		t.Errorf("AC112 Class Y: `### Routing summary` heading must not appear (table dropped), got:\n%s", ac)
	}
	if strings.Contains(ac, "| File | What diverged | Operator lean (as of staging) |") {
		t.Errorf("AC112 Class Y: legacy 3-column routing-summary header must not appear, got:\n%s", ac)
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
	// Anchor on the heading at start of line — the format-defining-routing
	// rationale paragraph contains the literal substring "## Director Review"
	// (as a backticked section reference), so a bare strings.Index would
	// match inside that paragraph instead of the heading.
	drStart := strings.Index(ac, "\n## Director Review\n")
	if drStart < 0 {
		t.Fatalf("no Director Review section, got:\n%s", ac)
	}
	dr := ac[drStart:]
	drEnd := strings.Index(dr[1:], "\n## ")
	if drEnd > 0 {
		dr = dr[:drEnd+1]
	}
	// AC109 Class V: per-Q text leads with the file in backticks instead
	// of the "Should `<file>` be synced" boilerplate. Coupling is purely
	// informational via `### Coupled sets`.
	for _, rel := range []string{"cmd/rel/main.go", "rel.sh"} {
		want := "**`" + rel + "`**"
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

// AC108 Class R: For divergent files with no coupled siblings, the per-file
// block must NOT emit any Coupled-with line (silence is clearer than
// negative-state noise). Reverses AC106's `Coupled-with: None` requirement.
func TestPerFileBlock_NoCoupledWithLineWhenUncoupled(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	matches := findStagedACs(t, dir)
	ac := mustRead(t, matches[0])
	if strings.Contains(ac, "Coupled-with: None") {
		t.Errorf("AC108 Class R: legacy `Coupled-with: None` line must NOT appear; uncoupled files emit no Coupled-with line, got:\n%s", ac)
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

// QA-9 retired by AC108 Class S: the routing summary table no longer
// carries the Local edit source column, so commit-subject pipe escaping
// is no longer relevant. Per-file blocks emit commit subjects in markdown
// list items where `|` does not break formatting. AC108 AT9 asserts the
// table is 3 columns (no Local edit source).

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

	dr, ok := sectionBody(ac, "Director Review")
	if !ok {
		t.Fatalf("no Director Review section, got:\n%s", ac)
	}
	// AC109 Class V: per-Q lead is `<N>. **`<file>`** — <placeholder>. Why: <placeholder>.`
	// — the literal `<!-- TBD by Operator -->` placeholder still anchors the
	// lean position; the `Operator lean:` label was dropped along with the
	// "Should ... synced/preserved/deferred" boilerplate.
	if !strings.Contains(dr, "** — <!-- TBD by Operator -->. Why: <!-- TBD by Operator -->.") {
		t.Errorf("expected per-Q placeholder shape `** — <!-- TBD by Operator -->. Why: <!-- TBD by Operator -->.`, got:\n%s", dr)
	}
	// Legacy `<TBD>` must not appear.
	if strings.Contains(dr, "<TBD>") {
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
	dr, ok := sectionBody(ac, "Director Review")
	if !ok {
		t.Fatalf("no Director Review section, got:\n%s", ac)
	}
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

	// Director Review must carry a Q for the target-has-no-canon file with
	// the `(target-has-no-canon)` annotation. AC109 Class V moved the
	// keep/delete/migrate-to-canon menu wording to the section's routing-menu
	// stamp; per-Q text only carries the file + `(target-has-no-canon)` +
	// placeholders.
	dr, ok := sectionBody(ac, "Director Review")
	if !ok {
		t.Fatalf("no Director Review section, got:\n%s", ac)
	}
	if !strings.Contains(dr, "(target-has-no-canon)") {
		t.Errorf("class I: expected target-has-no-canon Q in Director Review, got:\n%s", dr)
	}
	// AC111 Class X: keep/delete menu lives in the bulleted routing-menu
	// block at the top of Director Review (AC116 Q3 dropped migrate-to-canon).
	if !strings.Contains(dr, "`keep` / `delete`") {
		t.Errorf("class I: expected keep/delete menu in bulleted routing-menu block, got:\n%s", dr)
	}
}

// AC106 AT4: Routing summary table carries the staging-time stamp
// directly under the heading. Class B discipline against staleness.
// AC112 Class Y (formerly AC111 inverse, made vacuous by table removal) —
// the routing-summary table is gone entirely; legacy staging stamp absent
// is now trivially true. Test retired.

// AC111 Class X (formerly AC106 AT5 with-content, inverted) — In Scope no
// longer carries the expansion header note. Body is the routed lines directly.
func TestClassX_InScopeNoHeaderNote_WithContent(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	matches := findStagedACs(t, dir)
	ac := mustRead(t, matches[0])

	const legacyNote = "_In Scope expands as Director resolves Q1–Q"
	if strings.Contains(ac, legacyNote) {
		t.Errorf("AC111: In Scope must not carry the legacy expansion header note, got:\n%s", ac)
	}
}

// AC111 Class X (replaces AC106 AT5 None-replacement variant) — when
// In Scope is otherwise empty AND Director Review has open Qs, body is
// the terse one-liner `None — body lands as Director resolves Q1–Q<N>.`
func TestClassX_InScopeEmptyWithOpenQsTerseOneLiner(t *testing.T) {
	report := &Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "abcdef0"},
		Files: []FileResult{
			{Relpath: "preserved.md", Classification: ClassPreserve, Markers: []string{"| 0.1.0 | preserve preserved.md customization |"}},
			{Relpath: "ambiguity.md", Classification: ClassAmbiguity, Commits: []string{"abc subject"}},
		},
	}
	out := buildACStub(1, "abcdef0", report)
	const want = "None — body lands as Director resolves Q1–Q1."
	if !strings.Contains(out, want) {
		t.Errorf("AC111: expected terse one-liner %q in In Scope when empty + open Qs, got:\n%s", want, out)
	}
	// Legacy header note must NOT appear.
	const legacyNote = "_In Scope expands as Director resolves Q1–Q"
	if strings.Contains(out, legacyNote) {
		t.Errorf("AC111: legacy In Scope expansion header note must not appear, got:\n%s", out)
	}
}

// AC106 AT8 / Class H (updated AC116 Part D): Resolution protocol section is
// present in docs/drift-scan.md and documents the per-decision menus.
// Structural assertion per AC115's lesson — verifies presence of decision
// tokens without pinning verbatim wording shape.
func TestClassH_ResolutionProtocolDocumented(t *testing.T) {
	data, err := os.ReadFile("../../docs/drift-scan.md")
	if err != nil {
		t.Fatalf("read drift-scan.md: %v", err)
	}
	doc := string(data)
	rpBody, ok := sectionBody(doc, "Resolution protocol")
	if !ok {
		t.Fatal("AC106 AT8: drift-scan.md missing `## Resolution protocol` section")
	}
	for _, decision := range []string{"`sync`", "`preserve`", "`defer`", "`keep`", "`delete`"} {
		if !strings.Contains(rpBody, decision) {
			t.Errorf("`## Resolution protocol` missing %s decision", decision)
		}
	}
	// Per AC116 Part D: imperative + named failure mode (AC115 pattern).
	if !strings.Contains(rpBody, "MUST") {
		t.Error("`## Resolution protocol` missing imperative MUST token")
	}
	if !strings.Contains(rpBody, "**Failure mode:**") {
		t.Error("`## Resolution protocol` missing named failure mode")
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

// AC107 AT1 — qualifyGovernaPath helper returns the form `governa @ <sha>: <path>`.
func TestClassJ_QualifyGovernaPathHelper(t *testing.T) {
	got := qualifyGovernaPath("abcdef0", "docs/drift-scan.md")
	const want = "governa @ abcdef0: docs/drift-scan.md"
	if got != want {
		t.Errorf("qualifyGovernaPath: got %q, want %q", got, want)
	}
}

// stagedACWithOpenQs builds a synthetic Report that produces a staged AC
// hitting all five qualified-form emission sites: In Scope header note,
// In Scope format-defining sync line, `### Format-defining file routing`
// rationale, Out Of Scope header note, AT header note. Includes a
// format-defining-divergent file (docs/ac-template.md is in the registry)
// plus an ambiguity (so Director Review has open Qs and all header notes
// fire) plus clear-sync and preserve files for AT and Out Of Scope body.
func stagedACWithOpenQs(t *testing.T) string {
	t.Helper()
	report := &Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "abcdef0"},
		Files: []FileResult{
			{Relpath: "preserved.md", Classification: ClassPreserve, Markers: []string{"| 0.1.0 | preserve preserved.md customization |"}},
			{Relpath: "ambiguity.md", Classification: ClassAmbiguity, Commits: []string{"abc subject"}},
			{Relpath: "clear.md", Classification: ClassClearSync, CanonContent: "# clear\n"},
			{Relpath: "docs/ac-template.md", Classification: ClassAmbiguity, Commits: []string{"def subject"}, CanonContent: "# AC template\n"},
		},
	}
	return buildACStub(1, "abcdef0", report)
}

// stagedACNoOpenQs builds a synthetic Report whose staged AC has no open
// Director Review Qs (no ambiguities, no target-has-no-canon files).
// Used to assert the inverse: header notes are NOT emitted.
func stagedACNoOpenQs(t *testing.T) string {
	t.Helper()
	report := &Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "abcdef0"},
		Files: []FileResult{
			{Relpath: "clear.md", Classification: ClassClearSync, CanonContent: "# clear\n"},
		},
	}
	return buildACStub(1, "abcdef0", report)
}

// AC107 AT2 — registry-driven: every backticked governa-only path token in
// the staged body is preceded by `governa @ <sha>: ` in the same backtick
// span. Adding a new prefix to governaOnlyPathPrefixes extends coverage.
func TestClassJ_NoBareGovernaOnlyPathInStagedBody(t *testing.T) {
	body := stagedACWithOpenQs(t)
	// Tokenize backticked spans, then for each whitespace-split token
	// inside, check whether it has a governa-only prefix and whether it is
	// qualified (preceded somewhere in the same span by `governa @ `).
	spanRe := regexp.MustCompile("`([^`]+)`")
	for _, m := range spanRe.FindAllStringSubmatch(body, -1) {
		span := m[1]
		qualified := strings.Contains(span, "governa @ ")
		for tok := range strings.FieldsSeq(span) {
			tok = strings.TrimRight(tok, ".,;:)")
			if !isGovernaOnlyPath(tok) {
				continue
			}
			if !qualified {
				t.Errorf("AC107 AT2: bare governa-only path %q in span `%s` — must be qualified via qualifyGovernaPath()", tok, span)
			}
		}
	}
}

// AC107 AT2 (positive sub-assertion) — qualified form appears at the
// post-AC111 emission sites: format-defining sync line, format-defining
// rationale paragraph, Director Review menu's target-has-no-canon bullet.
// (Pre-AC111 also emitted at In Scope / Out Of Scope / AT header notes;
// AC111 removed those stamps.)
func TestClassJ_QualifiedFormAtKnownSites(t *testing.T) {
	body := stagedACWithOpenQs(t)
	const qualified = "`governa @ abcdef0: docs/drift-scan.md "
	count := strings.Count(body, qualified)
	if count < 3 {
		t.Errorf("AC107 AT2 positive (post-AC111): expected ≥3 occurrences of qualified `governa @ <sha>: docs/drift-scan.md ...`, got %d:\n%s", count, body)
	}
}

// AC111 Class X (formerly AC107 AT3, inverted) — Out Of Scope section must
// NOT carry the AC107-era defer-resolution header note. The recap stamp was
// removed; the routing-menu lives only in Director Review.
func TestClassX_OutOfScopeNoHeaderNote(t *testing.T) {
	body := stagedACWithOpenQs(t)
	const note = "_Defer resolutions add a bullet here"
	if strings.Contains(body, note) {
		t.Errorf("AC111: Out Of Scope must NOT carry the legacy defer-resolution header note, got:\n%s", body)
	}
}

// AC111 Class X (formerly AC107 AT4, inverted) — AT section must NOT carry
// the AC107-era sync-resolution header note.
func TestClassX_AcceptanceTestsNoHeaderNote(t *testing.T) {
	body := stagedACWithOpenQs(t)
	const note = "_Each sync resolution adds a paired byte-equality AT here"
	if strings.Contains(body, note) {
		t.Errorf("AC111: Acceptance Tests must NOT carry the legacy sync-resolution header note, got:\n%s", body)
	}
}

// AC111 Class X (formerly AC107 AT5, simplified) — header notes are gone in
// every state, not just when Director Review is None. The inverse-of-OpenQs
// fixture still passes because the same negative assertions hold.
func TestClassX_NoHeaderNotesNoOpenQsEither(t *testing.T) {
	body := stagedACNoOpenQs(t)
	const ooNote = "_Defer resolutions add a bullet here"
	const atNote = "_Each sync resolution adds a paired byte-equality AT here"
	if strings.Contains(body, ooNote) {
		t.Errorf("AC111: Out Of Scope must not carry legacy header note, got:\n%s", body)
	}
	if strings.Contains(body, atNote) {
		t.Errorf("AC111: AT section must not carry legacy header note, got:\n%s", body)
	}
}

// AC107 AT6 — docs/drift-scan.md carries `## Reference qualification` section.
func TestClassJ_ReferenceQualificationDocumented(t *testing.T) {
	data, err := os.ReadFile("../../docs/drift-scan.md")
	if err != nil {
		t.Fatalf("read drift-scan.md: %v", err)
	}
	doc := string(data)
	if !strings.Contains(doc, "## Reference qualification") {
		t.Error("AC107 AT6: drift-scan.md missing `## Reference qualification` section")
	}
	if !strings.Contains(doc, "governa @ <sha>: <path>") {
		t.Error("AC107 AT6: drift-scan.md `## Reference qualification` missing rule statement")
	}
}

// AC111 Class X (formerly AC107 AT7, inverted) — drift-scan.md no longer
// documents the AC107-era header-note descriptions; the recap stamps were
// removed and their descriptions stripped from `## What the tool emits`.
func TestClassX_NewHeaderNotesNotDocumented(t *testing.T) {
	data, err := os.ReadFile("../../docs/drift-scan.md")
	if err != nil {
		t.Fatalf("read drift-scan.md: %v", err)
	}
	doc := string(data)
	if strings.Contains(doc, "Defer resolutions add a bullet here") {
		t.Error("AC111: drift-scan.md must not retain the legacy defer-resolution header-note description")
	}
	if strings.Contains(doc, "Each sync resolution adds a paired byte-equality AT here") {
		t.Error("AC111: drift-scan.md must not retain the legacy sync-resolution header-note description")
	}
}

// AC111 Class X (formerly AC107 AT8, inverted) — drift-scan.md no longer
// carries the `## Scaffold emission policy` section. The umbrella rationale
// was tied to the now-removed stamps; the no-empty-`###` rule survives as
// test-internal discipline (TestClassM_NoEmptySubSections still passes).
func TestClassX_NoScaffoldEmissionPolicySection(t *testing.T) {
	data, err := os.ReadFile("../../docs/drift-scan.md")
	if err != nil {
		t.Fatalf("read drift-scan.md: %v", err)
	}
	doc := string(data)
	if strings.Contains(doc, "## Scaffold emission policy") {
		t.Error("AC111: drift-scan.md must not retain `## Scaffold emission policy` section")
	}
}

// AC107 AT9 (Class M) — staged AC body contains no empty sub-section: any
// `### <X>` heading is followed by at least one non-blank, non-`#` body
// line before the next `## ` or `### ` heading.
func TestClassM_NoEmptySubSections(t *testing.T) {
	bodies := []string{
		stagedACWithOpenQs(t),
		stagedACNoOpenQs(t),
	}
	for _, body := range bodies {
		lines := strings.Split(body, "\n")
		for i, line := range lines {
			if !strings.HasPrefix(line, "### ") {
				continue
			}
			heading := line
			hasBody := false
			for j := i + 1; j < len(lines); j++ {
				next := lines[j]
				if strings.HasPrefix(next, "## ") || strings.HasPrefix(next, "### ") {
					break
				}
				trimmed := strings.TrimSpace(next)
				if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
					hasBody = true
					break
				}
			}
			if !hasBody {
				t.Errorf("AC107 AT9: empty sub-section %q (no body before next heading) in staged AC body:\n%s", heading, body)
			}
		}
	}
}

// stagedACAC108Fixture builds a synthetic Report exercising AC108's classes:
// at least one divergent file with non-trivial diff (Class U Direction line),
// at least one missing-in-target with non-empty canon (Class Q routing block
// + counts annotation), at least one format-defining ambiguity (Class T
// counts annotation + Class P AGENTS.md verification), and clear-sync /
// preserve / ambiguity files for general per-file emission.
func stagedACAC108Fixture(t *testing.T) string {
	t.Helper()
	report := &Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "abcdef0"},
		Files: []FileResult{
			{Relpath: "preserved.md", Classification: ClassPreserve, Markers: []string{"| 0.1.0 | preserve preserved.md customization |"}, Diff: "--- canon/preserved.md\n+++ target/preserved.md\n@@ -1,2 +1,3 @@\n line\n+target-only\n-canon-only\n"},
			{Relpath: "ambiguity.md", Classification: ClassAmbiguity, Commits: []string{"abc subject"}, Diff: "--- canon/ambiguity.md\n+++ target/ambiguity.md\n@@ -1,1 +1,3 @@\n+target-add-1\n+target-add-2\n line\n"},
			{Relpath: "clear.md", Classification: ClassClearSync, CanonContent: "# clear\n", Diff: "--- canon/clear.md\n+++ target/clear.md\n@@ -1,1 +1,1 @@\n-canon-line\n+target-line\n"},
			{Relpath: "AGENTS.md", Classification: ClassAmbiguity, Commits: []string{"def subject"}, CanonContent: "# AGENTS\n", Diff: "--- canon/AGENTS.md\n+++ target/AGENTS.md\n@@ -1,1 +1,1 @@\n-canon-agents\n+target-agents\n"},
			{Relpath: "newfile.md", Classification: ClassMissingTarget, CanonContent: "# new\n"},
		},
	}
	return buildACStub(1, "abcdef0", report)
}

// AC108 AT1 — each per-file block under `### Divergent files` carries a
// `Direction:` line. Missing-in-target blocks (under `### Missing in target`)
// are out of scope — they have no canon-vs-target diff.
func TestClassU_DirectionLineEmitted(t *testing.T) {
	body := stagedACAC108Fixture(t)
	notes, ok := sectionBody(body, "Implementation Notes")
	if !ok {
		t.Fatalf("missing ## Implementation Notes")
	}
	dvIdx := strings.Index(notes, "### Divergent files")
	if dvIdx < 0 {
		t.Fatalf("missing ### Divergent files in Implementation Notes:\n%s", notes)
	}
	dvSection := notes[dvIdx:]
	// Truncate at the next ### sub-subsection boundary so we only inspect
	// per-file blocks under Divergent files.
	if endIdx := strings.Index(dvSection[1:], "\n### "); endIdx > 0 {
		dvSection = dvSection[:endIdx+1]
	}
	blocks := strings.Split(dvSection, "#### `")
	for _, blk := range blocks[1:] {
		if !strings.Contains(blk, "Direction:") {
			t.Errorf("AC108 AT1: per-file block under Divergent files missing Direction line:\n%s", blk)
		}
	}
}

// AC108 AT2 — Direction labels target-leads / canon-leads / explicit-N/M
// correctly based on diff content.
func TestClassU_DirectionLabels(t *testing.T) {
	cases := []struct {
		name           string
		diff           string
		wantSubstrings []string
	}{
		{"target-leads", "--- canon/x\n+++ target/x\n@@ -0,0 +1,2 @@\n+a\n+b\n", []string{"target leads", "target carries 2 lines absent in canon"}},
		{"canon-leads", "--- canon/x\n+++ target/x\n@@ -1,2 +0,0 @@\n-a\n-b\n", []string{"canon leads", "canon carries 2 lines absent in target"}},
		{"mutual", "--- canon/x\n+++ target/x\n@@ -1,2 +1,2 @@\n-a\n+b\n-c\n+d\n", []string{"target carries 2 lines absent in canon", "canon carries 2 lines absent in target"}},
		{"empty", "", []string{"no line-level divergence detected"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, t2 := computeDirection(tc.diff)
			got := formatDirection(c, t2)
			for _, want := range tc.wantSubstrings {
				if !strings.Contains(got, want) {
					t.Errorf("AC108 AT2 %s: expected %q in %q", tc.name, want, got)
				}
			}
		})
	}
}

// AC108 AT3 — Sister diffs file body opens with the convention stamp.
func TestClassU_SisterConventionStamp(t *testing.T) {
	report := &Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "abcdef0"},
		Files: []FileResult{
			{Relpath: "x.md", Classification: ClassAmbiguity, Diff: "--- canon/x\n+++ target/x\n@@ -1 +1 @@\n-a\n+b\n"},
		},
	}
	out := buildSisterDiffs(1, "abcdef0", report)
	const stamp = "_Diff convention: `+` lines exist in TARGET; `-` lines exist in CANON."
	if !strings.Contains(out, stamp) {
		t.Errorf("AC108 AT3: sister diffs file missing convention stamp, got:\n%s", out)
	}
	// Stamp should appear before the first per-file ## section.
	stampIdx := strings.Index(out, stamp)
	firstSection := strings.Index(out, "## `")
	if firstSection > 0 && stampIdx > firstSection {
		t.Errorf("AC108 AT3: stamp must appear before first per-file section; stampIdx=%d firstSection=%d", stampIdx, firstSection)
	}
}

// AC108 AT4 — formatDefiningCanonPaths registry contains AGENTS.md.
func TestClassP_AGENTSMdInRegistry(t *testing.T) {
	if !formatDefiningCanonPaths["AGENTS.md"] {
		t.Error("AC108 AT4: formatDefiningCanonPaths must contain AGENTS.md after Class P registry broadening")
	}
}

// AC108 AT5 — When AGENTS.md is divergent, `### Format-defining file routing`
// block lists it.
func TestClassP_AGENTSMdHardRouted(t *testing.T) {
	body := stagedACAC108Fixture(t)
	notes, ok := sectionBody(body, "Implementation Notes")
	if !ok {
		t.Fatalf("missing ## Implementation Notes")
	}
	if !strings.Contains(notes, "### Format-defining file routing") {
		t.Fatalf("missing ### Format-defining file routing block")
	}
	// AGENTS.md should appear in the format-defining-routing block as
	// raw classification ambiguity, auto-routed to In Scope as sync.
	fdrIdx := strings.Index(notes, "### Format-defining file routing")
	tail := notes[fdrIdx:]
	if endIdx := strings.Index(tail[1:], "### "); endIdx > 0 {
		tail = tail[:endIdx+1]
	}
	if !strings.Contains(tail, "`AGENTS.md`") {
		t.Errorf("AC108 AT5: ### Format-defining file routing must list AGENTS.md when divergent, got:\n%s", tail)
	}
}

// AC108 AT6 — When at least one missing-in-target with non-empty canon is
// auto-routed, `### Missing-in-target file routing` is emitted with the
// AGENTS.md Approval Boundaries citation.
func TestClassQ_MissingInTargetRoutingBlock(t *testing.T) {
	body := stagedACAC108Fixture(t)
	notes, ok := sectionBody(body, "Implementation Notes")
	if !ok {
		t.Fatalf("missing ## Implementation Notes")
	}
	if !strings.Contains(notes, "### Missing-in-target file routing") {
		t.Errorf("AC108 AT6: ### Missing-in-target file routing must be emitted when missing-in-target with non-empty canon exists, got:\n%s", notes)
	}
	if !strings.Contains(notes, "AGENTS.md Approval Boundaries") {
		t.Errorf("AC108 AT6: missing-in-target routing block must cite AGENTS.md Approval Boundaries, got:\n%s", notes)
	}
}

// AC108 AT7 — When no missing-in-target with non-empty canon exists, the
// `### Missing-in-target file routing` block is NOT emitted.
func TestClassQ_NoMissingInTargetRoutingWhenNone(t *testing.T) {
	report := &Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "abcdef0"},
		Files: []FileResult{
			{Relpath: "x.md", Classification: ClassClearSync, CanonContent: "# x\n"},
		},
	}
	body := buildACStub(1, "abcdef0", report)
	if strings.Contains(body, "### Missing-in-target file routing") {
		t.Errorf("AC108 AT7: ### Missing-in-target file routing must NOT appear when no missing-in-target files exist, got:\n%s", body)
	}
}

// AC108 AT8 — Per-file Coupled-with line uses signal-name shape for coupled
// files; emits no line for uncoupled.
func TestClassR_CoupledWithSignalName(t *testing.T) {
	// Coupled fixture: two .go files in the same package (Go same-package
	// signal). Use a real temp dir so classifyCouplingSignal can read.
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "cmd", "demo")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"a.go", "b.go"} {
		if err := os.WriteFile(filepath.Join(pkgDir, f), []byte("package demo\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	report := &Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "abcdef0", Target: dir},
		Files: []FileResult{
			{Relpath: "cmd/demo/a.go", Classification: ClassAmbiguity, Commits: []string{"abc subject"}},
			{Relpath: "cmd/demo/b.go", Classification: ClassAmbiguity, Commits: []string{"def subject"}},
			{Relpath: "lonely.md", Classification: ClassAmbiguity, Commits: []string{"ghi subject"}},
		},
		RoutingGroups: [][]string{
			{"cmd/demo/a.go", "cmd/demo/b.go"},
			{"lonely.md"},
		},
	}
	body := buildACStub(1, "abcdef0", report)
	// Coupled files: signal-name shape.
	if !strings.Contains(body, "Coupled-with: Go same-package set (see § Coupled sets)") {
		t.Errorf("AC108 AT8 (coupled): expected `Coupled-with: Go same-package set (see § Coupled sets)`, got:\n%s", body)
	}
	// Uncoupled file: no Coupled-with line at all (silence > negative-state noise).
	notes, _ := sectionBody(body, "Implementation Notes")
	lonelyIdx := strings.Index(notes, "#### `lonely.md`")
	if lonelyIdx < 0 {
		t.Fatalf("missing per-file block for lonely.md")
	}
	tail := notes[lonelyIdx:]
	if endIdx := strings.Index(tail[1:], "#### `"); endIdx > 0 {
		tail = tail[:endIdx+1]
	}
	if strings.Contains(tail, "Coupled-with:") {
		t.Errorf("AC108 AT8 (uncoupled): per-file block must not emit Coupled-with line, got:\n%s", tail)
	}
}

// AC112 Class Y (formerly AC108 AT9 / Class S, made obsolete) — the routing
// summary table is dropped entirely; column-shape assertions are vacuous.
// Replaced by TestClassY_NoRoutingSummaryTable above.

// AC108 AT10 — Counts line carries hard-routed-format-defining annotation
// when format-defining files are divergent, and auto-routed-create-from-canon
// annotation when missing-in-target with non-empty canon files exist.
func TestClassT_CountsLineAnnotated(t *testing.T) {
	body := stagedACAC108Fixture(t)
	if !strings.Contains(body, "ambiguity (1 hard-routed via format-defining)") {
		t.Errorf("AC108 AT10: counts line missing hard-routed-via-format-defining annotation, got:\n%s", body)
	}
	if !strings.Contains(body, "missing-in-target (1 auto-routed as create-from-canon)") {
		t.Errorf("AC108 AT10: counts line missing auto-routed-as-create-from-canon annotation, got:\n%s", body)
	}
}

// AC112 Class Y (formerly AC108 AT11) — counts annotation still emitted in
// `## Implementation Notes` Counts line; reconciliation against the now-dropped
// routing-summary table is no longer applicable. The annotated counts still
// surface format-defining hard-routes and missing-in-target auto-routes (covered
// by TestClassT_CountsLineAnnotated above).
func TestClassY_CountsAnnotationStillEmitted(t *testing.T) {
	body := stagedACAC108Fixture(t)
	// AGENTS.md is hard-routed via format-defining; counts annotation present.
	if !strings.Contains(body, "2 ambiguity (1 hard-routed via format-defining)") {
		t.Errorf("AC112 Class Y: counts annotation still required post-table-removal, got:\n%s", body)
	}
	// Routing summary table is gone — verify negative.
	if strings.Contains(body, "### Routing summary") {
		t.Errorf("AC112 Class Y: ### Routing summary must not appear, got:\n%s", body)
	}
}

// AC109 AT1 — Director Review opens with a routing-menu stamp when at least
// one Q exists. Stamp documents both menus (ambiguity: sync/preserve/defer;
// target-has-no-canon: keep/delete/migrate-to-canon) and references the
// qualified Resolution-protocol path.
func TestClassV_RoutingMenuStampEmitted(t *testing.T) {
	report := &Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "abcdef0"},
		Files: []FileResult{
			{Relpath: "amb.md", Classification: ClassAmbiguity, Commits: []string{"abc subject"}},
			{Relpath: "no-canon.md", Classification: ClassTargetNoCanon},
		},
	}
	body := buildACStub(1, "abcdef0", report)
	dr, ok := sectionBody(body, "Director Review")
	if !ok {
		t.Fatalf("missing Director Review")
	}
	// AC111 Class X: bulleted-block format. Verify the heading + each bullet.
	wants := []string{
		"**Routing menu** (pick one per Q):",
		"- `sync` — file moves to In Scope",
		"- `preserve` — file stays in Out Of Scope; backfill `preserve <path> <qualifier>` in CHANGELOG.md at next release prep",
		"- `defer` — file becomes a follow-on AC pointer (new IE in `plan.md`)",
		"- For `target-has-no-canon` files: `keep` / `delete` instead. See `governa @ abcdef0: docs/drift-scan.md ## Resolution protocol`.",
	}
	for _, want := range wants {
		if !strings.Contains(dr, want) {
			t.Errorf("AC111: bulleted routing-menu block missing %q, got:\n%s", want, dr)
		}
	}
}

// AC109 AT2 — Per-Q ambiguity entries match the new shape: file in backticks,
// then `— <placeholder>. Why: <placeholder>.` No "Should ... synced/preserved"
// boilerplate; no per-Q `Coupled-with:` annotation.
func TestClassV_AmbiguityQShape(t *testing.T) {
	report := &Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "abcdef0"},
		Files: []FileResult{
			{Relpath: "amb.md", Classification: ClassAmbiguity, Commits: []string{"abc subject"}},
		},
		RoutingGroups: [][]string{{"amb.md"}},
	}
	body := buildACStub(1, "abcdef0", report)
	dr, ok := sectionBody(body, "Director Review")
	if !ok {
		t.Fatalf("missing Director Review")
	}
	const wantShape = "1. **`amb.md`** — <!-- TBD by Operator -->. Why: <!-- TBD by Operator -->."
	if !strings.Contains(dr, wantShape) {
		t.Errorf("AC109 AT2: expected ambiguity Q shape %q, got:\n%s", wantShape, dr)
	}
	// Negative: legacy boilerplate must NOT appear.
	if strings.Contains(dr, "be synced to canon, preserved with a marker") {
		t.Errorf("AC109 AT2: legacy `Should ... synced/preserved/deferred` boilerplate must be removed, got:\n%s", dr)
	}
	// Negative: per-Q `Coupled-with:` annotation must NOT appear (coupling
	// info lives in `### Coupled sets`).
	if strings.Contains(dr, "Coupled-with:") {
		t.Errorf("AC109 AT2: per-Q `Coupled-with:` annotation must be removed (info lives in `### Coupled sets`), got:\n%s", dr)
	}
}

// AC109 AT3 — Per-Q target-has-no-canon entries carry `(target-has-no-canon)`
// annotation between file and placeholder; no per-Q keep/delete/migrate
// boilerplate.
func TestClassV_TargetHasNoCanonQShape(t *testing.T) {
	report := &Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "abcdef0"},
		Files: []FileResult{
			{Relpath: "no-canon.md", Classification: ClassTargetNoCanon},
		},
	}
	body := buildACStub(1, "abcdef0", report)
	dr, ok := sectionBody(body, "Director Review")
	if !ok {
		t.Fatalf("missing Director Review")
	}
	const wantShape = "1. **`no-canon.md`** (target-has-no-canon) — <!-- TBD by Operator -->. Why: <!-- TBD by Operator -->."
	if !strings.Contains(dr, wantShape) {
		t.Errorf("AC109 AT3: expected target-has-no-canon Q shape %q, got:\n%s", wantShape, dr)
	}
	// Negative: legacy per-Q "kept as a per-repo addition, deleted, or
	// migrated into canon" boilerplate must NOT appear (menu lives in stamp).
	if strings.Contains(dr, "kept as a per-repo addition, deleted, or migrated into canon") {
		t.Errorf("AC109 AT3: legacy keep/delete/migrate per-Q boilerplate must be removed (menu lives in stamp), got:\n%s", dr)
	}
}

// AC109 AT4 — When Director Review is `None.`, no routing-menu stamp is
// emitted.
func TestClassV_NoStampWhenDirectorReviewNone(t *testing.T) {
	report := &Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "abcdef0"},
		Files: []FileResult{
			{Relpath: "x.md", Classification: ClassClearSync, CanonContent: "# x\n"},
		},
	}
	body := buildACStub(1, "abcdef0", report)
	dr, ok := sectionBody(body, "Director Review")
	if !ok {
		t.Fatalf("missing Director Review")
	}
	if !strings.Contains(dr, "None.") {
		t.Errorf("AC109 AT4: expected `None.` body when no Qs, got:\n%s", dr)
	}
	if strings.Contains(dr, "_Routing menu —") {
		t.Errorf("AC109 AT4: routing-menu stamp must NOT be emitted when Director Review is None, got:\n%s", dr)
	}
}

// AC109 AT5 — `docs/drift-scan.md` documents the routing-matrix shape and the
// tool-emission exception to `docs/ac-template.md`'s question-form rule.
func TestClassV_DocumentedInDriftScanMd(t *testing.T) {
	data, err := os.ReadFile("../../docs/drift-scan.md")
	if err != nil {
		t.Fatalf("read drift-scan.md: %v", err)
	}
	doc := string(data)
	if !strings.Contains(doc, "routing-matrix shape") {
		t.Errorf("AC109 AT5: drift-scan.md must document the routing-matrix shape")
	}
	if !strings.Contains(doc, "Tool-emission exception") {
		t.Errorf("AC109 AT5: drift-scan.md must call out the tool-emission exception to ac-template.md's question-form rule")
	}
}

// AC112 AT2 — each per-file Divergent files block carries a `What diverged: <!-- TBD by Operator -->`
// line positioned between `Direction:` and `Local commits:`.
func TestClassY_PerFileWhatDivergedLine(t *testing.T) {
	body := stagedACAC108Fixture(t)
	notes, ok := sectionBody(body, "Implementation Notes")
	if !ok {
		t.Fatalf("missing ## Implementation Notes")
	}
	dvIdx := strings.Index(notes, "### Divergent files")
	if dvIdx < 0 {
		t.Fatalf("missing ### Divergent files in:\n%s", notes)
	}
	dvSection := notes[dvIdx:]
	if endIdx := strings.Index(dvSection[1:], "\n### "); endIdx > 0 {
		dvSection = dvSection[:endIdx+1]
	}
	blocks := strings.Split(dvSection, "#### `")
	for _, blk := range blocks[1:] {
		if !strings.Contains(blk, "What diverged: <!-- TBD by Operator -->") {
			t.Errorf("AC112 AT2: per-file block missing `What diverged: <!-- TBD by Operator -->` line:\n%s", blk)
		}
		// Sequencing: Direction must precede What diverged.
		dirIdx := strings.Index(blk, "Direction:")
		wdIdx := strings.Index(blk, "What diverged:")
		if dirIdx < 0 || wdIdx < 0 || dirIdx >= wdIdx {
			t.Errorf("AC112 AT2: `Direction:` must precede `What diverged:` in per-file block:\n%s", blk)
		}
	}
}

// AC112 AT3 — Director Review carries the convention footer when ≥1 Q exists.
func TestClassY_DirectorReviewConventionFooter(t *testing.T) {
	body := stagedACAC108Fixture(t)
	dr, ok := sectionBody(body, "Director Review")
	if !ok {
		t.Fatalf("missing Director Review")
	}
	const want = "_Director Review form follows the drift-scan tool-emission convention. See `governa @ abcdef0: docs/drift-scan.md ## Director Review` for the documented exception to your local docs/ac-template.md question-form rule._"
	if !strings.Contains(dr, want) {
		t.Errorf("AC112 AT3: Director Review must carry convention footer when Qs exist, got:\n%s", dr)
	}
}

// AC112 AT4 — When Director Review is `None.`, the convention footer is NOT emitted.
func TestClassY_NoConventionFooterWhenDirectorReviewNone(t *testing.T) {
	report := &Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "abcdef0"},
		Files: []FileResult{
			{Relpath: "x.md", Classification: ClassClearSync, CanonContent: "# x\n"},
		},
	}
	body := buildACStub(1, "abcdef0", report)
	dr, ok := sectionBody(body, "Director Review")
	if !ok {
		t.Fatalf("missing Director Review")
	}
	if !strings.Contains(dr, "None.") {
		t.Errorf("AC112 AT4: expected `None.` when no Qs, got:\n%s", dr)
	}
	if strings.Contains(dr, "Director Review form follows the drift-scan tool-emission convention") {
		t.Errorf("AC112 AT4: convention footer must NOT appear when Director Review is None, got:\n%s", dr)
	}
}

// AC112 AT5 — name-reference body scan surfaces target-only files referenced
// from a divergent target file. Fixture: a divergent rel.sh references
// ./cmd/foo/color.go which is target-only (no canon).
func TestClassZ_NameReferenceBodyScan(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS.md\n")
	mustWrite(t, filepath.Join(dir, "plan.md"), "# Plan\n\n## Ideas To Explore\n\n- IE1: existing\n")
	mustWrite(t, filepath.Join(dir, "CHANGELOG.md"), "# Changelog\n\n| Version | Summary |\n|---|---|\n| Unreleased | |\n")
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "docs/ac-template.md"), "# AC template\n")
	// Divergent rel.sh referencing target-only color.go.
	mustWrite(t, filepath.Join(dir, "rel.sh"), "#!/usr/bin/env bash\nexec go run ./cmd/foo/main.go ./cmd/foo/color.go \"$@\"\n")
	// Target-only files (no canon counterpart).
	mustWrite(t, filepath.Join(dir, "cmd/foo/main.go"), "package main\nfunc main() {}\n")
	mustWrite(t, filepath.Join(dir, "cmd/foo/color.go"), "package main\nfunc col() {}\n")
	gitInit(t, dir)
	gitAddCommit(t, dir, "AC1: rel.sh + color.go")

	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	matches := findStagedACs(t, dir)
	ac := mustRead(t, matches[0])

	// cmd/foo/color.go should appear under target-has-no-canon (via name-reference).
	if !strings.Contains(ac, "`cmd/foo/color.go`") {
		t.Errorf("AC112 AT5: name-referenced target-only `cmd/foo/color.go` must surface, got:\n%s", ac)
	}
	// Director Review Q with target-has-no-canon annotation.
	dr, _ := sectionBody(ac, "Director Review")
	if !strings.Contains(dr, "**`cmd/foo/color.go`** (target-has-no-canon)") {
		t.Errorf("AC112 AT5: Director Review must carry (target-has-no-canon) Q for color.go, got:\n%s", dr)
	}
}

// AC112 AT6 — name-reference scan does not false-positive on canon-resident refs.
func TestClassZ_NoFalsePositiveOnCanonResidentRef(t *testing.T) {
	// rel.sh references ./cmd/rel/main.go which IS in canon (DOC overlay).
	// Should NOT trigger target-has-no-canon for main.go.
	dir := docFixture(t)
	mustWrite(t, filepath.Join(dir, "rel.sh"), "#!/usr/bin/env bash\nexec go run ./cmd/rel/main.go \"$@\"\n")
	mustWrite(t, filepath.Join(dir, "cmd/rel/main.go"), "package main\nfunc main() {}\n")
	gitAddCommit(t, dir, "rel.sh refs canon-resident main.go")

	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	matches := findStagedACs(t, dir)
	ac := mustRead(t, matches[0])

	// main.go is in canon — must not appear with target-has-no-canon annotation.
	if strings.Contains(ac, "**`cmd/rel/main.go`** (target-has-no-canon)") {
		t.Errorf("AC112 AT6: canon-resident main.go must NOT trigger target-has-no-canon, got:\n%s", ac)
	}
}

// AC112 AT7 — docs/drift-scan.md `## What the Operator fills` is in imperative
// form: contains `the Operator MUST` at least twice; carries five numbered
// fill-spot subsections plus `### Handoff verification`; the Post-merge audit
// subsection contains a 5-step numbered procedure; the AC4-AT-label-rule
// concrete failure is cited verbatim.
func TestClassesAABB_OperatorFillsImperative(t *testing.T) {
	data, err := os.ReadFile("../../docs/drift-scan.md")
	if err != nil {
		t.Fatalf("read drift-scan.md: %v", err)
	}
	doc := string(data)
	if strings.Count(doc, "the Operator MUST") < 2 {
		t.Errorf("AC112 AT7: `## What the Operator fills` must use imperative `the Operator MUST` at least twice")
	}
	wants := []string{
		"### 1. Per-file `What diverged`",
		"### 2. Director Review per-Q lean + why",
		"### 3. Post-merge coherence audit",
		"### 4. Summary",
		"### 5. Objective Fit",
		"### Handoff verification",
		"AC4 in tips",
		"AT-label timing-axis",
	}
	for _, want := range wants {
		if !strings.Contains(doc, want) {
			t.Errorf("AC112 AT7: drift-scan.md ## What the Operator fills missing %q", want)
		}
	}
}

// =====================================================================
// AC114 — Drift-Scan Staging Promotions + Verify Subcommand
// =====================================================================

// AC114 AT1 — parseObjectiveFitForm returns the ordered heading list for
// 3-part / 4-part templates; nil for missing file / missing section / no items.
func TestAC114_AT1_ParseObjectiveFitForm(t *testing.T) {
	t.Run("3-part canon form", func(t *testing.T) {
		dir := t.TempDir()
		mustWrite(t, filepath.Join(dir, "docs/ac-template.md"),
			"## Objective Fit\n\n1. **Outcome.** What this delivers.\n2. **Priority.** Why this.\n3. **Dependencies.** Prior ACs.\n\n## In Scope\n")
		got := parseObjectiveFitForm(dir)
		want := []string{"Outcome.", "Priority.", "Dependencies."}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("got[%d]=%q, want %q", i, got[i], want[i])
			}
		}
	})
	t.Run("4-part target form", func(t *testing.T) {
		dir := t.TempDir()
		mustWrite(t, filepath.Join(dir, "docs/ac-template.md"),
			"## Objective Fit\n\n1. **Which part?** Tie to objective.\n2. **Why not higher?** Trade-off.\n3. **What existing decision?** Reference.\n4. **Intentional pivot?** If yes.\n\n## In Scope\n")
		got := parseObjectiveFitForm(dir)
		if len(got) != 4 {
			t.Fatalf("expected 4 headings, got %d: %v", len(got), got)
		}
	})
	t.Run("missing file returns nil", func(t *testing.T) {
		dir := t.TempDir()
		if got := parseObjectiveFitForm(dir); got != nil {
			t.Errorf("expected nil for missing file, got %v", got)
		}
	})
}

// AC114 AT2 — staged AC `## Objective Fit` is a numbered scaffold matching
// target's parsed form.
func TestAC114_AT2_ObjectiveFitScaffold(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "docs/ac-template.md"),
		"## Objective Fit\n\n1. **Which part?** Tie.\n2. **Why not?** TO.\n3. **Existing?** Ref.\n4. **Pivot?** If.\n\n## In Scope\n")
	report := &Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "abcdef0", Target: dir},
	}
	out := buildACStub(1, "abcdef0", report)
	for _, want := range []string{
		"## Objective Fit\n\n1. **Which part?** <!-- TBD by Operator -->",
		"2. **Why not?** <!-- TBD by Operator -->",
		"3. **Existing?** <!-- TBD by Operator -->",
		"4. **Pivot?** <!-- TBD by Operator -->",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

// AC114 AT3 — fallback to canon's 3-part form when target ac-template is
// missing.
func TestAC114_AT3_ObjectiveFitFallback(t *testing.T) {
	dir := t.TempDir() // no docs/ac-template.md
	report := &Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "abcdef0", Target: dir},
	}
	out := buildACStub(1, "abcdef0", report)
	for _, want := range []string{
		"## Objective Fit\n\n1. **Outcome** <!-- TBD by Operator -->",
		"2. **Priority** <!-- TBD by Operator -->",
		"3. **Dependencies** <!-- TBD by Operator -->",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing fallback %q in:\n%s", want, out)
		}
	}
}

// AC114 AT4 — named constant `imperativeRuleRe` exists with verbatim pattern.
func TestAC114_AT4_ImperativeRuleReConstant(t *testing.T) {
	const want = `(?i)\b(must|every|requires|shall|always|never|each)\b`
	if got := imperativeRuleRe.String(); got != want {
		t.Errorf("imperativeRuleRe.String() = %q, want %q", got, want)
	}
}

// AC114 AT5 — extractRuleCandidates uses imperativeRuleRe and returns one
// ruleCandidate per matching `+` line.
func TestAC114_AT5_ExtractRuleCandidates(t *testing.T) {
	diff := "--- canon/foo\n+++ target/foo\n" +
		"@@ -1,5 +1,8 @@\n" +
		" context line\n" +
		"+You must do X.\n" +
		"+Every AC labels stuff.\n" +
		"+- Each AC labels each acceptance test.\n" +
		"+just an addition no imperative\n" +
		"+another plain line\n"
	got := extractRuleCandidates("foo", diff)
	if len(got) != 3 {
		t.Fatalf("expected 3 candidates, got %d: %+v", len(got), got)
	}
	for _, want := range []string{"You must do X.", "Every AC labels stuff.", "- Each AC labels each acceptance test."} {
		found := false
		for _, c := range got {
			if strings.Contains(c.Excerpt, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing candidate matching %q in %+v", want, got)
		}
	}
}

// AC114 AT6 — integration: extractRuleCandidates output flows into rendered
// `[TBD] R<N>:` checklist lines.
func TestAC114_AT6_ChecklistIntegration(t *testing.T) {
	diff := "--- canon/AGENTS.md\n+++ target/AGENTS.md\n" +
		"@@ -1,3 +1,5 @@\n" +
		" line\n" +
		"+You must do X.\n" +
		"+Every rule applies.\n"
	report := &Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "abcdef0"},
		Files: []FileResult{
			{Relpath: "AGENTS.md", Classification: ClassClearSync, CanonContent: "# A\n", Diff: diff},
			{Relpath: "preserved.md", Classification: ClassPreserve, Markers: []string{"| 0.1.0 | preserve preserved.md customization |"}},
		},
	}
	out := buildACStub(1, "abcdef0", report)
	candidates := extractRuleCandidates("AGENTS.md", diff)
	if len(candidates) == 0 {
		t.Fatalf("test setup: no candidates extracted")
	}
	for _, c := range candidates {
		want := fmt.Sprintf("`%s` adds at line `%d`:", c.File, c.LineNum)
		if !strings.Contains(out, want) {
			t.Errorf("rendered checklist missing %q for candidate %+v in:\n%s", want, c, out)
		}
	}
}

// AC114 AT7 — when sync ∧ preserve, audit body is the checklist scaffold.
func TestAC114_AT7_AuditChecklistShape(t *testing.T) {
	diff := "--- canon/A\n+++ target/A\n@@ -1,1 +1,2 @@\n line\n+must do X\n"
	report := &Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "abcdef0"},
		Files: []FileResult{
			{Relpath: "A.md", Classification: ClassClearSync, CanonContent: "# A\n", Diff: diff},
			{Relpath: "B.md", Classification: ClassPreserve, Markers: []string{"| 0.1.0 | preserve B.md customization |"}},
		},
	}
	out := buildACStub(1, "abcdef0", report)
	for _, want := range []string{
		"### Post-merge coherence audit",
		"**Synced files:** `A.md`",
		"**Preserved files:** `B.md`",
		"Rules added by sync (extracted mechanically",
		"[TBD] R1:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

// AC114 AT8 — sync-empty: vacuous body, no `<!-- TBD by Operator -->`.
func TestAC114_AT8_VacuousSyncEmpty(t *testing.T) {
	report := &Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "abcdef0"},
		Files: []FileResult{
			{Relpath: "B.md", Classification: ClassPreserve, Markers: []string{"| 0.1.0 | preserve B.md customization |"}},
		},
	}
	out := buildACStub(1, "abcdef0", report)
	if !strings.Contains(out, vacuousAuditSyncEmpty) {
		t.Errorf("expected vacuous sync-empty body in:\n%s", out)
	}
	auditIdx := strings.Index(out, "### Post-merge coherence audit")
	if auditIdx < 0 {
		t.Fatalf("missing audit section")
	}
	tail := out[auditIdx:]
	endIdx := strings.Index(tail[1:], "## ")
	if endIdx > 0 {
		tail = tail[:endIdx+1]
	}
	if strings.Contains(tail, "<!-- TBD by Operator -->") {
		t.Errorf("audit section must not contain TBD when vacuous, got:\n%s", tail)
	}
}

// AC114 AT9 — preserve-empty (sync non-empty): vacuous preserve-empty body.
func TestAC114_AT9_VacuousPreserveEmpty(t *testing.T) {
	report := &Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "abcdef0"},
		Files: []FileResult{
			{Relpath: "A.md", Classification: ClassClearSync, CanonContent: "# A\n"},
		},
	}
	out := buildACStub(1, "abcdef0", report)
	if !strings.Contains(out, vacuousAuditPreserveEmpty) {
		t.Errorf("expected vacuous preserve-empty body in:\n%s", out)
	}
}

// AC114 AT10 — Verify reports failures for any TBD substring in 4 sections.
func TestAC114_AT10_VerifyTBDFailures(t *testing.T) {
	cases := []struct {
		name    string
		acBody  string
		wantSub string
	}{
		{"TBD in Summary", "# AC1\n\n## Summary\n\n<!-- TBD by Operator -->\n\n## Status\n", "Summary"},
		{"TBD in DR lean (via check 1)", "# AC1\n\n## Director Review\n\n1. **`x.md`** — <!-- TBD by Operator -->. Why: actual reason.\n\n## Status\n", "Director Review"},
		{"TBD in DR Why", "# AC1\n\n## Director Review\n\n1. **`x.md`** — preserve. Why: <!-- TBD by Operator -->.\n\n## Status\n", "Director Review"},
		{"TBD in What diverged", "# AC1\n\n## Implementation Notes\n\n### Divergent files\n\n#### `x.md` — ambiguity\n\nWhat diverged: <!-- TBD by Operator -->\n\n## Status\n", "Implementation Notes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "ac.md")
			mustWrite(t, path, tc.acBody)
			failures, err := Verify(path)
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if len(failures) == 0 {
				t.Fatalf("expected ≥1 failure, got 0")
			}
			found := false
			for _, f := range failures {
				if strings.Contains(f.Section, tc.wantSub) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected failure in section containing %q, got %+v", tc.wantSub, failures)
			}
		})
	}
}

// AC114 AT11 — check 5 sync ∧ preserve heuristic per pinned parse rules.
func TestAC114_AT11_VerifyCheck5HeuristicEdgeCases(t *testing.T) {
	makeAC := func(inScope, outOfScope, audit string) string {
		return "# AC1\n\n## In Scope\n\n" + inScope + "\n\n## Out Of Scope\n\n" + outOfScope +
			"\n\n## Implementation Notes\n\n### Post-merge coherence audit\n\n" + audit + "\n\n## Status\n\n`PENDING`\n"
	}
	t.Run("AT11a InScope-None + preserve-present → no fire", func(t *testing.T) {
		body := makeAC("None — body lands as Director resolves Q1–Q3.",
			"- `foo.md` — preserve marker present:\n  - `marker`",
			vacuousAuditSyncEmpty)
		if hasSyncItemsAndPreserveMarkers(body) {
			t.Errorf("expected no fire (no sync items)")
		}
	})
	t.Run("AT11b sync-present + OutOfScope-None → no fire", func(t *testing.T) {
		body := makeAC("- `foo.md` — sync to canon",
			"None.",
			vacuousAuditPreserveEmpty)
		if hasSyncItemsAndPreserveMarkers(body) {
			t.Errorf("expected no fire (no preserve markers)")
		}
	})
	t.Run("AT11c create-from-canon-only counts as sync", func(t *testing.T) {
		body := makeAC("- `foo.md` — create from canon",
			"- `bar.md` — preserve marker present:\n  - `marker`",
			"clean reconciliation outcome documented")
		if !hasSyncItemsAndPreserveMarkers(body) {
			t.Errorf("expected fire (create-from-canon counts as sync)")
		}
	})
	t.Run("AT11d both-present + audit has [TBD] → check 5 reports failure", func(t *testing.T) {
		body := makeAC("- `foo.md` — sync to canon\n- `baz.md` — create from canon",
			"- `bar.md` — preserve marker present:\n  - `marker`",
			"Rules:\n- [TBD] R1: foo.md adds at line 3 — reconciliation: ?")
		dir := t.TempDir()
		path := filepath.Join(dir, "ac.md")
		mustWrite(t, path, body)
		failures, err := Verify(path)
		if err != nil {
			t.Fatalf("Verify: %v", err)
		}
		found := false
		for _, f := range failures {
			if strings.Contains(f.Description, "[TBD]") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected failure mentioning [TBD] in audit, got %+v", failures)
		}
	})
	t.Run("AT11e both-present + audit clean → no [TBD] failure", func(t *testing.T) {
		body := makeAC("- `foo.md` — sync to canon",
			"- `bar.md` — preserve marker present:\n  - `marker`",
			"Reconciliation outcome: R1 acknowledged in bar.md.")
		dir := t.TempDir()
		path := filepath.Join(dir, "ac.md")
		mustWrite(t, path, body)
		failures, err := Verify(path)
		if err != nil {
			t.Fatalf("Verify: %v", err)
		}
		for _, f := range failures {
			if strings.Contains(f.Description, "[TBD]") {
				t.Errorf("unexpected [TBD] failure when audit body is clean: %+v", f)
			}
		}
	})
}

// AC114 AT12 — Verify returns zero failures for a clean filled AC body.
func TestAC114_AT12_VerifyCleanAC(t *testing.T) {
	body := "# AC1 Drift-Scan from governa @ abc\n\n## Summary\n\nFilled summary.\n\n## Objective Fit\n\n1. **Outcome** delivers X.\n2. **Priority** P.\n3. **Dependencies** D.\n\n## In Scope\n\n- `foo.md` — sync to canon\n\n## Out Of Scope\n\nNone.\n\n## Implementation Notes\n\n### Divergent files\n\n#### `foo.md` — ambiguity\n\nWhat diverged: target adds X.\n\n### Post-merge coherence audit\n\nReconciliation done.\n\n## Acceptance Tests\n\n**AT1** [Automated] — check.\n\n## Director Review\n\nNone.\n\n## Status\n\n`PENDING`\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "ac.md")
	mustWrite(t, path, body)
	failures, err := Verify(path)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected zero failures, got: %+v", failures)
	}
}

// AC114 AT13 — RunVerifyCLI prints failures + exits 1 on failures, 0 on clean.
func TestAC114_AT13_VerifyCLI(t *testing.T) {
	t.Run("failures present → exit 1", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "ac.md")
		mustWrite(t, path, "# AC1\n\n## Summary\n\n<!-- TBD by Operator -->\n\n## Status\n")
		var buf bytes.Buffer
		exit, err := RunVerifyCLI([]string{path}, &buf)
		if err != nil {
			t.Fatalf("RunVerifyCLI: %v", err)
		}
		if exit != 1 {
			t.Errorf("expected exit 1, got %d", exit)
		}
		out := buf.String()
		if !strings.Contains(out, ":Summary:") {
			t.Errorf("expected '<line>:Summary: ...' format, got:\n%s", out)
		}
	})
	t.Run("clean AC → exit 0, no output", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "ac.md")
		mustWrite(t, path, "# AC1\n\n## Summary\n\nFilled.\n\n## Status\n")
		var buf bytes.Buffer
		exit, err := RunVerifyCLI([]string{path}, &buf)
		if err != nil {
			t.Fatalf("RunVerifyCLI: %v", err)
		}
		if exit != 0 {
			t.Errorf("expected exit 0, got %d", exit)
		}
		if buf.Len() != 0 {
			t.Errorf("expected no output, got:\n%s", buf.String())
		}
	})
}

// =====================================================================
// AC116 — Drift-Scan Resolve Subcommand
// =====================================================================

// ac116Fixture builds an AC body containing both ambiguity Qs and a
// target-has-no-canon Q for testing.
func ac116Fixture() string {
	return "# AC4 Drift-Scan from governa @ abcdef0\n\n" +
		"## Summary\n\nFilled.\n\n" +
		"## Objective Fit\n\n1. **Outcome** delivers X.\n\n" +
		"## In Scope\n\nNone.\n\n" +
		"## Out Of Scope\n\nNone.\n\n" +
		"## Implementation Notes\n\nCanon: governa @ abcdef0, flavor `doc`.\n\n" +
		"## Acceptance Tests\n\nNone — this AC ships only the staged plan.md IE entry; nothing to verify in target.\n\n" +
		"## Director Review\n\n" +
		"**Routing menu** (pick one per Q):\n\n" +
		"1. **`amb1.md`** — preserve. Why: per-repo content.\n" +
		"2. **`AGENTS.md`** — sync. Why: canon refinement.\n" +
		"3. **`amb3.md`** — defer. Why: needs scoping.\n" +
		"4. **`thnc.md`** (target-has-no-canon) — keep. Why: per-repo addition.\n\n" +
		"## Status\n\n`PENDING`\n"
}

func writeAC116Fixture(t *testing.T, dir string) string {
	t.Helper()
	mustWrite(t, filepath.Join(dir, "plan.md"), "# Plan\n\n## Ideas To Explore\n\n(none active)\n")
	acPath := filepath.Join(dir, "docs", "ac4-drift-scan-from-abcdef0.md")
	mustWrite(t, acPath, ac116Fixture())
	return acPath
}

// AC116 AT1 — `resolve` is dispatched by RunCLI to RunResolveCLI.
func TestAC116_AT1_ResolveDispatch(t *testing.T) {
	exit, err := RunCLI([]string{"resolve", "/nonexistent/ac.md", "1", "sync"}, EmbeddedFS)
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if exit != 1 {
		t.Errorf("expected exit 1 (resolve error on nonexistent path), got %d", exit)
	}
}

// AC116 AT2 — Resolve errors for missing path, Q num out of range,
// invalid decision for Q type, decision not in any menu.
func TestAC116_AT2_ResolveErrors(t *testing.T) {
	dir := t.TempDir()
	acPath := writeAC116Fixture(t, dir)

	t.Run("missing AC path", func(t *testing.T) {
		err := Resolve("/nonexistent/ac.md", 1, "sync", EmbeddedFS)
		if err == nil {
			t.Error("expected error for missing path")
		}
	})
	t.Run("Q num out of range", func(t *testing.T) {
		err := Resolve(acPath, 99, "sync", EmbeddedFS)
		if err == nil || !strings.Contains(err.Error(), "Q99 not found") {
			t.Errorf("expected Q-not-found error, got: %v", err)
		}
	})
	t.Run("invalid decision for ambiguity", func(t *testing.T) {
		err := Resolve(acPath, 1, "keep", EmbeddedFS)
		if err == nil || !strings.Contains(err.Error(), "invalid decision") {
			t.Errorf("expected invalid-decision error, got: %v", err)
		}
	})
	t.Run("invalid decision for target-has-no-canon", func(t *testing.T) {
		err := Resolve(acPath, 4, "sync", EmbeddedFS)
		if err == nil || !strings.Contains(err.Error(), "invalid decision") {
			t.Errorf("expected invalid-decision error, got: %v", err)
		}
	})
	t.Run("migrate-to-canon rejected (Q3 dropped)", func(t *testing.T) {
		err := Resolve(acPath, 4, "migrate-to-canon", EmbeddedFS)
		if err == nil || !strings.Contains(err.Error(), "invalid decision") {
			t.Errorf("expected invalid-decision error for migrate-to-canon, got: %v", err)
		}
	})
}

// AC116 AT3 — parseDirectorReviewQ identifies Q types via the
// (target-has-no-canon) annotation and extracts the file path.
func TestAC116_AT3_ParseDirectorReviewQ(t *testing.T) {
	body := ac116Fixture()
	t.Run("ambiguity Q", func(t *testing.T) {
		qType, fp, _, err := parseDirectorReviewQ(body, 2)
		if err != nil {
			t.Fatalf("parseDirectorReviewQ: %v", err)
		}
		if qType != "ambiguity" || fp != "AGENTS.md" {
			t.Errorf("got (%q, %q), want (ambiguity, AGENTS.md)", qType, fp)
		}
	})
	t.Run("target-has-no-canon Q", func(t *testing.T) {
		qType, fp, _, err := parseDirectorReviewQ(body, 4)
		if err != nil {
			t.Fatalf("parseDirectorReviewQ: %v", err)
		}
		if qType != "target-has-no-canon" || fp != "thnc.md" {
			t.Errorf("got (%q, %q), want (target-has-no-canon, thnc.md)", qType, fp)
		}
	})
}

// AC116 AT4 — validateDecision accepts per-Q-type menus, rejects others.
func TestAC116_AT4_ValidateDecision(t *testing.T) {
	cases := []struct {
		qType    string
		decision string
		wantErr  bool
	}{
		{"ambiguity", "sync", false},
		{"ambiguity", "preserve", false},
		{"ambiguity", "defer", false},
		{"ambiguity", "keep", true},
		{"ambiguity", "delete", true},
		{"ambiguity", "migrate-to-canon", true},
		{"ambiguity", "unknown", true},
		{"target-has-no-canon", "keep", false},
		{"target-has-no-canon", "delete", false},
		{"target-has-no-canon", "migrate-to-canon", true}, // Q3 dropped
		{"target-has-no-canon", "sync", true},
		{"target-has-no-canon", "preserve", true},
		{"target-has-no-canon", "defer", true},
		{"target-has-no-canon", "unknown", true},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s/%s", tc.qType, tc.decision), func(t *testing.T) {
			err := validateDecision(tc.qType, tc.decision)
			if (err != nil) != tc.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

// AC116 AT5 — sync resolution: In Scope gets new line containing file +
// (Director-set); AT section gets new AT line.
func TestAC116_AT5_SyncResolution(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "docs/ac-template.md"), "# template\n")
	acPath := writeAC116Fixture(t, dir)

	if err := Resolve(acPath, 2, "sync", EmbeddedFS); err != nil {
		t.Fatalf("Resolve sync: %v", err)
	}
	body := mustRead(t, acPath)
	inScope, _ := sectionBody(body, "In Scope")
	atSection, _ := sectionBody(body, "Acceptance Tests")
	foundInScope := false
	for _, line := range strings.Split(inScope, "\n") {
		if strings.Contains(line, "AGENTS.md") && strings.Contains(line, "(Director-set)") {
			foundInScope = true
			break
		}
	}
	if !foundInScope {
		t.Errorf("In Scope missing new sync line for AGENTS.md with (Director-set):\n%s", inScope)
	}
	foundAT := false
	atRe := regexp.MustCompile(`\*\*AT\d+\*\*.*AGENTS\.md.*\(Director-set\)`)
	for _, line := range strings.Split(atSection, "\n") {
		if atRe.MatchString(line) {
			foundAT = true
			break
		}
	}
	if !foundAT {
		t.Errorf("AT section missing new AT for AGENTS.md with (Director-set):\n%s", atSection)
	}
}

// AC116 AT6 — preserve resolution: In Scope gets CHANGELOG marker-backfill
// line containing file + (Director-set) + CHANGELOG.md.
func TestAC116_AT6_PreserveResolution(t *testing.T) {
	dir := t.TempDir()
	acPath := writeAC116Fixture(t, dir)

	if err := Resolve(acPath, 1, "preserve", EmbeddedFS); err != nil {
		t.Fatalf("Resolve preserve: %v", err)
	}
	body := mustRead(t, acPath)
	inScope, _ := sectionBody(body, "In Scope")
	found := false
	for _, line := range strings.Split(inScope, "\n") {
		if strings.Contains(line, "amb1.md") && strings.Contains(line, "(Director-set)") && strings.Contains(line, "CHANGELOG.md") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("In Scope missing preserve marker-backfill line for amb1.md:\n%s", inScope)
	}
}

// AC116 AT7 — defer resolution: plan.md gets a new IE entry containing
// file + defer marker. AC body In Scope/AT/Out Of Scope unchanged.
func TestAC116_AT7_DeferResolution(t *testing.T) {
	dir := t.TempDir()
	acPath := writeAC116Fixture(t, dir)
	preBody := mustRead(t, acPath)
	preInScope, _ := sectionBody(preBody, "In Scope")
	preATs, _ := sectionBody(preBody, "Acceptance Tests")
	preOOS, _ := sectionBody(preBody, "Out Of Scope")

	if err := Resolve(acPath, 3, "defer", EmbeddedFS); err != nil {
		t.Fatalf("Resolve defer: %v", err)
	}
	plan := mustRead(t, filepath.Join(dir, "plan.md"))
	if !strings.Contains(plan, "amb3.md") || !strings.Contains(plan, "defer") {
		t.Errorf("plan.md missing defer IE for amb3.md:\n%s", plan)
	}
	postBody := mustRead(t, acPath)
	postInScope, _ := sectionBody(postBody, "In Scope")
	postATs, _ := sectionBody(postBody, "Acceptance Tests")
	postOOS, _ := sectionBody(postBody, "Out Of Scope")
	if preInScope != postInScope {
		t.Errorf("In Scope changed unexpectedly")
	}
	if preATs != postATs {
		t.Errorf("Acceptance Tests changed unexpectedly")
	}
	if preOOS != postOOS {
		t.Errorf("Out Of Scope changed unexpectedly")
	}
}

// AC116 AT8 — Q line in Director Review gets (Director-set) annotation +
// resolved-decision marker after each Resolve.
func TestAC116_AT8_QLineAnnotation(t *testing.T) {
	dir := t.TempDir()
	acPath := writeAC116Fixture(t, dir)

	if err := Resolve(acPath, 1, "preserve", EmbeddedFS); err != nil {
		t.Fatalf("Resolve preserve: %v", err)
	}
	body := mustRead(t, acPath)
	dr, _ := sectionBody(body, "Director Review")
	q1Found := false
	q2Untouched := true
	for _, line := range strings.Split(dr, "\n") {
		if strings.HasPrefix(line, "1. **`amb1.md`**") {
			if strings.Contains(line, "(Director-set)") && strings.Contains(line, "preserve") {
				q1Found = true
			}
		}
		if strings.HasPrefix(line, "2. **`AGENTS.md`**") {
			if strings.Contains(line, "(Director-set)") {
				q2Untouched = false
			}
		}
	}
	if !q1Found {
		t.Errorf("Q1 line missing (Director-set) annotation:\n%s", dr)
	}
	if !q2Untouched {
		t.Errorf("Q2 line should be untouched but carries (Director-set):\n%s", dr)
	}
}

// AC116 AT9 — keep resolution: no AC In Scope/AT/Out Of Scope mutation,
// no plan.md mutation; only the Q line gets annotated.
func TestAC116_AT9_KeepResolution(t *testing.T) {
	dir := t.TempDir()
	acPath := writeAC116Fixture(t, dir)
	preBody := mustRead(t, acPath)
	preInScope, _ := sectionBody(preBody, "In Scope")
	preATs, _ := sectionBody(preBody, "Acceptance Tests")
	preOOS, _ := sectionBody(preBody, "Out Of Scope")
	prePlan := mustRead(t, filepath.Join(dir, "plan.md"))

	if err := Resolve(acPath, 4, "keep", EmbeddedFS); err != nil {
		t.Fatalf("Resolve keep: %v", err)
	}
	postBody := mustRead(t, acPath)
	postInScope, _ := sectionBody(postBody, "In Scope")
	postATs, _ := sectionBody(postBody, "Acceptance Tests")
	postOOS, _ := sectionBody(postBody, "Out Of Scope")
	postPlan := mustRead(t, filepath.Join(dir, "plan.md"))

	if preInScope != postInScope || preATs != postATs || preOOS != postOOS {
		t.Errorf("keep should not mutate In Scope/AT/Out Of Scope")
	}
	if prePlan != postPlan {
		t.Errorf("keep should not mutate plan.md")
	}
	dr, _ := sectionBody(postBody, "Director Review")
	if !strings.Contains(dr, "(Director-set)") {
		t.Errorf("keep should annotate Q line:\n%s", dr)
	}
}

// AC116 AT10 — delete resolution: In Scope gets new line containing file +
// (Director-set) + delete marker.
func TestAC116_AT10_DeleteResolution(t *testing.T) {
	dir := t.TempDir()
	acPath := writeAC116Fixture(t, dir)

	if err := Resolve(acPath, 4, "delete", EmbeddedFS); err != nil {
		t.Fatalf("Resolve delete: %v", err)
	}
	body := mustRead(t, acPath)
	inScope, _ := sectionBody(body, "In Scope")
	found := false
	for _, line := range strings.Split(inScope, "\n") {
		if strings.Contains(line, "thnc.md") && strings.Contains(line, "(Director-set)") && strings.Contains(line, "delete") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("In Scope missing delete line for thnc.md:\n%s", inScope)
	}
}

// AC116 AT11 — Resolve errors when invoked on a Q already resolved
// (idempotency-as-error per Q4).
func TestAC116_AT11_IdempotencyError(t *testing.T) {
	dir := t.TempDir()
	acPath := writeAC116Fixture(t, dir)

	if err := Resolve(acPath, 1, "preserve", EmbeddedFS); err != nil {
		t.Fatalf("first Resolve: %v", err)
	}
	err := Resolve(acPath, 1, "sync", EmbeddedFS)
	if err == nil || !strings.Contains(err.Error(), "already resolved") {
		t.Errorf("expected already-resolved error on second Resolve, got: %v", err)
	}
}

// AC116 AT12 — Resolution protocol section in drift-scan.md has imperative
// MUST + named failure mode (structural per AC115 lesson).
func TestAC116_AT12_ResolutionProtocolStructural(t *testing.T) {
	data, err := os.ReadFile("../../docs/drift-scan.md")
	if err != nil {
		t.Fatalf("read drift-scan.md: %v", err)
	}
	body, ok := sectionBody(string(data), "Resolution protocol")
	if !ok {
		t.Fatal("missing ## Resolution protocol section")
	}
	if !strings.Contains(body, "MUST") {
		t.Error("Resolution protocol missing imperative MUST token")
	}
	if !strings.Contains(body, "**Failure mode:**") {
		t.Error("Resolution protocol missing **Failure mode:** substring")
	}
}
