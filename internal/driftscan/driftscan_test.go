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
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0", RepoName: "my-override"}

	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	report := mustRead(t, filepath.Join(dir, "drift-report-abcdef0.md"))
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
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	report := mustRead(t, filepath.Join(dir, "drift-report-abcdef0.md"))
	if !strings.Contains(report, "### `plan.md` — expected-divergence") {
		t.Errorf("expected plan.md classified as expected-divergence, got:\n%s", report)
	}
}

// Sanity check that the report is valid markdown headed by the right title.
func TestReportShape(t *testing.T) {
	r := Report{
		Header: ReportHeader{Invocation: "test", CanonSHA: "abcdef0", Target: "/tmp/x", Flavor: "doc", RepoName: "x"},
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
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}

	out := captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	if !strings.Contains(out, "missing-in-target") {
		t.Errorf("stdout summary missing counts (expected missing-in-target), got:\n%s", out)
	}
	report := mustRead(t, filepath.Join(dir, "drift-report-abcdef0.md"))
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

// Missing-in-target with non-empty canon must route into ## In Scope as
// `create from canon`, AND get a detail subsection with content preview.

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

// AC106 Class I: `target-has-no-canon` files emit a Director Review Q
// with keep/delete/migrate-to-canon options. Closes the decision-surface
// coverage gap — every non-terminal classification must pair with a Q.
func TestClassP_AGENTSMdInRegistry(t *testing.T) {
	if !formatDefiningCanonPaths["AGENTS.md"] {
		t.Error("AC108 AT4: formatDefiningCanonPaths must contain AGENTS.md after Class P registry broadening")
	}
}

// AC108 AT5 — When AGENTS.md is divergent, `### Format-defining file routing`
// block lists it.
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
	report := mustRead(t, filepath.Join(dir, "drift-report-abcdef0.md"))

	// cmd/foo/color.go should appear under target-has-no-canon (via name-reference).
	if !strings.Contains(report, "`cmd/foo/color.go`") {
		t.Errorf("AC112 Class Z: name-referenced target-only `cmd/foo/color.go` must surface in drift report, got:\n%s", report)
	}
	if !strings.Contains(report, "target-has-no-canon") {
		t.Errorf("AC112 Class Z: report must classify color.go as target-has-no-canon, got:\n%s", report)
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
	report := mustRead(t, filepath.Join(dir, "drift-report-abcdef0.md"))

	// main.go is in canon — must not appear under target-has-no-canon classification.
	if strings.Contains(report, "### `cmd/rel/main.go` — target-has-no-canon") {
		t.Errorf("AC112 Class Z: canon-resident main.go must NOT classify as target-has-no-canon, got:\n%s", report)
	}
}

// =====================================================================
// AC119 — Drift-scan retrench: report-pair emission
// =====================================================================

// AC119 AT1 — Two report files emitted at consumer root.
func TestAC119_ReportPairEmitted(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) {
		if exit, _ := Run(cfg, EmbeddedFS, f); exit != ExitOK {
			t.Errorf("expected ExitOK, got %d", exit)
		}
	})
	for _, name := range []string{"drift-report-abcdef0.md", "drift-report-abcdef0-diffs.md"} {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("AC119 AT1: expected %s at consumer root, got: %v", name, err)
		}
		if info.Size() == 0 {
			t.Errorf("AC119 AT1: %s is empty", name)
		}
	}
}

// AC119 AT2 — Report file 1 carries header + per-file blocks + format-defining flag.
func TestAC119_Report1Shape(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) { Run(cfg, EmbeddedFS, f) })
	report := mustRead(t, filepath.Join(dir, "drift-report-abcdef0.md"))
	for _, want := range []string{
		"# Drift-Scan Report",
		"- Canon: governa @ abcdef0",
		"- Flavor: doc",
		"- Counts: ",
		"## Files",
		"### `AGENTS.md`",
		"Format-defining: yes", // AGENTS.md is in formatDefiningCanonPaths
	} {
		if !strings.Contains(report, want) {
			t.Errorf("AC119 AT2: report file 1 missing %q. got:\n%s", want, report)
		}
	}
}

// AC119 AT3 — Report file 2 carries the convention stamp + per-file H2 sections.
func TestAC119_Report2Shape(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) { Run(cfg, EmbeddedFS, f) })
	diffs := mustRead(t, filepath.Join(dir, "drift-report-abcdef0-diffs.md"))
	for _, want := range []string{
		"# Drift-Scan Diffs (governa @ abcdef0)",
		"_Diff convention: `+` lines exist in TARGET; `-` lines exist in CANON.",
	} {
		if !strings.Contains(diffs, want) {
			t.Errorf("AC119 AT3: diffs file missing %q. got:\n%s", want, diffs)
		}
	}
}

// AC119 AT4 — Run() does NOT stage an AC under <target>/docs/ and does NOT
// modify plan.md.
func TestAC119_NoStaging(t *testing.T) {
	dir := docFixture(t)
	planBefore := mustRead(t, filepath.Join(dir, "plan.md"))
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) { Run(cfg, EmbeddedFS, f) })

	// No new AC files under docs/.
	matches, _ := filepath.Glob(filepath.Join(dir, "docs/ac*-drift-scan-from-*.md"))
	if len(matches) != 0 {
		t.Errorf("AC119 AT4: expected no staged AC under docs/, got %v", matches)
	}
	// plan.md unchanged.
	planAfter := mustRead(t, filepath.Join(dir, "plan.md"))
	if planBefore != planAfter {
		t.Errorf("AC119 AT4: plan.md must not be modified.\nbefore:\n%s\nafter:\n%s", planBefore, planAfter)
	}
}

// AC119 AT13 — Idempotent re-scan: invoking Run() twice against the same
// canon SHA produces identical files (overwrite, no append, no error).
func TestAC119_IdempotentRescan(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideSHA: "abcdef0"}
	captureOut(t, func(f *os.File) {
		if exit, _ := Run(cfg, EmbeddedFS, f); exit != ExitOK {
			t.Fatalf("first run: expected ExitOK, got %d", exit)
		}
	})
	report1 := mustRead(t, filepath.Join(dir, "drift-report-abcdef0.md"))
	captureOut(t, func(f *os.File) {
		if exit, _ := Run(cfg, EmbeddedFS, f); exit != ExitOK {
			t.Fatalf("second run: expected ExitOK, got %d", exit)
		}
	})
	report2 := mustRead(t, filepath.Join(dir, "drift-report-abcdef0.md"))
	if report1 != report2 {
		t.Errorf("AC119 AT13: re-scan produced different report (overwrite must be idempotent)")
	}
}
