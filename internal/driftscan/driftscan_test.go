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
// introduced for the sister diffs file.
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

// Repo name override propagates into the report file 1 header.
func TestRepoNameResolution(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test", RepoName: "my-override"}

	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	report := mustRead(t, filepath.Join(dir, "drift-report-v0.0.0-test.md"))
	if !strings.Contains(report, "Repo name: my-override") {
		t.Errorf("expected override in report file, got:\n%s", report)
	}
}

// AT4 (subset) — Numbering computation.
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
func TestJSONOutput(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, JSON: true, OverrideCanonID: "v0.0.0-test"}
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

	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
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
	cfg := Config{Target: "/no/such/dir", Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
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
// `match` label used pre-fix).
func TestPlanMdExpectedDivergence(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	report := mustRead(t, filepath.Join(dir, "drift-report-v0.0.0-test.md"))
	if !strings.Contains(report, "### `plan.md` — expected-divergence") {
		t.Errorf("expected plan.md classified as expected-divergence, got:\n%s", report)
	}
}

// Sanity check that the report is valid markdown headed by the right title.
func TestReportShape(t *testing.T) {
	r := Report{
		Header: ReportHeader{Invocation: "test", CanonSHA: "v0.0.0-test", Target: "/tmp/x", Flavor: "doc", RepoName: "x"},
	}
	var buf bytes.Buffer
	writeReport(&buf, r, false)
	if !strings.HasPrefix(buf.String(), "# Drift-Scan Report") {
		t.Errorf("expected report header, got: %s", buf.String()[:50])
	}
}

// Counts tally line appears in the report file 1 header and the stdout summary.
func TestCountsTallyLine(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}

	out := captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	if !strings.Contains(out, "missing-in-target") {
		t.Errorf("stdout summary missing counts (expected missing-in-target), got:\n%s", out)
	}
	report := mustRead(t, filepath.Join(dir, "drift-report-v0.0.0-test.md"))
	if !strings.Contains(report, "- Counts: ") {
		t.Errorf("report file missing Counts line, got:\n%s", report)
	}
	if !strings.Contains(report, "missing-in-target") {
		t.Errorf("report Counts line missing missing-in-target, got:\n%s", report)
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

// format-defining files in the registry are auto-routed
// to ## In Scope as sync (regardless of raw classification), suppressed
// from Director Review, and named under ### Format-defining file routing
// with rationale.
func TestCanonCoherenceHardFailEmitsReport(t *testing.T) {
	// Use the live canon — it's coherent after reconciliation.
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
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	out := captureOut(t, func(f *os.File) {
		exit, _ := Run(cfg, EmbeddedFS, f)
		if exit == ExitOK {
			t.Errorf("expected non-zero exit on canon-coherence failure, got ExitOK")
		}
	})
	if !strings.Contains(out, "# Canon-Coherence Precondition Failed") {
		t.Errorf("expected `# Canon-Coherence Precondition Failed` H1 on stdout, got:\n%s", out)
	}
	if !strings.Contains(out, "**governa-side**") {
		t.Errorf("expected governa-side framing in report, got:\n%s", out)
	}
	// No target writes.
	staged := findStagedACs(t, dir)
	if len(staged) > 0 {
		t.Errorf("expected no staged ACs after hard-fail, got %v", staged)
	}
}

// `target-has-no-canon` files emit a Director Review Q
// with keep/delete/migrate-to-canon options. Closes the decision-surface
// coverage gap — every non-terminal classification must pair with a Q.
func TestAGENTSMdRegisteredAsFormatDefining(t *testing.T) {
	if !formatDefiningCanonPaths["AGENTS.md"] {
		t.Error("formatDefiningCanonPaths must contain AGENTS.md")
	}
}

// When AGENTS.md is divergent, `### Format-defining file routing`
// block lists it.
func TestNameReferenceSurfacesTargetOnlyFile(t *testing.T) {
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
	gitAddCommit(t, dir, "rel.sh + color.go")

	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	report := mustRead(t, filepath.Join(dir, "drift-report-v0.0.0-test.md"))

	// cmd/foo/color.go should appear under target-has-no-canon (via name-reference).
	if !strings.Contains(report, "`cmd/foo/color.go`") {
		t.Errorf("name-referenced target-only `cmd/foo/color.go` must surface in drift report, got:\n%s", report)
	}
	if !strings.Contains(report, "target-has-no-canon") {
		t.Errorf("report must classify color.go as target-has-no-canon, got:\n%s", report)
	}
}

// name-reference scan does not false-positive on canon-resident refs.
func TestNameReferenceNoFalsePositiveOnCanonResidentRef(t *testing.T) {
	// rel.sh references ./cmd/rel/main.go which IS in canon (DOC overlay).
	// Should NOT trigger target-has-no-canon for main.go.
	dir := docFixture(t)
	mustWrite(t, filepath.Join(dir, "rel.sh"), "#!/usr/bin/env bash\nexec go run ./cmd/rel/main.go \"$@\"\n")
	mustWrite(t, filepath.Join(dir, "cmd/rel/main.go"), "package main\nfunc main() {}\n")
	gitAddCommit(t, dir, "rel.sh refs canon-resident main.go")

	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	report := mustRead(t, filepath.Join(dir, "drift-report-v0.0.0-test.md"))

	// main.go is in canon — must not appear under target-has-no-canon classification.
	if strings.Contains(report, "### `cmd/rel/main.go` — target-has-no-canon") {
		t.Errorf("canon-resident main.go must NOT classify as target-has-no-canon, got:\n%s", report)
	}
}

// =====================================================================
// Historical: Drift-scan retrench: report-pair emission
// =====================================================================

// Two report files emitted at consumer root.
func TestReportPairEmittedAtConsumerRoot(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	captureOut(t, func(f *os.File) {
		if exit, _ := Run(cfg, EmbeddedFS, f); exit != ExitOK {
			t.Errorf("expected ExitOK, got %d", exit)
		}
	})
	for _, name := range []string{"drift-report-v0.0.0-test.md", "drift-report-v0.0.0-test-diffs.md"} {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected %s at consumer root, got: %v", name, err)
		}
		if info.Size() == 0 {
			t.Errorf("%s is empty", name)
		}
	}
}

// Report file 1 carries header + per-file blocks + format-defining flag.
func TestReport1HeaderAndPerFileBlocks(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	captureOut(t, func(f *os.File) { Run(cfg, EmbeddedFS, f) })
	report := mustRead(t, filepath.Join(dir, "drift-report-v0.0.0-test.md"))
	for _, want := range []string{
		"# Drift-Scan Report",
		"- Canon: governa @ v0.0.0-test",
		"- Flavor: doc",
		"- Counts: ",
		"## Files",
		"### `AGENTS.md`",
		"Format-defining: yes", // AGENTS.md is in formatDefiningCanonPaths
	} {
		if !strings.Contains(report, want) {
			t.Errorf("report file 1 missing %q. got:\n%s", want, report)
		}
	}
}

// Report file 2 carries the convention stamp + per-file H2 sections.
func TestReport2DiffsHeaderAndConvention(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	captureOut(t, func(f *os.File) { Run(cfg, EmbeddedFS, f) })
	diffs := mustRead(t, filepath.Join(dir, "drift-report-v0.0.0-test-diffs.md"))
	for _, want := range []string{
		"# Drift-Scan Diffs (governa @ v0.0.0-test)",
		"Diff convention: `+` lines exist in TARGET; `-` lines exist in CANON.",
	} {
		if !strings.Contains(diffs, want) {
			t.Errorf("diffs file missing %q. got:\n%s", want, diffs)
		}
	}
}

// Run() does NOT stage an AC under <target>/docs/ and does NOT
// modify plan.md.
func TestRunDoesNotStageACOrModifyPlan(t *testing.T) {
	dir := docFixture(t)
	planBefore := mustRead(t, filepath.Join(dir, "plan.md"))
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	captureOut(t, func(f *os.File) { Run(cfg, EmbeddedFS, f) })

	// No new AC files under docs/.
	matches, _ := filepath.Glob(filepath.Join(dir, "docs/ac*-drift-scan-from-*.md"))
	if len(matches) != 0 {
		t.Errorf("expected no staged AC under docs/, got %v", matches)
	}
	// plan.md unchanged.
	planAfter := mustRead(t, filepath.Join(dir, "plan.md"))
	if planBefore != planAfter {
		t.Errorf("plan.md must not be modified.\nbefore:\n%s\nafter:\n%s", planBefore, planAfter)
	}
}

// Idempotent re-scan: invoking Run() twice against the same
// canon SHA produces identical files (overwrite, no append, no error).
func TestRescanOverwritesIdempotently(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	captureOut(t, func(f *os.File) {
		if exit, _ := Run(cfg, EmbeddedFS, f); exit != ExitOK {
			t.Fatalf("first run: expected ExitOK, got %d", exit)
		}
	})
	report1 := mustRead(t, filepath.Join(dir, "drift-report-v0.0.0-test.md"))
	captureOut(t, func(f *os.File) {
		if exit, _ := Run(cfg, EmbeddedFS, f); exit != ExitOK {
			t.Fatalf("second run: expected ExitOK, got %d", exit)
		}
	})
	report2 := mustRead(t, filepath.Join(dir, "drift-report-v0.0.0-test.md"))
	if report1 != report2 {
		t.Errorf("re-scan produced different report (overwrite must be idempotent)")
	}
}

// =====================================================================
// Historical: Report cleanups: asymmetry note + symmetric diffs + stamp
// =====================================================================

// A missing-in-target file appears in the diffs file with
// a unified diff against empty target (canon lines as `-`).
func TestMissingInTargetSurfacesInDiffsFile(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	captureOut(t, func(f *os.File) { Run(cfg, EmbeddedFS, f) })
	diffs := mustRead(t, filepath.Join(dir, "drift-report-v0.0.0-test-diffs.md"))

	// docFixture's canon includes .gitignore. Target lacks it.
	if !strings.Contains(diffs, "## `.gitignore`") {
		t.Errorf("missing-in-target file (.gitignore) absent from diffs file:\n%s", diffs)
	}
	// Find the .gitignore section and verify it carries `-` lines (canon-only content).
	idx := strings.Index(diffs, "## `.gitignore`")
	if idx < 0 {
		t.Fatal("section not found")
	}
	tail := diffs[idx:]
	end := strings.Index(tail[len("## `.gitignore`"):], "\n## ")
	var section string
	if end < 0 {
		section = tail
	} else {
		section = tail[:len("## `.gitignore`")+end]
	}
	if !strings.Contains(section, "\n-") {
		t.Errorf(".gitignore section must carry `-` lines (canon-only). got:\n%s", section)
	}
}

// A target-has-no-canon file (name-reference branch) appears
// in the diffs file with a unified diff against empty canon (target lines as `+`).
func TestTargetHasNoCanonSurfacesInDiffsFile(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS.md\n")
	mustWrite(t, filepath.Join(dir, "plan.md"), "# Plan\n\n## Ideas To Explore\n\n- IE1: existing\n")
	mustWrite(t, filepath.Join(dir, "CHANGELOG.md"), "# Changelog\n\n| Version | Summary |\n|---|---|\n| Unreleased | |\n")
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "docs/ac-template.md"), "# AC template\n")
	// Divergent rel.sh referencing target-only color.go (name-reference fixture).
	mustWrite(t, filepath.Join(dir, "rel.sh"), "#!/usr/bin/env bash\nexec go run ./cmd/foo/main.go ./cmd/foo/color.go \"$@\"\n")
	mustWrite(t, filepath.Join(dir, "cmd/foo/main.go"), "package main\nfunc main() {}\n")
	mustWrite(t, filepath.Join(dir, "cmd/foo/color.go"), "package main\n\nfunc col() string { return \"yellow\" }\n")
	gitInit(t, dir)
	gitAddCommit(t, dir, "rel.sh + color.go")

	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	captureOut(t, func(f *os.File) { Run(cfg, EmbeddedFS, f) })
	diffs := mustRead(t, filepath.Join(dir, "drift-report-v0.0.0-test-diffs.md"))

	if !strings.Contains(diffs, "## `cmd/foo/color.go`") {
		t.Errorf("target-has-no-canon file (cmd/foo/color.go) absent from diffs file:\n%s", diffs)
	}
	idx := strings.Index(diffs, "## `cmd/foo/color.go`")
	if idx < 0 {
		t.Fatal("section not found")
	}
	tail := diffs[idx:]
	end := strings.Index(tail[len("## `cmd/foo/color.go`"):], "\n## ")
	var section string
	if end < 0 {
		section = tail
	} else {
		section = tail[:len("## `cmd/foo/color.go`")+end]
	}
	if !strings.Contains(section, "\n+") {
		t.Errorf("cmd/foo/color.go section must carry `+` lines (target-only). got:\n%s", section)
	}
}

// =====================================================================
// Historical: Per-file Direction line in diffs file (diff-direction integrity mitigation)
// =====================================================================

// // Direction line emitted per-file in the diffs file.
// docFixture has missing-in-target (.gitignore: canon-only → canon leads).
// Add a target-has-no-canon file via the name-reference fixture pattern (target
// content via name-reference from divergent rel.sh → target leads).
func TestDirectionLineEmittedPerFile(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS.md\n")
	mustWrite(t, filepath.Join(dir, "plan.md"), "# Plan\n\n## Ideas To Explore\n\n- IE1: existing\n")
	mustWrite(t, filepath.Join(dir, "CHANGELOG.md"), "# Changelog\n\n| Version | Summary |\n|---|---|\n| Unreleased | |\n")
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "docs/ac-template.md"), "# AC template\n")
	mustWrite(t, filepath.Join(dir, "rel.sh"), "#!/usr/bin/env bash\nexec go run ./cmd/foo/main.go ./cmd/foo/color.go \"$@\"\n")
	mustWrite(t, filepath.Join(dir, "cmd/foo/main.go"), "package main\nfunc main() {}\n")
	mustWrite(t, filepath.Join(dir, "cmd/foo/color.go"), "package main\n\nfunc col() string { return \"yellow\" }\n")
	gitInit(t, dir)
	gitAddCommit(t, dir, "rel.sh + color.go")

	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	captureOut(t, func(f *os.File) { Run(cfg, EmbeddedFS, f) })
	diffs := mustRead(t, filepath.Join(dir, "drift-report-v0.0.0-test-diffs.md"))

	// Each per-file H2 section should carry a Direction line.
	if !strings.Contains(diffs, "Direction: ") {
		t.Errorf("diffs file missing Direction lines:\n%s", diffs)
	}
	// rel.sh diverges (target +1 / canon -1) → mutual form (target carries N lines absent in canon; canon carries M lines absent in target).
	// .gitignore is missing-in-target (canon has content; target empty) → canon leads.
	gitignoreIdx := strings.Index(diffs, "## `.gitignore`")
	if gitignoreIdx < 0 {
		t.Fatal(".gitignore section not found in diffs file")
	}
	gitignoreSection := diffs[gitignoreIdx:]
	if !strings.Contains(gitignoreSection[:200], "Direction: canon leads") {
		t.Errorf(".gitignore section must carry `Direction: canon leads` near the heading. got:\n%s", gitignoreSection[:min(len(gitignoreSection), 400)])
	}
	// cmd/foo/color.go is target-has-no-canon (target has content; canon empty) → target leads.
	colorIdx := strings.Index(diffs, "## `cmd/foo/color.go`")
	if colorIdx < 0 {
		t.Fatal("cmd/foo/color.go section not found in diffs file")
	}
	colorSection := diffs[colorIdx:]
	if !strings.Contains(colorSection[:200], "Direction: target leads") {
		t.Errorf("cmd/foo/color.go section must carry `Direction: target leads` near the heading. got:\n%s", colorSection[:min(len(colorSection), 400)])
	}
}

// codeFixture creates a code-flavor target dir with a Go module manifest,
// minimal scaffolding, and a one-commit history. Module path is NOT
// github.com/queone/governa, and internal/templates/base/ is absent, so
// the governa-self fail-safe does not trigger.
func codeFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module github.com/example/test\n\ngo 1.22\n")
	mustWrite(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS.md\n")
	mustWrite(t, filepath.Join(dir, "plan.md"), "# Plan\n\n## Ideas To Explore\n\n- IE1: existing\n")
	mustWrite(t, filepath.Join(dir, "CHANGELOG.md"), "# Changelog\n\n| Version | Summary |\n|---|---|\n| Unreleased | |\n| 0.1.0 | initial |\n")
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "docs/ac-template.md"), "# AC template\n")
	gitInit(t, dir)
	gitAddCommit(t, dir, "initial")
	return dir
}

// AT2 — Code-flavor drift-scan report header carries the reachability
// reminder. Substring assertion against ReachabilityHeaderReminder
// (referenced as the constant, not a hardcoded string).
func TestReachabilityReminderEmittedInCodeFlavorHeader(t *testing.T) {
	dir := codeFixture(t)
	cfg := Config{Target: dir, Flavor: "code", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}

	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	report := mustRead(t, filepath.Join(dir, "drift-report-v0.0.0-test.md"))
	if !strings.Contains(report, ReachabilityHeaderReminder) {
		t.Errorf("expected ReachabilityHeaderReminder in code-flavor report header, got:\n%s", report)
	}
}

// AT3 — Doc-flavor drift-scan report header has no line beginning with
// `Reachability check:`. Stronger than asserting the exact reminder is
// absent: closes the escape hatch where a future AC adds a different
// `Reachability check: ...` line for doc flavor.
func TestNoReachabilityLineInDocFlavorHeader(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}

	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	report := mustRead(t, filepath.Join(dir, "drift-report-v0.0.0-test.md"))
	header, _, ok := strings.Cut(report, "## Files")
	if !ok {
		t.Fatalf("report missing `## Files` heading, got:\n%s", report)
	}
	for line := range strings.SplitSeq(header, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Reachability check:") {
			t.Errorf("doc-flavor header contains forbidden line: %q\nfull header:\n%s", line, header)
		}
	}
}

// AT4 — Drift-scan classification behavior is unchanged. The reachability
// reminder is a header-level addition only; no per-file flag, no new
// FileResult field. Per Round 2 disposition (registry rejected), no
// `Host-shape-dependent:` line should ever appear in the rendered report.
func TestClassificationUnaffectedByReachabilityReminder(t *testing.T) {
	dir := codeFixture(t)
	cfg := Config{Target: dir, Flavor: "code", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}

	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	report := mustRead(t, filepath.Join(dir, "drift-report-v0.0.0-test.md"))
	if strings.Contains(report, "Host-shape-dependent:") {
		t.Errorf("unexpected `Host-shape-dependent:` flag in report (per-file flag is out of scope per Round 2 disposition):\n%s", report)
	}
	if !strings.Contains(report, "- Counts: ") {
		t.Errorf("report missing Counts line — classification path may be broken:\n%s", report)
	}
}

// AT5 — JSON output is markdown-only-out-of-scope (Round 10): the
// reachability reminder is a human-targeted nudge; JSON consumers are
// tools. JSON output must NOT contain the `Reachability check:` prefix or
// any `reachability_header_reminder` field. Future PRs proposing a struct
// field for symmetry-with-markdown should be rejected unless the
// audience-boundary justification has changed.
func TestNoReachabilityReminderInJSONOutput(t *testing.T) {
	dir := codeFixture(t)
	cfg := Config{Target: dir, Flavor: "code", DiffLines: 50, JSON: true, OverrideCanonID: "v0.0.0-test"}
	out := captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	if strings.Contains(out, "Reachability check:") {
		t.Errorf("JSON output unexpectedly contains `Reachability check:` line; markdown-only scope per Round 10 disposition:\n%s", out)
	}
	if strings.Contains(out, "reachability_header_reminder") {
		t.Errorf("JSON output unexpectedly contains `reachability_header_reminder` field; markdown-only scope per Round 10 disposition:\n%s", out)
	}
}

// Main drift-report file carries the cleanup-after-AC instruction. Test
// references CleanupReminder directly (not hardcoded), so editing the
// constant without updating the emission breaks this assertion.
func TestCleanupReminderInMainReport(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	report := mustRead(t, filepath.Join(dir, "drift-report-v0.0.0-test.md"))
	if !strings.Contains(report, CleanupReminder) {
		t.Errorf("expected CleanupReminder in main drift-report, got:\n%s", report)
	}
}

// Diffs drift-report file carries the cleanup-after-AC instruction.
func TestCleanupReminderInDiffsReport(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	diffs := mustRead(t, filepath.Join(dir, "drift-report-v0.0.0-test-diffs.md"))
	if !strings.Contains(diffs, CleanupReminder) {
		t.Errorf("expected CleanupReminder in diffs drift-report, got:\n%s", diffs)
	}
}

// Stdout summary carries no `Cleanup:` prefix line — audience boundary:
// stdout is the tool-confirmation channel for invokers, not a
// workflow-instruction surface.
func TestNoCleanupLineInStdoutSummary(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	out := captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	for line := range strings.SplitSeq(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Cleanup:") {
			t.Errorf("stdout summary contains forbidden `Cleanup:` line: %q\nfull stdout:\n%s", line, out)
		}
	}
}

// JSON output is markdown-only-out-of-scope — cleanup instruction is a
// human-targeted nudge; JSON consumers are tools.
func TestNoCleanupReminderInJSONOutput(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, JSON: true, OverrideCanonID: "v0.0.0-test"}
	out := captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	if strings.Contains(out, CleanupReminder) {
		t.Errorf("JSON output unexpectedly contains CleanupReminder; markdown-only scope:\n%s", out)
	}
	if strings.Contains(out, "cleanup_reminder") {
		t.Errorf("JSON output unexpectedly contains `cleanup_reminder` field; markdown-only scope:\n%s", out)
	}
}
