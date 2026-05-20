package driftscan

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/queone/governa/internal/emission"
	"github.com/queone/governa/internal/governance"
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
// drift-scan staging pattern. The `-diffs.md` suffix filter is retained as
// defense-in-depth; AC139 retired the sister-diffs emission so no files
// match the suffix in current runs.
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

// (Removed under AC136: RepoName is not emitted as a header field in the new AC-stub format.)

// AT4 (subset) — Numbering computation.
func TestPreserveMarker(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "CHANGELOG.md"),
		"| 0.1.0 | preserve docs/foo.md customization |\n",
	)
	hits := emission.PreserveMarkers(dir, "docs/foo.md")
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
	mustWrite(t, filepath.Join(dir, "docs/ac-template.md"), "# AC template\n")
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
	hits := emission.PreserveMarkers(dir, "docs/foo.md")
	if len(hits) != 0 {
		t.Errorf("expected 0 hits for prose, got %d: %v", len(hits), hits)
	}

	// Sanity: the same phrase as a CHANGELOG table row IS detected.
	mustWrite(t, filepath.Join(dir, "CHANGELOG.md"),
		"| 0.1.0 | preserve docs/foo.md customization |\n",
	)
	hits = emission.PreserveMarkers(dir, "docs/foo.md")
	if len(hits) != 1 {
		t.Errorf("expected 1 hit for table row, got %d: %v", len(hits), hits)
	}

	// Sanity: as a CHANGELOG cell separator (`; preserve <path>`).
	mustWrite(t, filepath.Join(dir, "CHANGELOG.md"),
		"| 0.1.0 | AC1: did stuff; preserve docs/foo.md customization |\n",
	)
	hits = emission.PreserveMarkers(dir, "docs/foo.md")
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
// as expected-divergence and appear in the AC stub's ## Out Of Scope section.
func TestPlanMdExpectedDivergence(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	stub := mustRead(t, filepath.Join(dir, "docs/ac1-drift-scan-v0.0.0-test.md"))
	if !strings.Contains(stub, "`plan.md` — expected-divergence") {
		t.Errorf("expected plan.md classified as expected-divergence in stub, got:\n%s", stub)
	}
}

// arch.md is a per-repo stub like plan.md. It must classify as
// expected-divergence and appear in the AC stub's ## Out Of Scope section.
func TestArchMdExpectedDivergence(t *testing.T) {
	dir := codeFixture(t)
	mustWrite(t, filepath.Join(dir, "arch.md"), "# arch\n\nrepo-specific content\n")
	cfg := Config{Target: dir, Flavor: "code", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	captureOut(t, func(f *os.File) {
		Run(cfg, EmbeddedFS, f)
	})
	stub := mustRead(t, filepath.Join(dir, "docs/ac1-drift-scan-v0.0.0-test.md"))
	if !strings.Contains(stub, "`arch.md` — expected-divergence") {
		t.Errorf("expected arch.md classified as expected-divergence in stub, got:\n%s", stub)
	}
}

// (Removed under AC136: TestReportShape and TestCountsTallyLine asserted
// the old report-pair format that AC136 replaced with AC-stub emission.)

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
// from Routing Decisions, and named under ### Format-defining file routing
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

// `target-has-no-canon` files emit a routing question
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
	report := mustRead(t, filepath.Join(dir, "docs/ac1-drift-scan-v0.0.0-test.md"))

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
	report := mustRead(t, filepath.Join(dir, "docs/ac1-drift-scan-v0.0.0-test.md"))

	// main.go is in canon — must not appear under target-has-no-canon classification.
	if strings.Contains(report, "### `cmd/rel/main.go` — target-has-no-canon") {
		t.Errorf("canon-resident main.go must NOT classify as target-has-no-canon, got:\n%s", report)
	}
}

// =====================================================================
// AC136: AC-stub + sister-diffs emission under <target>/docs/
// =====================================================================

// AC139: drift-scan emits exactly the AC stub under <target>/docs/; the
// retired *-diffs.md sibling file must NOT be emitted.
func TestACStubEmittedNoDiffsSibling(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	captureOut(t, func(f *os.File) {
		if exit, _ := Run(cfg, EmbeddedFS, f); exit != ExitOK {
			t.Errorf("expected ExitOK, got %d", exit)
		}
	})
	stub := filepath.Join(dir, "docs/ac1-drift-scan-v0.0.0-test.md")
	info, err := os.Stat(stub)
	if err != nil {
		t.Fatalf("expected AC stub at %s, got: %v", stub, err)
	}
	if info.Size() == 0 {
		t.Errorf("AC stub is empty")
	}
	// AC139: no diffs sibling.
	if _, err := os.Stat(filepath.Join(dir, "docs/ac1-drift-scan-v0.0.0-test-diffs.md")); err == nil {
		t.Errorf("diffs sibling file must not be emitted under AC139 (retired)")
	}
	// Old root-level report-pair must NOT exist.
	for _, name := range []string{"drift-report-v0.0.0-test.md", "drift-report-v0.0.0-test-diffs.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("legacy %s should not be emitted", name)
		}
	}
}

// AT7 (subset): AC stub conforms to ac-template skeleton.
func TestACStubConformsToTemplate(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	captureOut(t, func(f *os.File) { Run(cfg, EmbeddedFS, f) })
	stub := mustRead(t, filepath.Join(dir, "docs/ac1-drift-scan-v0.0.0-test.md"))
	for _, want := range []string{
		"# AC1 Drift-Scan Adoption from governa v0.0.0-test",
		"## Summary",
		"## In Scope",
		"## Out Of Scope",
		"## Acceptance Tests",
		"## Status",
		"`PENDING`",
	} {
		if !strings.Contains(stub, want) {
			t.Errorf("AC stub missing %q. got:\n%s", want, stub)
		}
	}
	for _, absent := range []string{
		"## Objective Fit",
		"## Director Review",
		"## Implementation Notes",
		"## Documentation Updates",
		"## Critique",
	} {
		if strings.Contains(stub, absent) {
			t.Errorf("AC stub must not contain %q (retired in AC138). got:\n%s", absent, stub)
		}
	}
}

// AT8 (subset): re-running against same canon version on unedited stub
// is idempotent — the AC stub overwrites in place with identical content.
func TestRescanOverwritesIdempotentlyOnUnedited(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	captureOut(t, func(f *os.File) {
		if exit, _ := Run(cfg, EmbeddedFS, f); exit != ExitOK {
			t.Fatalf("first run: expected ExitOK, got %d", exit)
		}
	})
	stub1 := mustRead(t, filepath.Join(dir, "docs/ac1-drift-scan-v0.0.0-test.md"))
	captureOut(t, func(f *os.File) {
		if exit, _ := Run(cfg, EmbeddedFS, f); exit != ExitOK {
			t.Fatalf("second run: expected ExitOK, got %d", exit)
		}
	})
	stub2 := mustRead(t, filepath.Join(dir, "docs/ac1-drift-scan-v0.0.0-test.md"))
	if stub1 != stub2 {
		t.Errorf("re-scan produced different stub (overwrite must be idempotent on unedited)")
	}
}

// AT15: edit-detection guard refuses overwrite on edited stub.
func TestEditDetectionRefusesOverwrite(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	captureOut(t, func(f *os.File) {
		if exit, _ := Run(cfg, EmbeddedFS, f); exit != ExitOK {
			t.Fatalf("first run: expected ExitOK, got %d", exit)
		}
	})
	// Edit the stub body without rewriting the marker.
	stubPath := filepath.Join(dir, "docs/ac1-drift-scan-v0.0.0-test.md")
	original := mustRead(t, stubPath)
	edited := original + "\n\nedited by hand\n"
	if err := os.WriteFile(stubPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	// Re-run must refuse.
	exit, err := Run(cfg, EmbeddedFS, devNull(t))
	if exit == ExitOK {
		t.Errorf("expected non-zero exit on edited-stub re-run, got ExitOK")
	}
	if err == nil || !strings.Contains(err.Error(), "edited since last drift-scan emission") {
		t.Errorf("expected edit-detection error, got: %v", err)
	}
	// Stub should still carry the edited content (no overwrite).
	after := mustRead(t, stubPath)
	if after != edited {
		t.Errorf("edited stub was overwritten — refuse path failed")
	}
}

// AT10 (subset): --json emission includes the emitted file paths.
func TestJSONIncludesEmittedPaths(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, JSON: true, OverrideCanonID: "v0.0.0-test"}
	out := captureOut(t, func(f *os.File) { Run(cfg, EmbeddedFS, f) })
	var r Report
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if r.Emitted == nil {
		t.Fatalf("JSON output missing emitted-paths block: %s", out)
	}
	if want := "docs/ac1-drift-scan-v0.0.0-test.md"; r.Emitted.ACStub != want {
		t.Errorf("emitted.ac_stub = %q, want %q", r.Emitted.ACStub, want)
	}
}

// Format-defining files override raw classification: any divergence on a
// file in formatDefiningCanonPaths routes to ## In Scope as a sync item
// regardless of whether the raw classification was preserve / ambiguity.
// docFixture's docs/ac-template.md has minimal content (canon is the full
// template); after the single fixture commit, it classifies as ambiguity
// (1 commit, no marker) — without the override it would route to Routing
// Decisions. With override, it must appear in ## In Scope with a
// (format-defining) annotation and not in ### Routing Decisions.
func TestFormatDefiningOverrideRoutesToInScope(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	captureOut(t, func(f *os.File) { Run(cfg, EmbeddedFS, f) })
	stub := mustRead(t, filepath.Join(dir, "docs/ac1-drift-scan-v0.0.0-test.md"))

	// In Scope section must list the format-defining file with annotation.
	inScopeStart := strings.Index(stub, "## In Scope")
	inScopeEnd := strings.Index(stub, "## Out Of Scope")
	if inScopeStart < 0 || inScopeEnd < 0 {
		t.Fatalf("stub missing required sections: %s", stub)
	}
	inScope := stub[inScopeStart:inScopeEnd]
	if !strings.Contains(inScope, "`docs/ac-template.md`") {
		t.Errorf("expected docs/ac-template.md in ## In Scope (format-defining override), got:\n%s", inScope)
	}
	if !strings.Contains(inScope, "(format-defining)") {
		t.Errorf("expected (format-defining) annotation in ## In Scope, got:\n%s", inScope)
	}

	// Routing Decisions subheading (if present) must NOT list the format-
	// defining file — that is the override's whole point.
	if routingStart := strings.Index(stub, "### Routing Decisions"); routingStart >= 0 {
		routingEnd := strings.Index(stub[routingStart:], "## In Scope")
		if routingEnd < 0 {
			routingEnd = len(stub) - routingStart
		}
		routing := stub[routingStart : routingStart+routingEnd]
		if strings.Contains(routing, "`docs/ac-template.md`") {
			t.Errorf("docs/ac-template.md must not appear in ### Routing Decisions (format-defining override failed), got:\n%s", routing)
		}
	}
}

// docs/ is created on emission if missing. Adoption check can pass on
// AGENTS.md + CHANGELOG row alone; emission must still succeed.
func TestEmissionCreatesDocsDir(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS.md\n")
	mustWrite(t, filepath.Join(dir, "CHANGELOG.md"), "# Changelog\n\n| 0.1.0 | initial governa apply |\n")
	gitInit(t, dir)
	gitAddCommit(t, dir, "initial")
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	exit, err := Run(cfg, EmbeddedFS, devNull(t))
	if exit != ExitOK {
		t.Fatalf("expected ExitOK, got %d (err=%v)", exit, err)
	}
	// docs/ should now exist with the AC stub inside.
	if _, err := os.Stat(filepath.Join(dir, "docs/ac1-drift-scan-v0.0.0-test.md")); err != nil {
		t.Errorf("expected AC stub at docs/ac1-drift-scan-v0.0.0-test.md after auto-MkdirAll, got: %v", err)
	}
}

// AT14: adoption check hard-errors when AGENTS.md present without secondary signal.
func TestAdoptionCheckFailsWithoutSignal(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS.md\n")
	gitInit(t, dir)
	gitAddCommit(t, dir, "initial")
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	exit, err := Run(cfg, EmbeddedFS, devNull(t))
	if exit == ExitOK {
		t.Errorf("expected non-zero exit when AGENTS.md present without adoption signal")
	}
	if err == nil || !strings.Contains(err.Error(), "no governa adoption signal") {
		t.Errorf("expected adoption-signal error, got: %v", err)
	}
}

// Plan.md must remain unmodified across drift-scan runs (only the AC stub
// under docs/ is written).
func TestRunDoesNotModifyPlan(t *testing.T) {
	dir := docFixture(t)
	planBefore := mustRead(t, filepath.Join(dir, "plan.md"))
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	captureOut(t, func(f *os.File) { Run(cfg, EmbeddedFS, f) })
	planAfter := mustRead(t, filepath.Join(dir, "plan.md"))
	if planBefore != planAfter {
		t.Errorf("plan.md must not be modified by drift-scan")
	}
}

// (Removed under AC139: TestMissingInTargetSurfacesInDiffsFile,
// TestTargetHasNoCanonSurfacesInDiffsFile, and TestDirectionLineEmittedPerFile
// asserted per-file shape of the sister diffs file. AC139 retired the diffs
// file emission; adopters now use `governa render-canon` + `diff -ru` for
// per-file inspection. Classification routing (missing-in-target,
// target-has-no-canon) is still covered by TestNameReferenceSurfacesTargetOnlyFile
// and the AC-stub-shape tests.)

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
	report := mustRead(t, filepath.Join(dir, "docs/ac1-drift-scan-v0.0.0-test.md"))
	if !strings.Contains(report, ReachabilityHeaderReminder) {
		t.Errorf("expected ReachabilityHeaderReminder in code-flavor report header, got:\n%s", report)
	}
}

// Doc-flavor AC stub does not carry the Reachability reminder.
func TestNoReachabilityLineInDocFlavorStub(t *testing.T) {
	dir := docFixture(t)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	captureOut(t, func(f *os.File) { Run(cfg, EmbeddedFS, f) })
	stub := mustRead(t, filepath.Join(dir, "docs/ac1-drift-scan-v0.0.0-test.md"))
	if strings.Contains(stub, "Reachability check:") {
		t.Errorf("doc-flavor AC stub must not contain `Reachability check:`. got:\n%s", stub)
	}
}

// (Removed under AC136: TestClassificationUnaffectedByReachabilityReminder
// asserted the old "Counts:" header line, which is not part of the new AC
// stub format. Classification correctness is still covered by the
// per-classification tests above.)

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

// (Removed under AC136: CleanupReminder applied to the old disposable
// report-pair. The new emission IS an AC stub the consumer Operator
// iterates on, not a disposable artifact.)

// Stdout summary has no `Cleanup:` line (no reminder is emitted by the new flow).
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

// (Removed under AC136: TestAdoptionReminder* and the Adoption/Cleanup
// reminder constants were tied to the old report-pair format. The new
// AC stub carries adoption guidance through `## Summary` / the emitted
// ATs themselves; the inline reminder constants are no longer emitted.)

// AC142 AT1 — mixedContentBoundary registry contents are exactly the three
// documented entries with the correct boundary headings. Asserts the map
// directly rather than grepping source so the test is robust to formatting.
func TestMixedContentBoundaryRegistry(t *testing.T) {
	want := map[string]string{
		"AGENTS.md":                      "## Project Rules",
		"docs/development-guidelines.md": "## Project Practices",
		"docs/editing-guidelines.md":     "## Project Practices",
	}
	if len(mixedContentBoundary) != len(want) {
		t.Fatalf("registry size: got %d, want %d (registry=%v)", len(mixedContentBoundary), len(want), mixedContentBoundary)
	}
	for k, v := range want {
		got, ok := mixedContentBoundary[k]
		if !ok {
			t.Errorf("registry missing entry %q", k)
			continue
		}
		if got != v {
			t.Errorf("registry[%q] = %q, want %q", k, got, v)
		}
	}
}

// AC142 AT2 — classifyFile on a mixed-content file with byte-identical
// canon-zone bytes (and an arbitrary repo-owned tail) classifies as
// ClassMatch with Boundary populated.
func TestClassifyFileMixedContentMatchOnCanonZone(t *testing.T) {
	dir := t.TempDir()
	canon := "Header line A\nHeader line B\n## Project Rules\n\n- canon rule\n"
	target := "Header line A\nHeader line B\n## Project Rules\n\n- arbitrary local rule\n"
	mustWrite(t, filepath.Join(dir, "AGENTS.md"), target)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50}
	fr := classifyFile(cfg, "AGENTS.md", canon, "v0.0.0-test")
	if fr.Classification != ClassMatch {
		t.Errorf("Classification: got %q, want %q", fr.Classification, ClassMatch)
	}
	if fr.Boundary != "## Project Rules" {
		t.Errorf("Boundary: got %q, want %q", fr.Boundary, "## Project Rules")
	}
	if !strings.Contains(fr.CompareCommand, "canon-zone byte-equal above") {
		t.Errorf("CompareCommand missing canon-zone evidence form: %q", fr.CompareCommand)
	}
}

// AC142 AT3 — classifyFile on a mixed-content file whose canon zone
// differs from canon by one byte falls through to the divergent path
// (ClassClearSync absent git/markers) with Boundary populated so any
// emitted AT can anchor on the canon zone.
func TestClassifyFileMixedContentDriftInCanonZone(t *testing.T) {
	dir := t.TempDir()
	canon := "Header line A\nHeader line B\n## Project Rules\n\n- canon rule\n"
	target := "Header line X\nHeader line B\n## Project Rules\n\n- arbitrary local rule\n"
	mustWrite(t, filepath.Join(dir, "AGENTS.md"), target)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50}
	fr := classifyFile(cfg, "AGENTS.md", canon, "v0.0.0-test")
	if fr.Classification == ClassMatch {
		t.Errorf("Classification: got ClassMatch on canon-zone drift; want divergent")
	}
	if fr.Boundary != "## Project Rules" {
		t.Errorf("Boundary: got %q, want %q (must still be populated for canon-zone AT wording)", fr.Boundary, "## Project Rules")
	}
}

// AC142 AT4 — classifyFile on a mixed-content file whose target lacks the
// boundary heading falls through to whole-file comparison and leaves
// Boundary empty (safe default: whole-file AT wording applies).
func TestClassifyFileMixedContentMissingBoundary(t *testing.T) {
	dir := t.TempDir()
	canon := "Header line A\nHeader line B\n## Project Rules\n\n- canon rule\n"
	target := "Header line A\nHeader line B\nno boundary heading anywhere in target\n"
	mustWrite(t, filepath.Join(dir, "AGENTS.md"), target)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50}
	fr := classifyFile(cfg, "AGENTS.md", canon, "v0.0.0-test")
	if fr.Classification == ClassMatch {
		t.Errorf("Classification: got ClassMatch on missing-boundary target; want divergent")
	}
	if fr.Boundary != "" {
		t.Errorf("Boundary: got %q, want empty (target lacked boundary heading)", fr.Boundary)
	}
}

// AC142 AT5 (skout regression) — end-to-end: Run on a fixture target whose
// AGENTS.md has byte-identical canon-zone bytes plus a repo-owned
// ## Project Rules tail must NOT emit AGENTS.md under `## In Scope`. This is
// the recurring false-positive the format-defining override produced before
// this AC's mixed-content branch.
func TestRunMixedContentNotEmittedInScope(t *testing.T) {
	dir := docFixture(t)
	// Render canon to memory and extract AGENTS.md's canon zone for the
	// fixture overwrite so the target's canon-zone bytes equal canon's.
	gcfg := governance.Config{
		Mode:     governance.ModeApply,
		Target:   dir,
		Type:     governance.RepoTypeDoc,
		RepoName: "test-mixed-content",
	}
	canon, err := governance.RenderCanonicalFiles(EmbeddedFS, gcfg, dir)
	if err != nil {
		t.Fatalf("render canon: %v", err)
	}
	canonAgents, ok := canon["AGENTS.md"]
	if !ok {
		t.Fatalf("canon missing AGENTS.md")
	}
	canonZone, ok := extractCanonZone(canonAgents, "## Project Rules")
	if !ok {
		t.Fatalf("canon AGENTS.md missing `## Project Rules` boundary heading")
	}
	targetAgents := canonZone + "## Project Rules\n\n- Local project rule unique to this fixture.\n"
	mustWrite(t, filepath.Join(dir, "AGENTS.md"), targetAgents)
	gitAddCommit(t, dir, "overwrite AGENTS.md with canon-zone-equal fixture")

	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50, OverrideCanonID: "v0.0.0-test"}
	captureOut(t, func(f *os.File) { Run(cfg, EmbeddedFS, f) })
	stub := mustRead(t, filepath.Join(dir, "docs/ac1-drift-scan-v0.0.0-test.md"))

	inScopeStart := strings.Index(stub, "## In Scope")
	inScopeEnd := strings.Index(stub, "## Out Of Scope")
	if inScopeStart < 0 || inScopeEnd < 0 {
		t.Fatalf("stub missing required sections: %s", stub)
	}
	inScope := stub[inScopeStart:inScopeEnd]
	if strings.Contains(inScope, "`AGENTS.md`") {
		t.Errorf("AGENTS.md must not appear in `## In Scope` when canon zone is byte-equal (skout regression). Got:\n%s", inScope)
	}
}

// AC142 AT6 — buildACStub renders canon-zone-anchored AT wording for any
// sync entry whose FileResult.Boundary is non-empty, and whole-file AT
// wording when Boundary is empty.
func TestBuildACStubMixedContentATWording(t *testing.T) {
	r := Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "v0.0.0-test"},
		Files: []FileResult{
			{Relpath: "AGENTS.md", Classification: ClassClearSync, Boundary: "## Project Rules"},
		},
	}
	body := buildACStub(r, 1, "v0.0.0-test")
	wantMixed := "canon zone of `AGENTS.md` (above `## Project Rules`) matches canon byte-for-byte after sync"
	if !strings.Contains(body, wantMixed) {
		t.Errorf("expected mixed-content AT wording %q in stub, got:\n%s", wantMixed, body)
	}
	// Negative: must NOT emit the whole-file wording for this entry.
	whole := "`AGENTS.md` matches canon byte-for-byte after sync"
	if strings.Contains(body, whole) {
		t.Errorf("mixed-content entry must not emit whole-file AT wording %q; got:\n%s", whole, body)
	}
}

// AC142 AT7 — non-mixed-content sync entries continue to receive
// whole-file AT wording (Boundary stays empty for them).
func TestNonMixedContentUnaffected(t *testing.T) {
	dir := t.TempDir()
	canon := "canon body\n"
	target := "drift body\n"
	relpath := "docs/release.md" // not in mixedContentBoundary
	mustWrite(t, filepath.Join(dir, relpath), target)
	cfg := Config{Target: dir, Flavor: "doc", DiffLines: 50}
	fr := classifyFile(cfg, relpath, canon, "v0.0.0-test")
	if fr.Boundary != "" {
		t.Errorf("Boundary: got %q, want empty for non-mixed-content path %q", fr.Boundary, relpath)
	}
	if fr.Classification != ClassClearSync {
		t.Errorf("Classification: got %q, want %q for non-mixed-content drift", fr.Classification, ClassClearSync)
	}
	// AT wording in buildACStub stays whole-file.
	r := Report{
		Header: ReportHeader{Flavor: "doc", CanonSHA: "v0.0.0-test"},
		Files:  []FileResult{fr},
	}
	body := buildACStub(r, 1, "v0.0.0-test")
	want := "`docs/release.md` matches canon byte-for-byte after sync"
	if !strings.Contains(body, want) {
		t.Errorf("expected whole-file AT wording %q in stub, got:\n%s", want, body)
	}
}

// AC142 AT8 — docs/drift-scan.md contains the new subsection and the four
// amendments to existing generic statements so the new behavior does not
// coexist with stale generic wording. Run from inside the driftscan
// package; resolves docs/drift-scan.md at the repo root.
func TestDriftScanDocAmendments(t *testing.T) {
	// Resolve the repo-root drift-scan.md from the package's location.
	// The test runs with cwd = internal/driftscan/, so go up two levels.
	docPath := filepath.Join("..", "..", "docs", "drift-scan.md")
	body := mustRead(t, docPath)
	wants := []string{
		"## Mixed-content classification",
		"canon-zone-only for paths registered in mixed-content",
		"canon-zone byte-equality for mixed-content sync items",
		"or canon-zone-equal for mixed-content paths",
		"canon-zone byte-equal above",
	}
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("docs/drift-scan.md missing required wording: %q", w)
		}
	}
}
