package governance

import (
	"bufio"
	"github.com/kquo/governa/internal/templates"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssessTargetCodeRepo(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example\n")
	mustWrite(t, filepath.Join(root, "main.go"), "package main\n")
	mustWrite(t, filepath.Join(root, "README.md"), "# Example\n")

	assessment, err := AssessTarget(root, RepoTypeCode)
	if err != nil {
		t.Fatalf("AssessTarget() error = %v", err)
	}
	if assessment.RepoShape != "likely CODE" {
		t.Fatalf("RepoShape = %q, want likely CODE", assessment.RepoShape)
	}
	if assessment.CollisionRisk == "low" {
		t.Fatalf("CollisionRisk = %q, want medium or high", assessment.CollisionRisk)
	}
}

func TestAssessTargetDocRepo(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "README.md"), "# Docs\n")
	mustWrite(t, filepath.Join(root, "style.md"), "# Style\n")
	mustWrite(t, filepath.Join(root, "content-plan.md"), "# Plan\n")

	assessment, err := AssessTarget(root, RepoTypeDoc)
	if err != nil {
		t.Fatalf("AssessTarget() error = %v", err)
	}
	if assessment.RepoShape != "likely DOC" {
		t.Fatalf("RepoShape = %q, want likely DOC", assessment.RepoShape)
	}
	if assessment.Recommendation == "" {
		t.Fatal("Recommendation should not be empty")
	}
}

func TestAssessTargetDocsHeavyCodeRepo(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example\n")
	mustWrite(t, filepath.Join(root, "cmd", "tool", "main.go"), "package main\n")
	mustWrite(t, filepath.Join(root, "README.md"), "# Example\n")
	mustWrite(t, filepath.Join(root, "docs", "one.md"), "# One\n")
	mustWrite(t, filepath.Join(root, "docs", "two.md"), "# Two\n")
	mustWrite(t, filepath.Join(root, "docs", "three.md"), "# Three\n")

	assessment, err := AssessTarget(root, RepoTypeCode)
	if err != nil {
		t.Fatalf("AssessTarget() error = %v", err)
	}
	if assessment.RepoShape != "likely CODE" {
		t.Fatalf("RepoShape = %q, want likely CODE", assessment.RepoShape)
	}
}

func TestStackSuggestsGo(t *testing.T) {
	t.Parallel()

	if !stackSuggestsGo("Go CLI") {
		t.Fatal("expected Go stack to be detected")
	}
	if !stackSuggestsGo("golang service") {
		t.Fatal("expected golang stack to be detected")
	}
	if !stackSuggestsGo("Go-based CLI") {
		t.Fatal("expected Go-based stack to be detected")
	}
	if !stackSuggestsGo("go/grpc service") {
		t.Fatal("expected go/grpc stack to be detected")
	}
	if stackSuggestsGo("Rust service") {
		t.Fatal("did not expect Rust stack to be detected as Go")
	}
	if stackSuggestsGo("Django service") {
		t.Fatal("did not expect Django stack to be detected as Go")
	}
	if stackSuggestsGo("Google Cloud Functions") {
		t.Fatal("did not expect Google stack to be detected as Go")
	}
	if stackSuggestsGo("Cargo workspace") {
		t.Fatal("did not expect Cargo stack to be detected as Go")
	}
	if stackSuggestsGo("Hugo site") {
		t.Fatal("did not expect Hugo stack to be detected as Go")
	}
}

func TestReviewEnhancementExtractsGovernedSectionCandidates(t *testing.T) {
	t.Parallel()

	templateRoot := t.TempDir()
	referenceRoot := t.TempDir()

	mustWrite(t, filepath.Join(templateRoot, "base", "AGENTS.md"), `# AGENTS.md

## Purpose

Base purpose.

## Governed Sections

- Purpose

## Interaction Mode

- Default to discussion first.

## Approval Boundaries

- Do not release without approval.

## Review Style

- Findings first.

## File-Change Discipline

- Prefer targeted edits.

## Release Or Publish Triggers

- Release only on request.

## Documentation Update Expectations

- Update docs with behavior.
`)
	mustWrite(t, filepath.Join(referenceRoot, "AGENTS.md"), `# AGENTS.md

## Purpose

Base purpose.

## Governed Sections

- Purpose

## Interaction Mode

- Default to discussion first.
- Do not create artifacts or make changes unless explicitly authorized.

## Approval Boundaries

- Do not release without approval.

## Review Style

- Findings first.

## File-Change Discipline

- Prefer targeted edits.

## Release Or Publish Triggers

- Release only on request.

## Documentation Update Expectations

- Update docs with behavior.
`)

	report, err := ReviewEnhancement(os.DirFS(templateRoot), templateRoot, referenceRoot)
	if err != nil {
		t.Fatalf("ReviewEnhancement() error = %v", err)
	}
	if len(report.Candidates) != 1 {
		t.Fatalf("len(report.Candidates) = %d, want 1", len(report.Candidates))
	}
	got := report.Candidates[0]
	if got.Area != "base governance" {
		t.Fatalf("candidate Area = %q, want base governance", got.Area)
	}
	if got.Section != "Interaction Mode" {
		t.Fatalf("candidate Section = %q, want Interaction Mode", got.Section)
	}
	if got.Disposition != "accept" {
		t.Fatalf("candidate Disposition = %q, want accept", got.Disposition)
	}
	if got.Portability != "portable" {
		t.Fatalf("candidate Portability = %q, want portable", got.Portability)
	}
}

func TestReviewEnhancementSkipsEquivalentGovernedSectionGuidance(t *testing.T) {
	t.Parallel()

	templateRoot := t.TempDir()
	referenceRoot := filepath.Join(t.TempDir(), "skout")
	if err := os.MkdirAll(referenceRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", referenceRoot, err)
	}

	mustWrite(t, filepath.Join(templateRoot, "base", "AGENTS.md"), `# AGENTS.md

## Purpose

Base purpose.

## Governed Sections

- Purpose

## Interaction Mode

- Treat requests as exploratory discussion unless the user explicitly asks for implementation or file changes.
- Do not create artifacts or make changes unless the user explicitly authorizes them.
- When the user authorizes changes, make the smallest concrete change that satisfies the request.
- Surface assumptions, ambiguities, and missing context plainly before taking action that could change project direction.

## Approval Boundaries

- Do not release without approval.

## Review Style

- Findings first.

## File-Change Discipline

- Prefer targeted edits.

## Release Or Publish Triggers

- Release only on request.

## Documentation Update Expectations

- Update docs with behavior.
`)
	// Reference has a subset of the template's constraints using identical text.
	mustWrite(t, filepath.Join(referenceRoot, "AGENTS.md"), `# AGENTS.md

## Purpose

Base purpose.

## Governed Sections

- Purpose

## Interaction Mode

- Treat requests as exploratory discussion unless the user explicitly asks for implementation or file changes.
- Do not create artifacts or make changes unless the user explicitly authorizes them.

## Approval Boundaries

- Do not release without approval.

## Review Style

- Findings first.

## File-Change Discipline

- Prefer targeted edits.

## Release Or Publish Triggers

- Release only on request.

## Documentation Update Expectations

- Update docs with behavior.
`)

	report, err := ReviewEnhancement(os.DirFS(templateRoot), templateRoot, referenceRoot)
	if err != nil {
		t.Fatalf("ReviewEnhancement() error = %v", err)
	}
	for _, candidate := range report.Candidates {
		if candidate.Area == "base governance" && candidate.Section == "Interaction Mode" {
			t.Fatal("did not expect equivalent Interaction Mode guidance to be flagged")
		}
	}
}

func TestReviewEnhancementDefersProjectSpecificFiles(t *testing.T) {
	t.Parallel()

	templateRoot := t.TempDir()
	referenceRoot := filepath.Join(t.TempDir(), "skout")
	if err := os.MkdirAll(referenceRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", referenceRoot, err)
	}

	mustWrite(t, filepath.Join(templateRoot, "base", "AGENTS.md"), "# AGENTS.md\n")
	mustWrite(t, filepath.Join(templateRoot, "overlays", "code", "files", "README.md.tmpl"), "# {{REPO_NAME}}\n")
	mustWrite(t, filepath.Join(referenceRoot, "README.md"), "# skout\n\nThis repo keeps skout-specific release notes.\n")

	report, err := ReviewEnhancement(os.DirFS(templateRoot), templateRoot, referenceRoot)
	if err != nil {
		t.Fatalf("ReviewEnhancement() error = %v", err)
	}
	if len(report.Candidates) == 0 {
		t.Fatal("expected at least one candidate")
	}
	found := false
	for _, candidate := range report.Candidates {
		if filepath.Base(candidate.Path) == "README.md" && candidate.Area == "CODE overlay" {
			found = true
			if candidate.Disposition != "defer" {
				t.Fatalf("candidate Disposition = %q, want defer", candidate.Disposition)
			}
			if candidate.Portability != "project-specific" {
				t.Fatalf("candidate Portability = %q, want project-specific", candidate.Portability)
			}
		}
	}
	if !found {
		t.Fatal("expected a CODE overlay README candidate")
	}
}

func TestSelectActionableCandidatesNoneActionable(t *testing.T) {
	t.Parallel()

	candidates := []EnhancementCandidate{
		{Area: "CODE overlay", Disposition: "defer", Portability: "project-specific"},
		{Area: "DOC overlay", Disposition: "reject", Portability: "project-specific"},
	}
	_, _, ok := selectActionableCandidates(candidates)
	if ok {
		t.Fatal("expected no actionable candidates")
	}
}

func TestSelectActionableCandidatesSingleAccept(t *testing.T) {
	t.Parallel()

	candidates := []EnhancementCandidate{
		{Area: "CODE overlay", Disposition: "defer", Portability: "project-specific"},
		{Area: "base governance", Disposition: "accept", Portability: "portable", Section: "Review Style"},
	}
	selected, deferred, ok := selectActionableCandidates(candidates)
	if !ok {
		t.Fatal("expected an actionable candidate")
	}
	if selected.Section != "Review Style" {
		t.Fatalf("selected.Section = %q, want Review Style", selected.Section)
	}
	if len(deferred) != 0 {
		t.Fatalf("expected no deferred candidates, got %d", len(deferred))
	}
}

func TestSelectActionableCandidatesRanking(t *testing.T) {
	t.Parallel()

	candidates := []EnhancementCandidate{
		{Area: "DOC overlay", Disposition: "adapt", Portability: "portable", Section: "lower-rank"},
		{Area: "base governance", Disposition: "accept", Portability: "portable", Section: "highest-rank"},
		{Area: "CODE overlay", Disposition: "adapt", Portability: "needs-review", Section: "lowest-rank"},
	}
	selected, deferred, ok := selectActionableCandidates(candidates)
	if !ok {
		t.Fatal("expected an actionable candidate")
	}
	if selected.Section != "highest-rank" {
		t.Fatalf("selected.Section = %q, want highest-rank", selected.Section)
	}
	if len(deferred) != 2 {
		t.Fatalf("expected 2 deferred candidates, got %d", len(deferred))
	}
}

func TestNextACNumber(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "ac1-first.md"), "# AC\n")
	mustWrite(t, filepath.Join(dir, "ac3-third.md"), "# AC\n")
	mustWrite(t, filepath.Join(dir, "ac-template.md"), "# Template\n")
	mustWrite(t, filepath.Join(dir, "ac-001-old.md"), "# Old format\n")
	mustWrite(t, filepath.Join(dir, "other.md"), "# Other\n")

	num, err := nextACNumber(dir)
	if err != nil {
		t.Fatalf("nextACNumber() error = %v", err)
	}
	if num != 4 {
		t.Fatalf("nextACNumber() = %d, want 4 (old-format ac-001-old.md must be ignored)", num)
	}
}

func TestIsWorkingACFile(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		want bool
	}{
		{"ac1-foo.md", true},
		{"ac10-foo.md", true},
		{"ac100-foo.md", true},
		{"ac-template.md", false},
		{"ac-001-foo.md", false},
		{"acfoo.md", false},
		{"ac1.md", false},
		{"random.md", false},
	}
	for _, tc := range cases {
		if got := isWorkingACFile(tc.name); got != tc.want {
			t.Errorf("isWorkingACFile(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestIsACKeeperFile(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		want bool
	}{
		{"ac-template.md", true},
		{"ac-example.md", false},
		{"ac1-foo.md", false},
		{"ac-001-foo.md", false},
		{"random.md", false},
	}
	for _, tc := range cases {
		if got := isACKeeperFile(tc.name); got != tc.want {
			t.Errorf("isACKeeperFile(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestNextACNumberEmptyDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	num, err := nextACNumber(dir)
	if err != nil {
		t.Fatalf("nextACNumber() error = %v", err)
	}
	if num != 1 {
		t.Fatalf("nextACNumber() = %d, want 1", num)
	}
}

func TestRenderACDocIncludesRequiredSections(t *testing.T) {
	t.Parallel()

	selected := EnhancementCandidate{
		Area:            "base governance",
		Path:            "/tmp/reference/AGENTS.md",
		Section:         "Interaction Mode",
		Disposition:     "accept",
		Reason:          "portable delta",
		Portability:     "portable",
		TemplateTarget:  "base/AGENTS.md",
		Summary:         "section differs",
		CollisionImpact: "medium",
	}
	deferred := []EnhancementCandidate{{
		Area:            "CODE overlay",
		Disposition:     "adapt",
		Portability:     "portable",
		TemplateTarget:  "overlays/code/files/README.md.tmpl",
		CollisionImpact: "low",
	}}
	report := EnhancementReport{ReferenceRoot: "/tmp/reference"}

	content := renderACDoc(selected, deferred, report, 2)
	for _, want := range []string{
		"# AC2 Enhance: base governance — Interaction Mode",
		"## Objective Fit",
		"## Summary",
		"portable delta",
		"## In Scope",
		"base/AGENTS.md",
		"## Out Of Scope",
		"## Implementation Notes",
		"## Acceptance Tests",
		"## Deferred Candidates",
		"CODE overlay",
		"## Status",
		"PENDING",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("AC doc missing %q", want)
		}
	}
}

func TestRenderACDocReflectsActualDisposition(t *testing.T) {
	t.Parallel()

	selected := EnhancementCandidate{
		Area:        "CODE overlay",
		Disposition: "adapt",
		Portability: "needs-review",
		Reason:      "needs adaptation",
	}
	report := EnhancementReport{ReferenceRoot: "/tmp/reference"}

	content := renderACDoc(selected, nil, report, 1)
	if !strings.Contains(content, "`adapt` with portability `needs-review`") {
		t.Fatal("AC doc Objective Fit should reflect actual disposition and portability")
	}
	if strings.Contains(content, "portable and suitable") {
		t.Fatal("AC doc should not hardcode 'portable and suitable' for non-portable candidates")
	}
}

func TestRenderACDocNoDeferredSection(t *testing.T) {
	t.Parallel()

	selected := EnhancementCandidate{
		Area:        "CODE overlay",
		Disposition: "accept",
		Portability: "portable",
		Reason:      "portable improvement",
	}
	report := EnhancementReport{ReferenceRoot: "/tmp/reference"}

	content := renderACDoc(selected, nil, report, 1)
	if strings.Contains(content, "Deferred Candidates") {
		t.Fatal("AC doc should not contain Deferred Candidates section when none exist")
	}
}

func TestACSlug(t *testing.T) {
	t.Parallel()

	if got := acSlug(EnhancementCandidate{Area: "base governance", Section: "Interaction Mode"}); got != "interaction-mode" {
		t.Fatalf("acSlug() = %q, want interaction-mode", got)
	}
	if got := acSlug(EnhancementCandidate{Area: "CODE overlay"}); got != "code-overlay" {
		t.Fatalf("acSlug() = %q, want code-overlay", got)
	}
}

func TestRunEnhanceNoActionableCandidatesCreatesNoFile(t *testing.T) {
	t.Parallel()

	templateRoot := t.TempDir()
	referenceRoot := filepath.Join(t.TempDir(), "ref")
	if err := os.MkdirAll(referenceRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	mustWrite(t, filepath.Join(templateRoot, "base", "AGENTS.md"), "# AGENTS.md\n\n## Purpose\n\nBase purpose.\n")
	mustWrite(t, filepath.Join(templateRoot, "docs", "ac-template.md"), "# AC template\n")
	mustWrite(t, filepath.Join(referenceRoot, "AGENTS.md"), "# AGENTS.md\n\n## Purpose\n\nBase purpose.\n")

	cfg := Config{Mode: ModeEnhance, Reference: referenceRoot}
	if err := RunEnhance(os.DirFS(templateRoot), templateRoot, cfg); err != nil {
		t.Fatalf("RunEnhance() error = %v", err)
	}

	entries, _ := os.ReadDir(filepath.Join(templateRoot, "docs"))
	for _, entry := range entries {
		if isWorkingACFile(entry.Name()) {
			t.Fatalf("unexpected AC doc created: %s", entry.Name())
		}
	}
}

func TestRunEnhanceSingleActionableCandidateCreatesACDoc(t *testing.T) {
	t.Parallel()

	templateRoot := t.TempDir()
	referenceRoot := filepath.Join(t.TempDir(), "ref")
	if err := os.MkdirAll(referenceRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	mustWrite(t, filepath.Join(templateRoot, "base", "AGENTS.md"), `# AGENTS.md

## Purpose

Base purpose.

## Interaction Mode

- Default to discussion first.
`)
	mustWrite(t, filepath.Join(templateRoot, "docs", "ac-template.md"), "# AC template\n")
	mustWrite(t, filepath.Join(referenceRoot, "AGENTS.md"), `# AGENTS.md

## Purpose

Base purpose.

## Interaction Mode

- Default to discussion first.
- Do not create artifacts or make changes unless explicitly authorized.
`)

	cfg := Config{Mode: ModeEnhance, Reference: referenceRoot}
	if err := RunEnhance(os.DirFS(templateRoot), templateRoot, cfg); err != nil {
		t.Fatalf("RunEnhance() error = %v", err)
	}

	docsDir := filepath.Join(templateRoot, "docs")
	acFile := findACDoc(t, docsDir)
	if acFile == "" {
		t.Fatal("expected an AC doc to be created")
	}

	content, err := os.ReadFile(filepath.Join(docsDir, acFile))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	contentStr := string(content)
	for _, want := range []string{"# AC1", "## Summary", "## Status", "PENDING"} {
		if !strings.Contains(contentStr, want) {
			t.Fatalf("AC doc missing %q", want)
		}
	}
}

func TestRunEnhanceMultipleActionableCandidatesCreatesSingleACDoc(t *testing.T) {
	t.Parallel()

	templateRoot := t.TempDir()
	referenceRoot := filepath.Join(t.TempDir(), "ref")
	if err := os.MkdirAll(referenceRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	mustWrite(t, filepath.Join(templateRoot, "base", "AGENTS.md"), `# AGENTS.md

## Purpose

Base purpose.

## Interaction Mode

- Default to discussion first.

## Review Style

- Findings first.
`)
	mustWrite(t, filepath.Join(templateRoot, "overlays", "code", "files", "README.md.tmpl"), "# {{REPO_NAME}}\n")
	mustWrite(t, filepath.Join(templateRoot, "docs", "ac-template.md"), "# AC template\n")
	mustWrite(t, filepath.Join(referenceRoot, "AGENTS.md"), `# AGENTS.md

## Purpose

Base purpose.

## Interaction Mode

- Default to discussion first.
- Added authorization rule.

## Review Style

- Findings first.
- Added coverage rule.
`)
	mustWrite(t, filepath.Join(referenceRoot, "README.md"), "# Different README\n")

	cfg := Config{Mode: ModeEnhance, Reference: referenceRoot}
	if err := RunEnhance(os.DirFS(templateRoot), templateRoot, cfg); err != nil {
		t.Fatalf("RunEnhance() error = %v", err)
	}

	docsDir := filepath.Join(templateRoot, "docs")
	entries, _ := os.ReadDir(docsDir)
	acCount := 0
	var acFile string
	for _, entry := range entries {
		if isWorkingACFile(entry.Name()) {
			acCount++
			acFile = entry.Name()
		}
	}
	if acCount != 1 {
		t.Fatalf("expected 1 AC doc, got %d", acCount)
	}

	content, _ := os.ReadFile(filepath.Join(docsDir, acFile))
	if !strings.Contains(string(content), "Deferred Candidates") {
		t.Fatal("AC doc should include Deferred Candidates section")
	}
}

func TestRunEnhanceDryRunCreatesNoFile(t *testing.T) {
	t.Parallel()

	templateRoot := t.TempDir()
	referenceRoot := filepath.Join(t.TempDir(), "ref")
	if err := os.MkdirAll(referenceRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	mustWrite(t, filepath.Join(templateRoot, "base", "AGENTS.md"), `# AGENTS.md

## Purpose

Base purpose.

## Interaction Mode

- Default to discussion first.
`)
	mustWrite(t, filepath.Join(templateRoot, "docs", "ac-template.md"), "# AC template\n")
	mustWrite(t, filepath.Join(referenceRoot, "AGENTS.md"), `# AGENTS.md

## Purpose

Base purpose.

## Interaction Mode

- Default to discussion first.
- Added authorization rule.
`)

	cfg := Config{Mode: ModeEnhance, Reference: referenceRoot, DryRun: true}
	if err := RunEnhance(os.DirFS(templateRoot), templateRoot, cfg); err != nil {
		t.Fatalf("RunEnhance() error = %v", err)
	}

	docsDir := filepath.Join(templateRoot, "docs")
	entries, _ := os.ReadDir(docsDir)
	for _, entry := range entries {
		if isWorkingACFile(entry.Name()) {
			t.Fatalf("dry-run should not create AC doc, found: %s", entry.Name())
		}
	}
}

func findACDoc(t *testing.T, docsDir string) string {
	t.Helper()
	entries, err := os.ReadDir(docsDir)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", docsDir, err)
	}
	for _, entry := range entries {
		if isWorkingACFile(entry.Name()) {
			return entry.Name()
		}
	}
	return ""
}

// --- summarizeFileContent / truncateForSummary / candidateRank tests ---

func TestSummarizeFileContentWithHeadings(t *testing.T) {
	t.Parallel()
	got := summarizeFileContent("test.md", "# Title\n\n## Section One\n\nBody.\n")
	if !strings.Contains(got, "Title") {
		t.Fatalf("expected heading in summary, got %q", got)
	}
}

func TestSummarizeFileContentEmpty(t *testing.T) {
	t.Parallel()
	got := summarizeFileContent("test.md", "")
	if !strings.Contains(got, "mostly empty") {
		t.Fatalf("expected mostly empty note, got %q", got)
	}
}

func TestSummarizeFileContentNoHeadings(t *testing.T) {
	t.Parallel()
	got := summarizeFileContent("test.md", "Just some text without headings.\n")
	if !strings.Contains(got, "starts with") {
		t.Fatalf("expected 'starts with' in summary, got %q", got)
	}
}

func TestTruncateForSummaryShort(t *testing.T) {
	t.Parallel()
	input := "short string"
	if got := truncateForSummary(input); got != input {
		t.Fatalf("got %q, want %q", got, input)
	}
}

func TestTruncateForSummaryLong(t *testing.T) {
	t.Parallel()
	input := strings.Repeat("a", 100)
	got := truncateForSummary(input)
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("expected truncation with ..., got %q", got)
	}
	if len(got) > 75 {
		t.Fatalf("truncated result too long: %d chars", len(got))
	}
}

func TestCandidateRankAllTiers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		disposition string
		portability string
		wantRank    int
	}{
		{"accept", "portable", 1},
		{"accept", "needs-review", 2},
		{"adapt", "portable", 3},
		{"adapt", "needs-review", 4},
		{"defer", "project-specific", 99},
		{"reject", "portable", 99},
	}
	for _, tc := range cases {
		c := EnhancementCandidate{Disposition: tc.disposition, Portability: tc.portability}
		if got := candidateRank(c); got != tc.wantRank {
			t.Fatalf("candidateRank(%s+%s) = %d, want %d", tc.disposition, tc.portability, got, tc.wantRank)
		}
	}
}

func TestDisplayReferencePathRelative(t *testing.T) {
	t.Parallel()
	got := displayReferencePath("/tmp/ref", "/tmp/ref/AGENTS.md")
	if got != "<reference-root>/AGENTS.md" {
		t.Fatalf("got %q", got)
	}
}

func TestDisplayReferencePathOutside(t *testing.T) {
	t.Parallel()
	got := displayReferencePath("/tmp/ref", "/other/path/file.md")
	if strings.Contains(got, "<reference-root>") {
		t.Fatalf("should not use placeholder for outside path, got %q", got)
	}
}

func TestDisplayReferencePathEmpty(t *testing.T) {
	t.Parallel()
	got := displayReferencePath("", "/some/path")
	if got != "/some/path" {
		t.Fatalf("got %q, want raw path", got)
	}
}

// --- ParseModeArgs help/error tests ---

func TestParseModeArgsHelpReturnsCleanExit(t *testing.T) {
	t.Parallel()
	_, help, err := ParseModeArgs(ModeSync, []string{"--help"})
	if err != nil {
		t.Fatalf("ParseModeArgs(--help) error = %v, want nil", err)
	}
	if !help {
		t.Fatal("ParseModeArgs(--help) help = false, want true")
	}
}

func TestParseModeArgsInvalidFlagReturnsError(t *testing.T) {
	t.Parallel()
	_, help, err := ParseModeArgs(ModeSync, []string{"--bogus-flag"})
	if err == nil {
		t.Fatal("ParseModeArgs(--bogus-flag) expected error")
	}
	if help {
		t.Fatal("ParseModeArgs(--bogus-flag) help = true, want false")
	}
}

// --- validateConfig tests ---

func TestValidateConfigNewCodeValid(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeSync, Type: RepoTypeCode, RepoName: "r", Purpose: "p", Stack: "Go CLI"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateConfigNewDocValid(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeSync, Type: RepoTypeDoc, RepoName: "r", Purpose: "p", PublishingPlatform: "Hugo", Style: "concise"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateConfigNewMissingName(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeSync, Type: RepoTypeCode, Purpose: "p", Stack: "Go"})
	if err == nil {
		t.Fatal("expected error for missing repo name")
	}
}

func TestValidateConfigNewMissingPurpose(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeSync, Type: RepoTypeCode, RepoName: "r", Stack: "Go"})
	if err == nil {
		t.Fatal("expected error for missing purpose")
	}
}

func TestValidateConfigNewBadType(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeSync, Type: "INVALID", RepoName: "r", Purpose: "p"})
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestValidateConfigNewCodeMissingStack(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeSync, Type: RepoTypeCode, RepoName: "r", Purpose: "p"})
	if err == nil {
		t.Fatal("expected error for missing stack")
	}
}

func TestValidateConfigNewDocMissingPlatform(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeSync, Type: RepoTypeDoc, RepoName: "r", Purpose: "p", Style: "concise"})
	if err == nil {
		t.Fatal("expected error for missing publishing platform")
	}
}

func TestValidateConfigNewDocMissingStyle(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeSync, Type: RepoTypeDoc, RepoName: "r", Purpose: "p", PublishingPlatform: "Hugo"})
	if err == nil {
		t.Fatal("expected error for missing style")
	}
}

func TestValidateConfigSyncRequiresType(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeSync, RepoName: "r", Purpose: "p"})
	if err == nil {
		t.Fatal("expected error for missing type in sync mode")
	}
}

func TestValidateConfigSyncRejectsBadType(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeSync, Type: "WRONG", RepoName: "r", Purpose: "p"})
	if err == nil {
		t.Fatal("expected error for invalid sync type")
	}
}

func TestValidateConfigEnhanceValid(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeEnhance, Reference: "/tmp/ref"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateConfigEnhanceEmptyRefAllowed(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeEnhance})
	if err != nil {
		t.Fatalf("expected no error for enhance with empty reference (self-review), got: %v", err)
	}
}

func TestValidateConfigNoMode(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{})
	if err == nil {
		t.Fatal("expected error for missing mode")
	}
}

// --- readAndRender tests ---

func TestReadAndRender(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "template.md")
	mustWrite(t, path, "# {{REPO_NAME}}\n\nPurpose: {{PROJECT_PURPOSE}}\nStack: {{STACK_OR_PLATFORM}}\n")

	result, err := readAndRender(os.DirFS(dir), "template.md", map[string]string{
		"{{REPO_NAME}}":         "my-repo",
		"{{PROJECT_PURPOSE}}":   "test purpose",
		"{{STACK_OR_PLATFORM}}": "Go CLI",
	})
	if err != nil {
		t.Fatalf("readAndRender() error = %v", err)
	}
	if !strings.Contains(result, "# my-repo") {
		t.Fatal("expected repo name substitution")
	}
	if !strings.Contains(result, "Purpose: test purpose") {
		t.Fatal("expected purpose substitution")
	}
	if !strings.Contains(result, "Stack: Go CLI") {
		t.Fatal("expected stack substitution")
	}
}

func TestReadAndRenderMissingFile(t *testing.T) {
	t.Parallel()
	_, err := readAndRender(os.DirFS(t.TempDir()), "nonexistent/file.md", nil)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// --- valueOrDefault / joinOrNone tests ---

func TestValueOrDefault(t *testing.T) {
	t.Parallel()
	if got := valueOrDefault("hello", "fallback"); got != "hello" {
		t.Fatalf("got %q, want hello", got)
	}
	if got := valueOrDefault("", "fallback"); got != "fallback" {
		t.Fatalf("got %q, want fallback", got)
	}
	if got := valueOrDefault("  ", "fallback"); got != "fallback" {
		t.Fatalf("got %q, want fallback for whitespace", got)
	}
}

func TestJoinOrNone(t *testing.T) {
	t.Parallel()
	if got := joinOrNone(nil); got != "none" {
		t.Fatalf("got %q, want none", got)
	}
	if got := joinOrNone([]string{"a", "b"}); got != "a, b" {
		t.Fatalf("got %q, want a, b", got)
	}
}

// --- formatAction tests ---

func TestFormatAction(t *testing.T) {
	t.Parallel()
	if got := formatAction(false, "write"); got != "write" {
		t.Fatalf("got %q, want write", got)
	}
	if got := formatAction(true, "write"); got != "dry-run write" {
		t.Fatalf("got %q, want dry-run write", got)
	}
}

// --- compactOperations tests ---

func TestCompactOperations(t *testing.T) {
	t.Parallel()
	ops := []operation{
		{kind: "write", path: "a"},
		{kind: "skip"},
		{kind: "symlink", path: "b"},
		{kind: "skip"},
	}
	result := compactOperations(ops)
	if len(result) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(result))
	}
	if result[0].path != "a" || result[1].path != "b" {
		t.Fatal("unexpected operation paths after compaction")
	}
}

// --- scoreOverlayCollision tests ---

func TestScoreOverlayCollisionKeepLargerExisting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	existing := filepath.Join(dir, "README.md")
	// 20 lines existing vs 5 lines proposed
	mustWrite(t, existing, strings.Repeat("line\n", 20))
	score := scoreOverlayCollision(existing, strings.Repeat("line\n", 5), "", "")
	if score.recommendation != "keep" {
		t.Fatalf("recommendation = %q, want keep", score.recommendation)
	}
}

func TestScoreOverlayCollisionKeepMoreSections(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	existing := filepath.Join(dir, "doc.md")
	mustWrite(t, existing, "# Doc\n\n## A\ncontent\n## B\ncontent\n## C\ncontent\n## D\ncontent\n")
	score := scoreOverlayCollision(existing, "# Doc\n\n## A\ncontent\n## B\ncontent\n", "", "")
	if score.recommendation != "keep" {
		t.Fatalf("recommendation = %q, want keep", score.recommendation)
	}
}

func TestScoreOverlayCollisionReviewNewSections(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	existing := filepath.Join(dir, "doc.md")
	mustWrite(t, existing, "# Doc\n\n## A\ncontent\n")
	score := scoreOverlayCollision(existing, "# Doc\n\n## A\ncontent\n## B\nnew content\n", "", "")
	if score.recommendation != "adopt" {
		t.Fatalf("recommendation = %q, want adopt", score.recommendation)
	}
	if len(score.missingSections) == 0 || score.missingSections[0] != "B" {
		t.Fatalf("missingSections = %v, want [B]", score.missingSections)
	}
}

func TestScoreOverlayCollisionAcceptMissing(t *testing.T) {
	t.Parallel()
	score := scoreOverlayCollision("/nonexistent/file.md", "content\n", "", "")
	if score.recommendation != "accept" {
		t.Fatalf("recommendation = %q, want accept", score.recommendation)
	}
}

func TestScoreOverlayCollisionReviewNonMarkdown(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	existing := filepath.Join(dir, "build.sh")
	mustWrite(t, existing, "#!/bin/bash\necho hello\n")
	score := scoreOverlayCollision(existing, "#!/bin/bash\necho world\n", "", "")
	if score.recommendation != "keep" {
		t.Fatalf("recommendation = %q, want keep for non-markdown with unchanged template", score.recommendation)
	}
}

func TestSkipIfExistsFileExists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	existing := filepath.Join(dir, "file.md")
	mustWrite(t, existing, "content")

	op := operation{kind: "write", path: existing}
	result := skipIfExists(op)
	if result.kind != "skip" {
		t.Fatalf("expected skip, got %q", result.kind)
	}
}

func TestSkipIfExistsFileDoesNotExist(t *testing.T) {
	t.Parallel()
	op := operation{kind: "write", path: "/nonexistent/file.md"}
	result := skipIfExists(op)
	if result.kind != "write" {
		t.Fatalf("expected write, got %q", result.kind)
	}
}

// --- applyOperations tests ---

func TestApplyOperationsWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "sub", "file.md")
	ops := []operation{{kind: "write", path: target, content: "hello", note: "test"}}

	if err := applyOperations(ops, false); err != nil {
		t.Fatalf("applyOperations() error = %v", err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("got %q, want hello", string(content))
	}
}

func TestApplyOperationsWriteShellExecutable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "build.sh")
	ops := []operation{{kind: "write", path: target, content: "#!/bin/bash", note: "test"}}

	if err := applyOperations(ops, false); err != nil {
		t.Fatalf("applyOperations() error = %v", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatal("expected .sh file to be executable")
	}
}

func TestApplyOperationsSymlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "CLAUDE.md")
	ops := []operation{{kind: "symlink", path: target, linkTo: "AGENTS.md", note: "test"}}

	if err := applyOperations(ops, false); err != nil {
		t.Fatalf("applyOperations() error = %v", err)
	}
	linkTarget, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("Readlink() error = %v", err)
	}
	if linkTarget != "AGENTS.md" {
		t.Fatalf("got link target %q, want AGENTS.md", linkTarget)
	}
}

func TestApplyOperationsDryRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "should-not-exist.md")
	ops := []operation{
		{kind: "write", path: target, content: "hello", note: "test"},
		{kind: "symlink", path: filepath.Join(dir, "link.md"), linkTo: "x", note: "test"},
		{kind: "mkdir", path: filepath.Join(dir, "newdir"), note: "test"},
	}

	if err := applyOperations(ops, true); err != nil {
		t.Fatalf("applyOperations() error = %v", err)
	}
	if _, err := os.Stat(target); err == nil {
		t.Fatal("dry-run should not create files")
	}
}

func TestApplyOperationsMkdir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "newdir", "subdir")
	ops := []operation{{kind: "mkdir", path: target, note: "test"}}

	if err := applyOperations(ops, false); err != nil {
		t.Fatalf("applyOperations() error = %v", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected a directory")
	}
}

func TestApplyOperationsUnsupportedKind(t *testing.T) {
	t.Parallel()
	ops := []operation{{kind: "bogus"}}
	if err := applyOperations(ops, false); err == nil {
		t.Fatal("expected error for unsupported operation kind")
	}
}

// --- planRender integration test ---

func TestPlanRenderCodeOverlay(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	targetRoot := t.TempDir()

	mustWrite(t, filepath.Join(root, "base", "AGENTS.md"), "# {{REPO_NAME}} governance\n")
	mustWrite(t, filepath.Join(root, "TEMPLATE_VERSION"), "0.1.0\n")
	mustWrite(t, filepath.Join(root, "overlays", "code", "files", "README.md.tmpl"), "# {{REPO_NAME}}\n\n{{PROJECT_PURPOSE}}\n")
	mustWrite(t, filepath.Join(root, "overlays", "code", "files", "build.sh.tmpl"), "#!/bin/bash\necho {{REPO_NAME}}\n")

	cfg := Config{
		Mode:     ModeSync,
		Type:     RepoTypeCode,
		RepoName: "test-repo",
		Purpose:  "test purpose",
		Stack:    "Go CLI",
	}

	ops, err := planRender(os.DirFS(root), root, cfg, targetRoot, false)
	if err != nil {
		t.Fatalf("planRender() error = %v", err)
	}
	if len(ops) == 0 {
		t.Fatal("expected at least one operation")
	}

	// Check AGENTS.md write operation has rendered content
	found := false
	for _, op := range ops {
		if strings.HasSuffix(op.path, "AGENTS.md") && op.kind == "write" {
			found = true
			if !strings.Contains(op.content, "test-repo") {
				t.Fatal("AGENTS.md should have repo name rendered")
			}
		}
	}
	if !found {
		t.Fatal("expected AGENTS.md write operation")
	}
}

func TestPlanRenderAdoptSkipsExistingGovernance(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	targetRoot := t.TempDir()

	mustWrite(t, filepath.Join(root, "base", "AGENTS.md"), "# governance\n")
	mustWrite(t, filepath.Join(root, "TEMPLATE_VERSION"), "0.1.0\n")
	mustWrite(t, filepath.Join(root, "overlays", "code", "files", "README.md.tmpl"), "# Template README\n")

	// Pre-existing file in target
	mustWrite(t, filepath.Join(targetRoot, "AGENTS.md"), "# Existing governance\n")

	cfg := Config{
		Mode:     ModeSync,
		Type:     RepoTypeCode,
		RepoName: "test-repo",
		Purpose:  "test purpose",
		Stack:    "Go CLI",
	}

	ops, err := planRender(os.DirFS(root), root, cfg, targetRoot, true)
	if err != nil {
		t.Fatalf("planRender() error = %v", err)
	}

	// Existing AGENTS.md should be skipped (collision handled via review doc)
	for _, op := range ops {
		if strings.Contains(op.path, "AGENTS") && op.kind == "write" {
			t.Fatalf("existing AGENTS.md should be skipped, got write to %q", op.path)
		}
	}
}

func TestPlanRenderNonGoStackSkipsGoFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	targetRoot := t.TempDir()

	mustWrite(t, filepath.Join(root, "base", "AGENTS.md"), "# governance\n")
	mustWrite(t, filepath.Join(root, "TEMPLATE_VERSION"), "0.1.0\n")
	mustWrite(t, filepath.Join(root, "overlays", "code", "files", "README.md.tmpl"), "# README\n")
	mustWrite(t, filepath.Join(root, "overlays", "code", "files", "cmd", "build", "main.go.tmpl"), "package main\n")
	mustWrite(t, filepath.Join(root, "overlays", "code", "files", "cmd", "rel", "main.go.tmpl"), "package main\n")
	mustWrite(t, filepath.Join(root, "overlays", "code", "files", "internal", "color", "color.go.tmpl"), "package color\n")

	cfg := Config{
		Mode:     ModeSync,
		Type:     RepoTypeCode,
		RepoName: "test-repo",
		Purpose:  "test purpose",
		Stack:    "Rust service",
	}

	ops, err := planRender(os.DirFS(root), root, cfg, targetRoot, false)
	if err != nil {
		t.Fatalf("planRender() error = %v", err)
	}

	for _, op := range ops {
		if strings.HasSuffix(op.path, ".go") {
			t.Fatalf("non-Go stack should not include Go files, found %q", op.path)
		}
	}
}

// --- AC-006 Phase 1: constraint-level governance comparison ---

func TestExtractConstraintsBulletList(t *testing.T) {
	t.Parallel()
	body := "- Do not release without approval.\n- Keep changes targeted.\n- Surface assumptions before acting.\n"
	got := extractConstraints(body)
	if len(got) != 3 {
		t.Fatalf("extractConstraints() returned %d constraints, want 3", len(got))
	}
}

func TestExtractConstraintsMultiLineBullet(t *testing.T) {
	t.Parallel()
	body := "- Do not release without approval\n  unless the user explicitly asks.\n- Keep changes targeted.\n"
	got := extractConstraints(body)
	if len(got) != 2 {
		t.Fatalf("extractConstraints() returned %d constraints, want 2", len(got))
	}
	if !strings.Contains(got[0], "unless") {
		t.Fatalf("first constraint should include continuation line, got %q", got[0])
	}
}

func TestExtractConstraintsNumberedList(t *testing.T) {
	t.Parallel()
	body := "1. Do not release without approval.\n2. Keep changes targeted.\n3. Surface assumptions before acting.\n"
	got := extractConstraints(body)
	if len(got) != 3 {
		t.Fatalf("extractConstraints() returned %d constraints, want 3", len(got))
	}
}

func TestConstraintsCoveredBulletsVsNumbered(t *testing.T) {
	t.Parallel()
	bullets := "- Do not release without approval.\n- Keep changes targeted.\n"
	numbered := "1. Do not release without approval.\n2. Keep changes targeted.\n"
	if !constraintsCovered(bullets, numbered) {
		t.Fatal("numbered list constraints should match equivalent bullet constraints")
	}
	if !constraintsCovered(numbered, bullets) {
		t.Fatal("bullet constraints should match equivalent numbered list constraints")
	}
}

func TestExtractConstraintsEmpty(t *testing.T) {
	t.Parallel()
	got := extractConstraints("")
	if len(got) != 0 {
		t.Fatalf("extractConstraints('') returned %d constraints, want 0", len(got))
	}
}

func TestConstraintsCoveredSubset(t *testing.T) {
	t.Parallel()
	template := "- Do not release without approval.\n- Keep changes targeted.\n- Surface assumptions.\n"
	reference := "- Do not release without approval.\n- Keep changes targeted.\n"
	if !constraintsCovered(template, reference) {
		t.Fatal("template constraints should cover reference subset")
	}
}

func TestConstraintsCoveredSameKeywordsDifferentConstraints(t *testing.T) {
	t.Parallel()
	template := "- Do not create artifacts or make changes unless the user explicitly authorizes them.\n"
	reference := "- Do not create, deploy, or publish artifacts unless the user explicitly authorizes them; security-sensitive changes require two-person review.\n"
	if constraintsCovered(template, reference) {
		t.Fatal("different constraint text should not be considered covered")
	}
}

func TestConstraintsCoveredIdentical(t *testing.T) {
	t.Parallel()
	body := "- Do not release without approval.\n- Keep changes targeted.\n"
	if !constraintsCovered(body, body) {
		t.Fatal("identical constraints should be covered")
	}
}

func TestConstraintsCoveredEmptyReference(t *testing.T) {
	t.Parallel()
	if !constraintsCovered("- Some constraint.\n", "") {
		t.Fatal("empty reference should be considered covered")
	}
}

func TestGovernanceSameKeywordsDifferentConstraintsProducesCandidate(t *testing.T) {
	t.Parallel()

	templateRoot := t.TempDir()
	referenceRoot := filepath.Join(t.TempDir(), "ref")
	if err := os.MkdirAll(referenceRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	mustWrite(t, filepath.Join(templateRoot, "base", "AGENTS.md"), `# AGENTS.md

## Purpose

Base purpose.

## Interaction Mode

- Treat requests as exploratory discussion unless the user explicitly asks for implementation or file changes.
- Do not create artifacts or make changes unless the user explicitly authorizes them.

## Approval Boundaries

- Do not release without approval.

## Review Style

- Findings first.

## File-Change Discipline

- Prefer targeted edits.

## Release Or Publish Triggers

- Release only on request.

## Documentation Update Expectations

- Update docs with behavior.
`)
	// Same keywords but materially different constraint for Interaction Mode.
	mustWrite(t, filepath.Join(referenceRoot, "AGENTS.md"), `# AGENTS.md

## Purpose

Base purpose.

## Interaction Mode

- Treat requests as exploratory discussion unless the user explicitly asks for implementation or file changes.
- Do not create, deploy, or publish artifacts unless the user explicitly authorizes them; security-sensitive changes require two-person review.

## Approval Boundaries

- Do not release without approval.

## Review Style

- Findings first.

## File-Change Discipline

- Prefer targeted edits.

## Release Or Publish Triggers

- Release only on request.

## Documentation Update Expectations

- Update docs with behavior.
`)

	report, err := ReviewEnhancement(os.DirFS(templateRoot), templateRoot, referenceRoot)
	if err != nil {
		t.Fatalf("ReviewEnhancement() error = %v", err)
	}
	found := false
	for _, c := range report.Candidates {
		if c.Area == "base governance" && c.Section == "Interaction Mode" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected candidate for Interaction Mode with different constraints despite same keywords")
	}
}

// --- AC-006 Phase 1: section-level file diffing ---

func TestDiffMarkdownSectionsOneSectionDiffers(t *testing.T) {
	t.Parallel()
	template := "# Title\n\n## Section A\n\nSame content.\n\n## Section B\n\nTemplate version.\n"
	reference := "# Title\n\n## Section A\n\nSame content.\n\n## Section B\n\nReference version.\n"
	got := diffMarkdownSections(template, reference)
	if len(got) != 1 || got[0] != "Section B" {
		t.Fatalf("diffMarkdownSections() = %v, want [Section B]", got)
	}
}

func TestDiffMarkdownSectionsNewSectionInReference(t *testing.T) {
	t.Parallel()
	template := "# Title\n\n## Section A\n\nContent.\n"
	reference := "# Title\n\n## Section A\n\nContent.\n\n## Section B\n\nNew section.\n"
	got := diffMarkdownSections(template, reference)
	if len(got) != 1 || got[0] != "Section B" {
		t.Fatalf("diffMarkdownSections() = %v, want [Section B]", got)
	}
}

func TestDiffMarkdownSectionsIdentical(t *testing.T) {
	t.Parallel()
	content := "# Title\n\n## Section A\n\nContent.\n\n## Section B\n\nMore.\n"
	got := diffMarkdownSections(content, content)
	if len(got) != 0 {
		t.Fatalf("diffMarkdownSections() = %v, want empty", got)
	}
}

func TestDiffMarkdownSectionsTemplateOnlySection(t *testing.T) {
	t.Parallel()
	template := "# Title\n\n## Section A\n\nContent.\n\n## Section B\n\nExtra.\n"
	reference := "# Title\n\n## Section A\n\nContent.\n"
	got := diffMarkdownSections(template, reference)
	if len(got) != 1 || got[0] != "Section B" {
		t.Fatalf("diffMarkdownSections() = %v, want [Section B] for template-only section", got)
	}
}

func TestDiffMarkdownSectionsNoStructureFallback(t *testing.T) {
	t.Parallel()
	template := "Just plain text without any headings.\n"
	reference := "Different plain text without any headings.\n"
	got := diffMarkdownSections(template, reference)
	if got != nil {
		t.Fatalf("diffMarkdownSections() = %v, want nil for unstructured files", got)
	}
}

func TestReviewMappedFilePopulatesDeltaSections(t *testing.T) {
	t.Parallel()

	templateRoot := t.TempDir()
	referenceRoot := t.TempDir()

	mustWrite(t, filepath.Join(templateRoot, "overlays", "code", "files", "docs", "arch.md.tmpl"),
		"# Arch\n\n## Overview\n\nSame.\n\n## Stack\n\nTemplate stack.\n")
	mustWrite(t, filepath.Join(referenceRoot, "docs", "arch.md"),
		"# Arch\n\n## Overview\n\nSame.\n\n## Stack\n\nDifferent stack info.\n")

	item := enhancementMapping{
		Area:           "CODE overlay",
		ReferencePaths: []string{"docs/arch.md"},
		TemplateTarget: filepath.Join("overlays", "code", "files", "docs", "arch.md.tmpl"),
	}
	candidate, ok, err := reviewMappedFile(os.DirFS(templateRoot), templateRoot, referenceRoot, item, nil)
	if err != nil {
		t.Fatalf("reviewMappedFile() error = %v", err)
	}
	if !ok {
		t.Fatal("expected a candidate")
	}
	if len(candidate.DeltaSections) != 1 || candidate.DeltaSections[0] != "Stack" {
		t.Fatalf("DeltaSections = %v, want [Stack]", candidate.DeltaSections)
	}
}

func TestReviewMappedFileNoDeltaSectionsForUnstructured(t *testing.T) {
	t.Parallel()

	templateRoot := t.TempDir()
	referenceRoot := t.TempDir()

	mustWrite(t, filepath.Join(templateRoot, "TEMPLATE_VERSION"), "0.1.0\n")
	mustWrite(t, filepath.Join(referenceRoot, "TEMPLATE_VERSION"), "0.2.0\n")

	item := enhancementMapping{
		Area:           "upgrade path",
		ReferencePaths: []string{"TEMPLATE_VERSION"},
		TemplateTarget: "TEMPLATE_VERSION",
	}
	candidate, ok, err := reviewMappedFile(os.DirFS(templateRoot), templateRoot, referenceRoot, item, nil)
	if err != nil {
		t.Fatalf("reviewMappedFile() error = %v", err)
	}
	if !ok {
		t.Fatal("expected a candidate")
	}
	if candidate.DeltaSections != nil {
		t.Fatalf("DeltaSections = %v, want nil for unstructured file", candidate.DeltaSections)
	}
}

// --- AC-006 Phase 1: summary and AC doc rendering with delta sections ---

func TestFormatCandidateLineIncludesDeltaSections(t *testing.T) {
	t.Parallel()

	candidate := EnhancementCandidate{
		Area:            "CODE overlay",
		Path:            "/tmp/ref/docs/arch.md",
		Disposition:     "adapt",
		Portability:     "needs-review",
		TemplateTarget:  "overlays/code/files/docs/arch.md.tmpl",
		Summary:         "headings: Arch, Overview, Stack",
		CollisionImpact: "medium",
		DeltaSections:   []string{"Stack", "Dependencies"},
	}

	line := formatCandidateLine(candidate, "/tmp/ref")
	if !strings.Contains(line, "delta-sections=Stack,Dependencies") {
		t.Fatalf("expected delta-sections in line, got:\n%s", line)
	}
}

func TestRenderACDocIncludesDeltaSections(t *testing.T) {
	t.Parallel()

	candidate := EnhancementCandidate{
		Area:            "CODE overlay",
		Path:            "/tmp/ref/docs/arch.md",
		Disposition:     "adapt",
		Reason:          "artifact may contain reusable structure",
		Portability:     "needs-review",
		TemplateTarget:  "overlays/code/files/docs/arch.md.tmpl",
		Summary:         "headings: Arch, Overview, Stack",
		CollisionImpact: "medium",
		DeltaSections:   []string{"Stack", "Dependencies"},
	}
	report := EnhancementReport{ReferenceRoot: "/tmp/ref", Candidates: []EnhancementCandidate{candidate}}
	doc := renderACDoc(candidate, nil, report, 7)

	if !strings.Contains(doc, "Changed sections: Stack, Dependencies") {
		t.Fatalf("expected 'Changed sections' in AC doc summary, got:\n%s", doc)
	}
	if !strings.Contains(doc, "- Section: `Stack`") {
		t.Fatalf("expected Section: Stack in scope, got:\n%s", doc)
	}
	if !strings.Contains(doc, "- Section: `Dependencies`") {
		t.Fatalf("expected Section: Dependencies in scope, got:\n%s", doc)
	}
}

func TestRenderACDocNoDeltaSectionsKeepsLegacySection(t *testing.T) {
	t.Parallel()

	candidate := EnhancementCandidate{
		Area:            "base governance",
		Path:            "/tmp/ref/AGENTS.md",
		Section:         "Interaction Mode",
		Disposition:     "accept",
		Reason:          "governance delta",
		Portability:     "portable",
		TemplateTarget:  "base/AGENTS.md",
		CollisionImpact: "medium",
	}
	report := EnhancementReport{ReferenceRoot: "/tmp/ref", Candidates: []EnhancementCandidate{candidate}}
	doc := renderACDoc(candidate, nil, report, 8)

	if !strings.Contains(doc, "- Section: `Interaction Mode`") {
		t.Fatalf("expected legacy Section field in scope when no DeltaSections, got:\n%s", doc)
	}
	if strings.Contains(doc, "Changed sections") {
		t.Fatalf("should not include 'Changed sections' without DeltaSections")
	}
}

// --- AC-006 Phase 2: bootstrap writes manifest ---

func TestBootstrapNewWritesManifest(t *testing.T) {
	t.Parallel()

	templateRoot, _ := filepath.Abs("../..")
	targetDir := t.TempDir()

	cfg := Config{
		Mode:     ModeSync,
		Type:     RepoTypeCode,
		Target:   targetDir,
		RepoName: "test-repo",
		Purpose:  "test purpose",
		Stack:    "Go CLI",
	}
	if err := runSync(templates.DiskFS(templateRoot), templateRoot, cfg); err != nil {
		t.Fatalf("runSync() error = %v", err)
	}

	manifestPath := filepath.Join(targetDir, manifestFileName)
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	m, err := parseManifest(string(content))
	if err != nil {
		t.Fatalf("parseManifest() error = %v", err)
	}
	if m.TemplateVersion == "" {
		t.Fatal("manifest has empty template version")
	}
	em := manifestEntryMap(m)
	if _, ok := em["AGENTS.md"]; !ok {
		t.Fatal("manifest missing AGENTS.md entry")
	}
	if _, ok := em["TEMPLATE_VERSION"]; !ok {
		t.Fatal("manifest missing TEMPLATE_VERSION entry")
	}
	agents := em["AGENTS.md"]
	if agents.SourcePath == "" || agents.SourceChecksum == "" {
		t.Fatal("AGENTS.md manifest entry missing source info")
	}
}

func TestBootstrapAdoptWritesManifestWithCanonicalChecksums(t *testing.T) {
	t.Parallel()

	templateRoot, _ := filepath.Abs("../..")
	targetDir := t.TempDir()

	// Pre-create AGENTS.md so adopt proposes instead of writing
	mustWrite(t, filepath.Join(targetDir, "AGENTS.md"), "# Existing AGENTS.md\n")
	mustWrite(t, filepath.Join(targetDir, "TEMPLATE_VERSION"), "0.0.1\n")

	cfg := Config{
		Mode:     ModeSync,
		Type:     RepoTypeCode,
		Target:   targetDir,
		RepoName: "test-repo",
		Purpose:  "test purpose",
		Stack:    "Go CLI",
	}
	if err := runSync(templates.DiskFS(templateRoot), templateRoot, cfg); err != nil {
		t.Fatalf("runSync() error = %v", err)
	}

	manifestPath := filepath.Join(targetDir, manifestFileName)
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest not written in adopt mode: %v", err)
	}
	m, err := parseManifest(string(content))
	if err != nil {
		t.Fatalf("parseManifest() error = %v", err)
	}

	em := manifestEntryMap(m)
	// Manifest should have AGENTS.md with canonical checksum (what template would produce),
	// even though adopt mode proposed instead of overwriting the existing file.
	agents, ok := em["AGENTS.md"]
	if !ok {
		t.Fatal("manifest missing AGENTS.md entry in adopt mode")
	}
	if agents.Checksum == "" {
		t.Fatal("AGENTS.md manifest entry has empty checksum")
	}
	if agents.SourcePath == "" || agents.SourceChecksum == "" {
		t.Fatal("AGENTS.md manifest entry missing source info in adopt mode")
	}

	// Verify the checksum is of the canonical (template-rendered) content, not the existing file
	existingChecksum := computeChecksum("# Existing AGENTS.md\n")
	if agents.Checksum == existingChecksum {
		t.Fatal("manifest should record canonical template checksum, not the existing file's checksum")
	}
}

// --- AC-006 Phase 2: three-way enhance with manifest ---

func writeManifestFile(t *testing.T, dir string, m Manifest) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, manifestFileName), []byte(formatManifest(m)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEnhanceWithManifestUserChangeOnly(t *testing.T) {
	t.Parallel()

	templateRoot := t.TempDir()
	referenceRoot := filepath.Join(t.TempDir(), "ref")
	if err := os.MkdirAll(referenceRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	templateAgents := "# AGENTS.md\n\n## Purpose\n\nBase purpose.\n\n## Interaction Mode\n\n- Default to discussion first.\n"
	mustWrite(t, filepath.Join(templateRoot, "base", "AGENTS.md"), templateAgents)

	// Reference has a user modification
	refAgents := "# AGENTS.md\n\n## Purpose\n\nBase purpose.\n\n## Interaction Mode\n\n- Default to discussion first.\n- Added user rule about authorization.\n"
	mustWrite(t, filepath.Join(referenceRoot, "AGENTS.md"), refAgents)

	// Manifest records what the template produced at bootstrap time (matches current template)
	writeManifestFile(t, referenceRoot, Manifest{
		FormatVersion:   manifestFormatVersion,
		TemplateVersion: "0.1.0",
		Entries: []ManifestEntry{
			{Path: "AGENTS.md", Kind: "file", Checksum: computeChecksum(templateAgents), SourcePath: "base/AGENTS.md", SourceChecksum: computeChecksum(templateAgents)},
		},
	})

	report, err := ReviewEnhancement(os.DirFS(templateRoot), templateRoot, referenceRoot)
	if err != nil {
		t.Fatalf("ReviewEnhancement() error = %v", err)
	}

	found := false
	for _, c := range report.Candidates {
		if c.Area == "base governance" && c.Section == "Interaction Mode" {
			found = true
			if c.ChangeOrigin != "user" {
				t.Fatalf("ChangeOrigin = %q, want user", c.ChangeOrigin)
			}
			if c.Disposition == "defer" {
				t.Fatal("user-only change should not be deferred")
			}
		}
	}
	if !found {
		t.Fatal("expected candidate for user-modified Interaction Mode")
	}
}

func TestEnhanceWithManifestTemplateChangeOnly(t *testing.T) {
	t.Parallel()

	templateRoot := t.TempDir()
	referenceRoot := filepath.Join(t.TempDir(), "ref")
	if err := os.MkdirAll(referenceRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	// Old template source (at bootstrap time)
	oldTemplateSource := "# AGENTS old\n"
	// Current template source (evolved)
	newTemplateSource := "# AGENTS updated\n"
	mustWrite(t, filepath.Join(templateRoot, "base", "AGENTS.md"), newTemplateSource)

	// Reference still has the old rendered content (user didn't touch it)
	oldRendered := "# AGENTS old rendered\n"
	mustWrite(t, filepath.Join(referenceRoot, "AGENTS.md"), oldRendered)

	// Manifest: rendered checksum matches current reference, source checksum is OLD template
	writeManifestFile(t, referenceRoot, Manifest{
		FormatVersion:   manifestFormatVersion,
		TemplateVersion: "0.1.0",
		Entries: []ManifestEntry{
			{Path: "AGENTS.md", Kind: "file", Checksum: computeChecksum(oldRendered), SourcePath: "base/AGENTS.md", SourceChecksum: computeChecksum(oldTemplateSource)},
		},
	})

	report, err := ReviewEnhancement(os.DirFS(templateRoot), templateRoot, referenceRoot)
	if err != nil {
		t.Fatalf("ReviewEnhancement() error = %v", err)
	}

	// Template-only change on AGENTS.md → section candidates should be skipped entirely
	for _, c := range report.Candidates {
		if c.Area == "base governance" {
			t.Fatalf("template-only change should produce no governance candidates, got section=%q", c.Section)
		}
	}
}

func TestEnhanceWithManifestBothChanged(t *testing.T) {
	t.Parallel()

	templateRoot := t.TempDir()
	referenceRoot := filepath.Join(t.TempDir(), "ref")
	if err := os.MkdirAll(referenceRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	oldTemplateSource := "# AGENTS.md\n\n## Purpose\n\nOld purpose.\n\n## Interaction Mode\n\n- Old rule.\n"
	newTemplateSource := "# AGENTS.md\n\n## Purpose\n\nNew purpose.\n\n## Interaction Mode\n\n- Updated rule.\n"
	mustWrite(t, filepath.Join(templateRoot, "base", "AGENTS.md"), newTemplateSource)

	oldRendered := "# AGENTS.md\n\n## Purpose\n\nOld purpose.\n\n## Interaction Mode\n\n- Old rule.\n"
	userModified := "# AGENTS.md\n\n## Purpose\n\nOld purpose.\n\n## Interaction Mode\n\n- Old rule.\n- User added constraint.\n"
	mustWrite(t, filepath.Join(referenceRoot, "AGENTS.md"), userModified)

	writeManifestFile(t, referenceRoot, Manifest{
		FormatVersion:   manifestFormatVersion,
		TemplateVersion: "0.1.0",
		Entries: []ManifestEntry{
			{Path: "AGENTS.md", Kind: "file", Checksum: computeChecksum(oldRendered), SourcePath: "base/AGENTS.md", SourceChecksum: computeChecksum(oldTemplateSource)},
		},
	})

	report, err := ReviewEnhancement(os.DirFS(templateRoot), templateRoot, referenceRoot)
	if err != nil {
		t.Fatalf("ReviewEnhancement() error = %v", err)
	}

	found := false
	for _, c := range report.Candidates {
		if c.Area == "base governance" {
			found = true
			if c.ChangeOrigin != "both" {
				t.Fatalf("ChangeOrigin = %q, want both", c.ChangeOrigin)
			}
		}
	}
	if !found {
		t.Fatal("expected candidate when both user and template changed")
	}
}

func TestEnhanceWithManifestNeitherChanged(t *testing.T) {
	t.Parallel()

	templateRoot := t.TempDir()
	referenceRoot := filepath.Join(t.TempDir(), "ref")
	if err := os.MkdirAll(referenceRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	templateSource := "# AGENTS.md\n\n## Purpose\n\nBase purpose.\n\n## Interaction Mode\n\n- Default rule.\n"
	rendered := "# AGENTS.md\n\n## Purpose\n\nBase purpose.\n\n## Interaction Mode\n\n- Default rule.\n"
	mustWrite(t, filepath.Join(templateRoot, "base", "AGENTS.md"), templateSource)
	mustWrite(t, filepath.Join(referenceRoot, "AGENTS.md"), rendered)

	writeManifestFile(t, referenceRoot, Manifest{
		FormatVersion:   manifestFormatVersion,
		TemplateVersion: "0.1.0",
		Entries: []ManifestEntry{
			{Path: "AGENTS.md", Kind: "file", Checksum: computeChecksum(rendered), SourcePath: "base/AGENTS.md", SourceChecksum: computeChecksum(templateSource)},
		},
	})

	report, err := ReviewEnhancement(os.DirFS(templateRoot), templateRoot, referenceRoot)
	if err != nil {
		t.Fatalf("ReviewEnhancement() error = %v", err)
	}

	for _, c := range report.Candidates {
		if c.Area == "base governance" {
			t.Fatal("neither-changed should produce no governance candidates")
		}
	}
}

func TestEnhanceWithoutManifestFallback(t *testing.T) {
	t.Parallel()

	templateRoot := t.TempDir()
	referenceRoot := filepath.Join(t.TempDir(), "ref")
	if err := os.MkdirAll(referenceRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	mustWrite(t, filepath.Join(templateRoot, "base", "AGENTS.md"), "# AGENTS.md\n\n## Purpose\n\nBase purpose.\n\n## Interaction Mode\n\n- Default rule.\n")
	mustWrite(t, filepath.Join(referenceRoot, "AGENTS.md"), "# AGENTS.md\n\n## Purpose\n\nBase purpose.\n\n## Interaction Mode\n\n- Default rule.\n- Extra user rule.\n")

	// No manifest file → two-way fallback
	report, err := ReviewEnhancement(os.DirFS(templateRoot), templateRoot, referenceRoot)
	if err != nil {
		t.Fatalf("ReviewEnhancement() error = %v", err)
	}

	found := false
	for _, c := range report.Candidates {
		if c.Area == "base governance" && c.Section == "Interaction Mode" {
			found = true
			if c.ChangeOrigin != "" {
				t.Fatalf("ChangeOrigin = %q, want empty for no-manifest fallback", c.ChangeOrigin)
			}
		}
	}
	if !found {
		t.Fatal("expected candidate in no-manifest two-way fallback")
	}
}

func TestEnhanceMappedFileTemplateOnlyDeferred(t *testing.T) {
	t.Parallel()

	templateRoot := t.TempDir()
	referenceRoot := t.TempDir()

	oldSource := "# Old README template\n"
	newSource := "# New README template\n"
	mustWrite(t, filepath.Join(templateRoot, "overlays", "code", "files", "README.md.tmpl"), newSource)

	rendered := "# Old README rendered\n"
	mustWrite(t, filepath.Join(referenceRoot, "README.md"), rendered)

	mmap := map[string]ManifestEntry{
		"README.md": {Path: "README.md", Kind: "file", Checksum: computeChecksum(rendered), SourcePath: "overlays/code/files/README.md.tmpl", SourceChecksum: computeChecksum(oldSource)},
	}

	item := enhancementMapping{
		Area:           "CODE overlay",
		ReferencePaths: []string{"README.md"},
		TemplateTarget: filepath.Join("overlays", "code", "files", "README.md.tmpl"),
	}
	candidate, ok, err := reviewMappedFile(os.DirFS(templateRoot), templateRoot, referenceRoot, item, mmap)
	if err != nil {
		t.Fatalf("reviewMappedFile() error = %v", err)
	}
	if !ok {
		t.Fatal("expected a candidate for template-only change")
	}
	if candidate.ChangeOrigin != "template" {
		t.Fatalf("ChangeOrigin = %q, want template", candidate.ChangeOrigin)
	}
	if candidate.Disposition != "defer" {
		t.Fatalf("Disposition = %q, want defer for template-only change", candidate.Disposition)
	}
}

func TestFormatCandidateLineIncludesChangeOrigin(t *testing.T) {
	t.Parallel()

	candidate := EnhancementCandidate{
		Area:            "CODE overlay",
		Path:            "/tmp/ref/README.md",
		Disposition:     "accept",
		Portability:     "portable",
		TemplateTarget:  "overlays/code/files/README.md.tmpl",
		CollisionImpact: "medium",
		ChangeOrigin:    "user",
	}

	line := formatCandidateLine(candidate, "/tmp/ref")
	if !strings.Contains(line, "change-origin=user") {
		t.Fatalf("expected change-origin in line, got:\n%s", line)
	}
}

func TestRenderACDocIncludesChangeOrigin(t *testing.T) {
	t.Parallel()

	candidate := EnhancementCandidate{
		Area:            "CODE overlay",
		Path:            "/tmp/ref/README.md",
		Disposition:     "accept",
		Reason:          "improvement found",
		Portability:     "portable",
		TemplateTarget:  "overlays/code/files/README.md.tmpl",
		CollisionImpact: "medium",
		ChangeOrigin:    "both",
	}
	report := EnhancementReport{ReferenceRoot: "/tmp/ref", Candidates: []EnhancementCandidate{candidate}}
	doc := renderACDoc(candidate, nil, report, 9)

	if !strings.Contains(doc, "Change origin: `both`") {
		t.Fatalf("expected 'Change origin' in AC doc, got:\n%s", doc)
	}
}

// --- AC-006 Phase 3: classifier extensibility ---

func TestClassifyEnhancementDefaultRulesProjectSpecific(t *testing.T) {
	t.Parallel()
	p, d, _ := classifyEnhancement("This mentions skout repo name", "/tmp/skout", "overlays/code/files/README.md.tmpl", false)
	if p != "project-specific" || d != "defer" {
		t.Fatalf("project-specific: got portability=%q disposition=%q", p, d)
	}
}

func TestClassifyEnhancementDefaultRulesGovernance(t *testing.T) {
	t.Parallel()
	p, d, _ := classifyEnhancement("generic content", "/tmp/ref", "base/AGENTS.md", true)
	if p != "portable" || d != "accept" {
		t.Fatalf("governance: got portability=%q disposition=%q", p, d)
	}
}

func TestClassifyEnhancementDefaultRulesWorkflowHelper(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"overlays/code/files/cmd/build/main.go.tmpl", "overlays/code/files/build.sh.tmpl", "TEMPLATE_VERSION"} {
		p, d, _ := classifyEnhancement("generic content", "/tmp/ref", target, false)
		if p != "portable" || d != "accept" {
			t.Fatalf("workflow helper %q: got portability=%q disposition=%q", target, p, d)
		}
	}
}

func TestClassifyEnhancementDefaultRulesFallback(t *testing.T) {
	t.Parallel()
	p, d, _ := classifyEnhancement("generic content", "/tmp/ref", "overlays/code/files/README.md.tmpl", false)
	if p != "needs-review" || d != "adapt" {
		t.Fatalf("fallback: got portability=%q disposition=%q", p, d)
	}
}

func TestProjectSpecificMarkersRepoName(t *testing.T) {
	t.Parallel()
	markers := projectSpecificMarkers("This mentions skout in the content", "/tmp/skout")
	if len(markers) != 1 || markers[0] != "mentions reference repo name" {
		t.Fatalf("markers = %v, want [mentions reference repo name]", markers)
	}
}

func TestProjectSpecificMarkersAbsolutePath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
	}{
		{"macOS", "path is /Users/<user>/project"},
		{"Linux", "path is /home/<user>/project"},
		{"Windows", "path is C:\\Users\\<user>\\project"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			markers := projectSpecificMarkers(tc.content, "/tmp/ref")
			if len(markers) != 1 || markers[0] != "contains absolute user path" {
				t.Fatalf("markers = %v, want [contains absolute user path]", markers)
			}
		})
	}
}

func TestProjectSpecificMarkersNone(t *testing.T) {
	t.Parallel()
	markers := projectSpecificMarkers("generic content", "/tmp/ref")
	if len(markers) != 0 {
		t.Fatalf("markers = %v, want empty", markers)
	}
}

func TestCustomClassificationRuleOverridesDefault(t *testing.T) {
	// No t.Parallel() — mutates package-global defaultClassificationRules
	original := defaultClassificationRules
	defer func() { defaultClassificationRules = original }()

	custom := classificationRule{
		Name:        "custom-override",
		Priority:    50, // lower than project-specific (100), so it wins
		Match:       func(ctx classificationContext) bool { return strings.Contains(ctx.Content, "CUSTOM_MARKER") },
		Portability: "portable", Disposition: "accept",
		Reason: "custom rule matched",
	}
	defaultClassificationRules = append(defaultClassificationRules, custom)

	p, d, r := classifyEnhancement("content with CUSTOM_MARKER here", "/tmp/ref", "overlays/code/files/README.md.tmpl", false)
	if p != "portable" || d != "accept" || r != "custom rule matched" {
		t.Fatalf("custom rule: got portability=%q disposition=%q reason=%q", p, d, r)
	}

	// Without the marker, falls through to default
	p2, d2, _ := classifyEnhancement("generic content", "/tmp/ref", "overlays/code/files/README.md.tmpl", false)
	if p2 != "needs-review" || d2 != "adapt" {
		t.Fatalf("fallthrough: got portability=%q disposition=%q", p2, d2)
	}
}

func TestCustomMarkerRuleEvaluated(t *testing.T) {
	// No t.Parallel() — mutates package-global defaultMarkerRules
	original := defaultMarkerRules
	defer func() { defaultMarkerRules = original }()

	defaultMarkerRules = append(defaultMarkerRules, markerRule{
		Name: "contains secret path",
		Match: func(content, _ string) bool {
			return strings.Contains(content, ".secrets/")
		},
	})

	markers := projectSpecificMarkers("found .secrets/ in config", "/tmp/ref")
	found := false
	for _, m := range markers {
		if m == "contains secret path" {
			found = true
		}
	}
	if !found {
		t.Fatalf("custom marker not found, markers = %v", markers)
	}
}

func TestCustomSignalDefRecognized(t *testing.T) {
	// No t.Parallel() — mutates package-global defaultSignalDefs
	original := defaultSignalDefs["Review Style"]
	defer func() { defaultSignalDefs["Review Style"] = original }()

	defaultSignalDefs["Review Style"] = append(defaultSignalDefs["Review Style"], signalDef{
		Name:  "review-security",
		Match: func(t string) bool { return containsAny(t, "security", "vulnerability") },
	})

	signals := sectionSignals("Review Style", "- Check for security vulnerabilities before merging.")
	if !signals["review-security"] {
		t.Fatalf("custom signal not detected, signals = %v", signals)
	}
}

func TestSectionSignalsUnknownSectionReturnsEmpty(t *testing.T) {
	t.Parallel()
	signals := sectionSignals("Nonexistent Section", "some body text")
	if len(signals) != 0 {
		t.Fatalf("unknown section should return empty signals, got %v", signals)
	}
}

func TestClassificationRulesNoMatchFallsThrough(t *testing.T) {
	// No t.Parallel() — mutates package-global defaultClassificationRules
	original := defaultClassificationRules
	defer func() { defaultClassificationRules = original }()

	// Replace with rules that never match (except catch-all)
	defaultClassificationRules = []classificationRule{
		{Name: "never-match", Match: func(_ classificationContext) bool { return false }, Portability: "x", Disposition: "x", Reason: "x"},
		{Name: "catch-all", Match: func(_ classificationContext) bool { return true }, Portability: "needs-review", Disposition: "adapt", Reason: "catch-all"},
	}

	p, d, r := classifyEnhancement("anything", "/tmp/ref", "any-target", false)
	if p != "needs-review" || d != "adapt" || r != "catch-all" {
		t.Fatalf("catch-all: got portability=%q disposition=%q reason=%q", p, d, r)
	}
}

// --- AC-007: adopt section-level patching ---

func TestPatchGovernedSectionsMissingSections(t *testing.T) {
	t.Parallel()
	existing := "# AGENTS.md\n\n## Purpose\n\nExisting purpose.\n"
	template := "# AGENTS.md\n\n## Purpose\n\nTemplate purpose.\n\n## Interaction Mode\n\n- Default to discussion.\n\n## Review Style\n\n- Findings first.\n"

	patched, changed := patchGovernedSections(existing, template)
	if !changed {
		t.Fatal("expected patching to add missing sections")
	}
	if !strings.Contains(patched, "## Interaction Mode") {
		t.Fatal("patched content should include missing Interaction Mode")
	}
	if !strings.Contains(patched, "## Review Style") {
		t.Fatal("patched content should include missing Review Style")
	}
	// Existing section content should be preserved
	if !strings.Contains(patched, "Existing purpose.") {
		t.Fatal("patched content should preserve existing Purpose body")
	}
	if strings.Contains(patched, "Template purpose.") {
		t.Fatal("patched content should NOT replace existing Purpose with template version")
	}
}

func TestPatchGovernedSectionsAllPresent(t *testing.T) {
	t.Parallel()
	existing := "# AGENTS.md\n\n## Purpose\n\nP.\n\n## Governed Sections\n\nG.\n\n## Interaction Mode\n\nI.\n\n## Approval Boundaries\n\nA.\n\n## Review Style\n\nR.\n\n## File-Change Discipline\n\nF.\n\n## Release Or Publish Triggers\n\nT.\n\n## Documentation Update Expectations\n\nD.\n"
	template := "# AGENTS.md\n\n## Purpose\n\nDifferent.\n"

	_, changed := patchGovernedSections(existing, template)
	if changed {
		t.Fatal("expected no patching when all governed sections present")
	}
}

func TestPatchGovernedSectionsPreservesNonGoverned(t *testing.T) {
	t.Parallel()
	existing := "# AGENTS.md\n\n## Purpose\n\nP.\n\n## Custom Section\n\nUser content.\n"
	template := "# AGENTS.md\n\n## Purpose\n\nP.\n\n## Interaction Mode\n\n- Default.\n"

	patched, changed := patchGovernedSections(existing, template)
	if !changed {
		t.Fatal("expected patching")
	}
	if !strings.Contains(patched, "## Custom Section") {
		t.Fatal("patched content should preserve non-governed sections")
	}
	if !strings.Contains(patched, "User content.") {
		t.Fatal("patched content should preserve non-governed section body")
	}
}

func TestPatchGovernedSectionsPreservesPreamble(t *testing.T) {
	t.Parallel()
	existing := "# My Custom Title\n\nSome intro text.\n\n## Purpose\n\nP.\n"
	template := "# AGENTS.md\n\n## Purpose\n\nP.\n\n## Interaction Mode\n\n- Default.\n"

	patched, changed := patchGovernedSections(existing, template)
	if !changed {
		t.Fatal("expected patching")
	}
	if !strings.HasPrefix(patched, "# My Custom Title\n\nSome intro text.") {
		t.Fatalf("patched content should preserve preamble, got:\n%s", patched)
	}
}

func TestPatchGovernedSectionsAppendsInTemplateOrder(t *testing.T) {
	t.Parallel()
	existing := "# AGENTS.md\n\n## Purpose\n\nP.\n"
	template := "# AGENTS.md\n\n## Purpose\n\nP.\n\n## Documentation Update Expectations\n\nD.\n\n## Interaction Mode\n\nI.\n\n## Review Style\n\nR.\n"

	patched, changed := patchGovernedSections(existing, template)
	if !changed {
		t.Fatal("expected patching")
	}
	// Should appear in governedSectionNames order: Interaction Mode before Review Style before Documentation Update Expectations
	imIdx := strings.Index(patched, "## Interaction Mode")
	rsIdx := strings.Index(patched, "## Review Style")
	duIdx := strings.Index(patched, "## Documentation Update Expectations")
	if imIdx < 0 || rsIdx < 0 || duIdx < 0 {
		t.Fatalf("missing sections in patched output:\n%s", patched)
	}
	if imIdx > rsIdx || rsIdx > duIdx {
		t.Fatalf("missing sections should be in template governed order, got IM=%d RS=%d DU=%d", imIdx, rsIdx, duIdx)
	}
}

func TestPatchGovernedSectionsNeverModifiesExisting(t *testing.T) {
	t.Parallel()
	existing := "# AGENTS.md\n\n## Purpose\n\nUser custom purpose.\n\n## Interaction Mode\n\n- User custom rule.\n"
	template := "# AGENTS.md\n\n## Purpose\n\nTemplate purpose.\n\n## Interaction Mode\n\n- Template rule.\n\n## Review Style\n\n- Findings first.\n"

	patched, changed := patchGovernedSections(existing, template)
	if !changed {
		t.Fatal("expected patching for missing Review Style")
	}
	if !strings.Contains(patched, "User custom purpose.") {
		t.Fatal("should not modify existing Purpose")
	}
	if !strings.Contains(patched, "User custom rule.") {
		t.Fatal("should not modify existing Interaction Mode")
	}
	if strings.Contains(patched, "Template purpose.") || strings.Contains(patched, "Template rule.") {
		t.Fatal("should not inject template content over existing sections")
	}
}

func TestAdoptPatchesMissingSections(t *testing.T) {
	t.Parallel()

	templateRoot, _ := filepath.Abs("../..")
	targetDir := t.TempDir()

	// Pre-create AGENTS.md with only Purpose section
	mustWrite(t, filepath.Join(targetDir, "AGENTS.md"), "# AGENTS.md\n\n## Purpose\n\nExisting purpose.\n")

	cfg := Config{
		Mode:     ModeSync,
		Type:     RepoTypeCode,
		Target:   targetDir,
		RepoName: "test-repo",
		Purpose:  "test purpose",
		Stack:    "Go CLI",
	}
	if err := runSync(templates.DiskFS(templateRoot), templateRoot, cfg); err != nil {
		t.Fatalf("runSync() error = %v", err)
	}

	// Original file should be untouched
	original, _ := os.ReadFile(filepath.Join(targetDir, "AGENTS.md"))
	if !strings.Contains(string(original), "Existing purpose.") {
		t.Fatal("original AGENTS.md should be preserved")
	}

	// No .template-proposed file should exist
	if _, err := os.Stat(filepath.Join(targetDir, "AGENTS.template-proposed.md")); err == nil {
		t.Fatal("should not create .template-proposed file")
	}

	// Review doc should exist at repo root
	reviewPath := filepath.Join(targetDir, "governa-sync-review.md")
	content, err := os.ReadFile(reviewPath)
	if err != nil {
		t.Fatalf("expected governa-sync-review.md at repo root, got error: %v", err)
	}
	if !strings.Contains(string(content), "AGENTS.md") {
		t.Fatal("review doc should reference AGENTS.md")
	}
	if !strings.Contains(string(content), "review") {
		t.Fatal("review doc should recommend review for AGENTS.md with missing sections")
	}
}

func TestAdoptSkipsWhenAllSectionsPresent(t *testing.T) {
	t.Parallel()

	templateRoot, _ := filepath.Abs("../..")
	targetDir := t.TempDir()

	// Read the template AGENTS.md to get all governed sections
	templateAgents, _ := os.ReadFile(filepath.Join(templateRoot, "internal", "templates", "base", "AGENTS.md"))
	mustWrite(t, filepath.Join(targetDir, "AGENTS.md"), string(templateAgents))

	cfg := Config{
		Mode:     ModeSync,
		Type:     RepoTypeCode,
		Target:   targetDir,
		RepoName: "test-repo",
		Purpose:  "test purpose",
		Stack:    "Go CLI",
	}
	if err := runSync(templates.DiskFS(templateRoot), templateRoot, cfg); err != nil {
		t.Fatalf("runSync() error = %v", err)
	}

	// No .template-proposed should exist
	if _, err := os.Stat(filepath.Join(targetDir, "AGENTS.template-proposed.md")); err == nil {
		t.Fatal("should not create .template-proposed file when all governed sections present")
	}
}

func TestAdoptNoExistingAgentsWritesDirectly(t *testing.T) {
	t.Parallel()

	templateRoot, _ := filepath.Abs("../..")
	targetDir := t.TempDir()

	cfg := Config{
		Mode:     ModeSync,
		Type:     RepoTypeCode,
		Target:   targetDir,
		RepoName: "test-repo",
		Purpose:  "test purpose",
		Stack:    "Go CLI",
	}
	if err := runSync(templates.DiskFS(templateRoot), templateRoot, cfg); err != nil {
		t.Fatalf("runSync() error = %v", err)
	}

	// AGENTS.md should be written directly (no proposal)
	content, err := os.ReadFile(filepath.Join(targetDir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("expected AGENTS.md to be written directly, got error: %v", err)
	}
	if !strings.Contains(string(content), "## Interaction Mode") {
		t.Fatal("directly written AGENTS.md should have full template content")
	}

	if _, err := os.Stat(filepath.Join(targetDir, "AGENTS.template-proposed.md")); err == nil {
		t.Fatal("should not create .template-proposed when no existing AGENTS.md")
	}
}

// --- AC-008: agent role bootstrap ---

func TestBootstrapNewProducesAgentRoles(t *testing.T) {
	t.Parallel()

	templateRoot, _ := filepath.Abs("../..")
	targetDir := t.TempDir()

	cfg := Config{
		Mode:     ModeSync,
		Type:     RepoTypeCode,
		Target:   targetDir,
		RepoName: "test-repo",
		Purpose:  "test purpose",
		Stack:    "Go CLI",
	}
	if err := runSync(templates.DiskFS(templateRoot), templateRoot, cfg); err != nil {
		t.Fatalf("runSync() error = %v", err)
	}

	for _, rel := range []string{
		filepath.Join("docs", "roles", "README.md"),
		filepath.Join("docs", "roles", "dev.md"),
		filepath.Join("docs", "roles", "qa.md"),
		filepath.Join("docs", "roles", "maintainer.md"),
	} {
		path := filepath.Join(targetDir, rel)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist, got error: %v", rel, err)
		}
	}

	devContent, _ := os.ReadFile(filepath.Join(targetDir, "docs", "roles", "dev.md"))
	if !strings.Contains(string(devContent), "test coverage") {
		t.Fatal("dev.md should state the test coverage requirement")
	}
	qaContent, _ := os.ReadFile(filepath.Join(targetDir, "docs", "roles", "qa.md"))
	if !strings.Contains(string(qaContent), "QA says") {
		t.Fatal("qa.md should state the QA says prefix requirement")
	}
	maintContent, _ := os.ReadFile(filepath.Join(targetDir, "docs", "roles", "maintainer.md"))
	if !strings.Contains(string(maintContent), "MAINT says:") {
		t.Fatal("maintainer.md should state the MAINT says: prefix")
	}
	if !strings.Contains(string(maintContent), "self-review") {
		t.Fatal("maintainer.md should require self-review")
	}
	if strings.Contains(string(maintContent), "Propagate fixes") {
		t.Fatal("maintainer.md should NOT contain governa-specific propagation rule in consumer repos")
	}
}

func TestBootstrapAdoptProposesAgentRoles(t *testing.T) {
	t.Parallel()

	templateRoot, _ := filepath.Abs("../..")
	targetDir := t.TempDir()

	// Pre-create roles files so adopt proposes them
	mustWrite(t, filepath.Join(targetDir, "docs", "roles", "README.md"), "# Existing roles index\n")
	mustWrite(t, filepath.Join(targetDir, "docs", "roles", "dev.md"), "# Existing dev role\n")
	mustWrite(t, filepath.Join(targetDir, "docs", "roles", "qa.md"), "# Existing qa role\n")
	mustWrite(t, filepath.Join(targetDir, "docs", "roles", "maintainer.md"), "# Existing maintainer role\n")
	mustWrite(t, filepath.Join(targetDir, "AGENTS.md"), "# Existing AGENTS.md\n\n## Purpose\n\nP.\n\n## Governed Sections\n\nG.\n\n## Interaction Mode\n\nI.\n\n## Approval Boundaries\n\nA.\n\n## Review Style\n\nR.\n\n## File-Change Discipline\n\nF.\n\n## Release Or Publish Triggers\n\nT.\n\n## Documentation Update Expectations\n\nD.\n")

	cfg := Config{
		Mode:     ModeSync,
		Type:     RepoTypeCode,
		Target:   targetDir,
		RepoName: "test-repo",
		Purpose:  "test purpose",
		Stack:    "Go CLI",
	}
	if err := runSync(templates.DiskFS(templateRoot), templateRoot, cfg); err != nil {
		t.Fatalf("runSync() error = %v", err)
	}

	// Existing files should be preserved
	devContent, _ := os.ReadFile(filepath.Join(targetDir, "docs", "roles", "dev.md"))
	if !strings.Contains(string(devContent), "Existing dev role") {
		t.Fatal("adopt should preserve existing dev.md")
	}

	// No .template-proposed files should exist
	for _, rel := range []string{
		filepath.Join("docs", "roles", "README.md"),
		filepath.Join("docs", "roles", "dev.md"),
		filepath.Join("docs", "roles", "qa.md"),
		filepath.Join("docs", "roles", "maintainer.md"),
	} {
		proposed := filepath.Join(targetDir, strings.TrimSuffix(rel, ".md")+".template-proposed.md")
		if _, err := os.Stat(proposed); err == nil {
			t.Fatalf("should not create .template-proposed for %s", rel)
		}
	}

	// Review doc should exist at repo root
	reviewPath := filepath.Join(targetDir, "governa-sync-review.md")
	if _, err := os.Stat(reviewPath); err != nil {
		t.Fatalf("expected governa-sync-review.md, got error: %v", err)
	}
}

// --- AC-011: CODE overlay enrichment ---

func TestBootstrapNewProducesEnrichedDocs(t *testing.T) {
	t.Parallel()

	templateRoot, _ := filepath.Abs("../..")
	targetDir := t.TempDir()

	cfg := Config{
		Mode:     ModeSync,
		Type:     RepoTypeCode,
		Target:   targetDir,
		RepoName: "test-repo",
		Purpose:  "test purpose",
		Stack:    "Go CLI",
	}
	if err := runSync(templates.DiskFS(templateRoot), templateRoot, cfg); err != nil {
		t.Fatalf("runSync() error = %v", err)
	}

	// ac-example.md should NOT exist (removed in AC29)
	if _, err := os.Stat(filepath.Join(targetDir, "docs", "ac-example.md")); err == nil {
		t.Fatal("ac-example.md should not be generated")
	}

	// build-release.md should contain new sections
	brPath := filepath.Join(targetDir, "docs", "build-release.md")
	brContent, err := os.ReadFile(brPath)
	if err != nil {
		t.Fatalf("expected docs/build-release.md, got error: %v", err)
	}
	if !strings.Contains(string(brContent), "## Template Upgrade") {
		t.Fatal("build-release.md should contain Template Upgrade section")
	}
	if !strings.Contains(string(brContent), "## Pre-Release Checklist") {
		t.Fatal("build-release.md should contain Pre-Release Checklist section")
	}
}

func TestBootstrapAdoptProposesEnrichedDocs(t *testing.T) {
	t.Parallel()

	templateRoot, _ := filepath.Abs("../..")
	targetDir := t.TempDir()

	// Pre-create files so adopt collides on them
	mustWrite(t, filepath.Join(targetDir, "docs", "build-release.md"), "# Old build release\n")
	mustWrite(t, filepath.Join(targetDir, "AGENTS.md"), "# AGENTS.md\n\n## Purpose\n\nP.\n\n## Governed Sections\n\nG.\n\n## Interaction Mode\n\nI.\n\n## Approval Boundaries\n\nA.\n\n## Review Style\n\nR.\n\n## File-Change Discipline\n\nF.\n\n## Release Or Publish Triggers\n\nT.\n\n## Documentation Update Expectations\n\nD.\n")

	cfg := Config{
		Mode:     ModeSync,
		Type:     RepoTypeCode,
		Target:   targetDir,
		RepoName: "test-repo",
		Purpose:  "test purpose",
		Stack:    "Go CLI",
	}
	if err := runSync(templates.DiskFS(templateRoot), templateRoot, cfg); err != nil {
		t.Fatalf("runSync() error = %v", err)
	}

	// Review doc should exist at repo root with collision entries
	reviewPath := filepath.Join(targetDir, "governa-sync-review.md")
	content, _ := os.ReadFile(reviewPath)
	if !strings.Contains(string(content), "build-release.md") {
		t.Fatal("review doc should reference colliding files")
	}
}

// --- AC-012: DOC overlay enrichment ---

func TestBootstrapNewDocProducesEnrichedFiles(t *testing.T) {
	t.Parallel()

	templateRoot, _ := filepath.Abs("../..")
	targetDir := t.TempDir()

	cfg := Config{
		Mode:               ModeSync,
		Type:               RepoTypeDoc,
		Target:             targetDir,
		RepoName:           "test-doc",
		Purpose:            "test purpose",
		PublishingPlatform: "Hugo",
		Style:              "concise",
	}
	if err := runSync(templates.DiskFS(templateRoot), templateRoot, cfg); err != nil {
		t.Fatalf("runSync() error = %v", err)
	}

	// publishing-workflow.md should contain platform notes
	pw, _ := os.ReadFile(filepath.Join(targetDir, "publishing-workflow.md"))
	if !strings.Contains(string(pw), "Platform-Specific Notes") {
		t.Fatal("publishing-workflow.md should contain Platform-Specific Notes")
	}

	// variant files should exist
	for _, rel := range []string{"voice.md", "calendar.md"} {
		if _, err := os.Stat(filepath.Join(targetDir, rel)); err != nil {
			t.Fatalf("expected %s to exist, got error: %v", rel, err)
		}
	}

	// roles should exist
	for _, rel := range []string{
		filepath.Join("docs", "roles", "dev.md"),
		filepath.Join("docs", "roles", "qa.md"),
		filepath.Join("docs", "roles", "maintainer.md"),
	} {
		if _, err := os.Stat(filepath.Join(targetDir, rel)); err != nil {
			t.Fatalf("expected %s to exist, got error: %v", rel, err)
		}
	}

	// DOC dev role should use editorial language, not build language
	devRole, _ := os.ReadFile(filepath.Join(targetDir, "docs", "roles", "dev.md"))
	if strings.Contains(string(devRole), "build command") {
		t.Fatal("DOC dev.md should not reference build commands")
	}
	if !strings.Contains(string(devRole), "publishing workflow") {
		t.Fatal("DOC dev.md should reference publishing workflow")
	}
}

func TestBootstrapAdoptDocProposesEnrichedFiles(t *testing.T) {
	t.Parallel()

	templateRoot, _ := filepath.Abs("../..")
	targetDir := t.TempDir()

	mustWrite(t, filepath.Join(targetDir, "voice.md"), "# Old voice\n")
	mustWrite(t, filepath.Join(targetDir, "calendar.md"), "# Old calendar\n")
	mustWrite(t, filepath.Join(targetDir, "publishing-workflow.md"), "# Old workflow\n")
	mustWrite(t, filepath.Join(targetDir, "docs", "roles", "dev.md"), "# Old dev\n")
	mustWrite(t, filepath.Join(targetDir, "AGENTS.md"), "# AGENTS.md\n\n## Purpose\n\nP.\n\n## Governed Sections\n\nG.\n\n## Interaction Mode\n\nI.\n\n## Approval Boundaries\n\nA.\n\n## Review Style\n\nR.\n\n## File-Change Discipline\n\nF.\n\n## Release Or Publish Triggers\n\nT.\n\n## Documentation Update Expectations\n\nD.\n")

	cfg := Config{
		Mode:               ModeSync,
		Type:               RepoTypeDoc,
		Target:             targetDir,
		RepoName:           "test-doc",
		Purpose:            "test purpose",
		PublishingPlatform: "Hugo",
		Style:              "concise",
	}
	if err := runSync(templates.DiskFS(templateRoot), templateRoot, cfg); err != nil {
		t.Fatalf("runSync() error = %v", err)
	}

	// No .template-proposed files should exist
	for _, rel := range []string{
		"voice.md",
		"calendar.md",
		"publishing-workflow.md",
		filepath.Join("docs", "roles", "dev.md"),
	} {
		ext := filepath.Ext(rel)
		name := strings.TrimSuffix(rel, ext)
		proposed := filepath.Join(targetDir, name+".template-proposed"+ext)
		if _, err := os.Stat(proposed); err == nil {
			t.Fatalf("should not create .template-proposed for %s", rel)
		}
	}

	// Review doc should exist at repo root
	reviewPath := filepath.Join(targetDir, "governa-sync-review.md")
	if _, err := os.Stat(reviewPath); err != nil {
		t.Fatal("expected governa-sync-review.md at repo root")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

// --- AC14: Ideas To Explore structural contract ---

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (no go.mod found walking up)")
		}
		dir = parent
	}
}

func readRepoFile(t *testing.T, relPath string) string {
	t.Helper()
	full := filepath.Join(repoRoot(t), relPath)
	content, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", full, err)
	}
	return string(content)
}

func assertSectionOrdering(t *testing.T, content, label string, sections ...string) {
	t.Helper()
	prev := -1
	prevName := ""
	for _, section := range sections {
		idx := strings.Index(content, section)
		if idx < 0 {
			t.Fatalf("%s: section %q not found", label, section)
		}
		if idx <= prev {
			t.Fatalf("%s: section %q (at %d) must come after %q (at %d)", label, section, idx, prevName, prev)
		}
		prev = idx
		prevName = section
	}
}

func TestPlanMdHasIdeasToExploreSection(t *testing.T) {
	t.Parallel()
	content := readRepoFile(t, "plan.md")
	assertSectionOrdering(t, content, "plan.md",
		"## Priorities",
		"## Ideas To Explore",
		"## Deferred",
	)
	if !strings.Contains(content, "Pre-rubric ideas captured for future discussion") {
		t.Fatal("plan.md: Ideas To Explore preamble missing")
	}
	if !strings.Contains(content, "pre-rubric staging, not a historical record") {
		t.Fatal("plan.md: pruning guidance missing")
	}
}

func TestCodeOverlayPlanTemplateHasIdeasToExploreSection(t *testing.T) {
	t.Parallel()
	content := readRepoFile(t, "internal/templates/overlays/code/files/plan.md.tmpl")
	assertSectionOrdering(t, content, "overlays/code/files/plan.md.tmpl",
		"## Priorities",
		"## Ideas To Explore",
		"## Deferred",
	)
	if !strings.Contains(content, "Pre-rubric ideas captured for future discussion") {
		t.Fatal("CODE plan template: Ideas To Explore preamble missing")
	}
}

func TestCodeRenderedExamplePlanHasIdeasToExploreSection(t *testing.T) {
	t.Parallel()
	content := readRepoFile(t, "examples/code/plan.md")
	assertSectionOrdering(t, content, "examples/code/plan.md",
		"## Priorities",
		"## Ideas To Explore",
		"## Deferred",
	)
	if !strings.Contains(content, "Pre-rubric ideas captured for future discussion") {
		t.Fatal("CODE rendered example plan: Ideas To Explore preamble missing")
	}
}

func TestDevelopmentCycleMentionsPriorities(t *testing.T) {
	t.Parallel()
	paths := []string{
		"docs/development-cycle.md",
		"internal/templates/overlays/code/files/docs/development-cycle.md.tmpl",
		"examples/code/docs/development-cycle.md",
	}
	for _, path := range paths {
		content := readRepoFile(t, path)
		if !strings.Contains(content, "`Priorities`") {
			t.Errorf("%s: should reference `Priorities` to make the source of approved items explicit", path)
		}
		if !strings.Contains(content, "`Ideas To Explore`") {
			t.Errorf("%s: should reference `Ideas To Explore` to name the boundary in the workflow doc", path)
		}
		if !strings.Contains(content, "pre-rubric follow-on ideas") {
			t.Errorf("%s: should direct pre-rubric follow-on ideas to Ideas To Explore", path)
		}
	}
}

func TestACFilenameConventionSurfacedInDocs(t *testing.T) {
	t.Parallel()
	hintFiles := []string{
		"docs/ac-template.md",
		"docs/development-cycle.md",
		"internal/templates/overlays/code/files/docs/ac-template.md.tmpl",
		"internal/templates/overlays/code/files/docs/development-cycle.md.tmpl",
		"examples/code/docs/ac-template.md",
		"examples/code/docs/development-cycle.md",
	}
	for _, path := range hintFiles {
		content := readRepoFile(t, path)
		if !strings.Contains(content, "ac<N>-<slug>.md") {
			t.Errorf("%s: should contain literal `ac<N>-<slug>.md` so contributors can name AC files from docs alone", path)
		}
	}
}

func TestACDocsReadmeUsesCurrentConvention(t *testing.T) {
	t.Parallel()
	overlayReadmes := []string{
		"internal/templates/overlays/code/files/docs/README.md.tmpl",
		"examples/code/docs/README.md",
	}
	for _, path := range overlayReadmes {
		content := readRepoFile(t, path)
		if !strings.Contains(content, "ac<N>-<slug>.md") {
			t.Errorf("%s: should describe AC files using current `ac<N>-<slug>.md` convention", path)
		}
		if strings.Contains(content, "ac_<id>") {
			t.Errorf("%s: should NOT use stale underscore form `ac_<id>` (replaced by `ac<N>-<slug>.md` in AC13)", path)
		}
		if strings.Contains(content, "critique") {
			t.Errorf("%s: should NOT mention AC critiques (no critique files exist in repo; bullet was dropped in AC15)", path)
		}
	}

	selfHosted := readRepoFile(t, "docs/README.md")
	if !strings.Contains(selfHosted, "ac<N>-<slug>.md") {
		t.Error("docs/README.md: should describe AC files using current `ac<N>-<slug>.md` convention")
	}
	if strings.Contains(selfHosted, "`ac-*.md`") {
		t.Error("docs/README.md: should NOT use stale glob `ac-*.md` (matches keepers but not working AC files post-AC13)")
	}
}

func TestReadmeConsolidatedStructure(t *testing.T) {
	t.Parallel()
	content := readRepoFile(t, "README.md")

	// New headers must be present
	for _, header := range []string{"## Modes", "## Design", "## Self-Hosting Status", "## Rendered Examples"} {
		if !strings.Contains(content, header) {
			t.Errorf("README.md: missing required section %q", header)
		}
	}

	// Old headers must be gone
	for _, header := range []string{"## Quick Start", "## Intended Use", "## Operating Model", "## Operator Guide"} {
		if strings.Contains(content, header) {
			t.Errorf("README.md: should NOT contain removed section %q", header)
		}
	}

	// All mode command examples present (subcommand form)
	for _, marker := range []string{"governa sync", "governa enhance"} {
		if !strings.Contains(content, marker) {
			t.Errorf("README.md: missing command example containing %q", marker)
		}
	}

	// Install section
	if !strings.Contains(content, "go install github.com/kquo/governa/cmd/governa@latest") {
		t.Error("README.md: should contain go install command")
	}

	// Help pointer
	if !strings.Contains(content, "governa help") {
		t.Error("README.md: should contain `governa help` pointer")
	}
}

func TestIdeasToExploreIEPrefix(t *testing.T) {
	t.Parallel()

	// Self-hosted plan.md must document the IE prefix convention and use it
	content := readRepoFile(t, "plan.md")
	if !strings.Contains(content, "`IE<N>:`") {
		t.Error("plan.md: Ideas To Explore preamble should document the `IE<N>:` prefix convention")
	}

	// Overlay template and rendered example must document the convention
	for _, path := range []string{
		"internal/templates/overlays/code/files/plan.md.tmpl",
		"examples/code/plan.md",
	} {
		c := readRepoFile(t, path)
		if !strings.Contains(c, "`IE<N>:`") {
			t.Errorf("%s: Ideas To Explore preamble should document the `IE<N>:` prefix convention", path)
		}
	}

	// Development cycle docs must reference IE prefix and cleanup rule
	for _, path := range []string{
		"docs/development-cycle.md",
		"internal/templates/overlays/code/files/docs/development-cycle.md.tmpl",
		"examples/code/docs/development-cycle.md",
	} {
		c := readRepoFile(t, path)
		if !strings.Contains(c, "`IE<N>:`") {
			t.Errorf("%s: should reference `IE<N>:` prefix for Ideas To Explore entries", path)
		}
		if !strings.Contains(c, "IE entry") {
			t.Errorf("%s: promotion path should reference IE entries", path)
		}
		if !strings.Contains(c, "remove IE entries when promoted") {
			t.Errorf("%s: should state the IE cleanup rule (remove when promoted or completed)", path)
		}
	}

	// Plan.md preambles must state the cleanup rule
	for _, path := range []string{
		"plan.md",
		"internal/templates/overlays/code/files/plan.md.tmpl",
		"examples/code/plan.md",
	} {
		c := readRepoFile(t, path)
		if !strings.Contains(c, "pre-rubric staging, not a historical record") {
			t.Errorf("%s: Ideas To Explore preamble should state that the list is staging, not history", path)
		}
	}

	// Self-hosted plan.md should not contain completed IE5
	if strings.Contains(content, "IE5:") {
		t.Error("plan.md: IE5 was completed and should be removed")
	}
}

func TestWhySectionInReadmeTemplates(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"internal/templates/overlays/code/files/README.md.tmpl": "CODE template",
		"internal/templates/overlays/doc/files/README.md.tmpl":  "DOC template",
		"examples/code/README.md":                               "CODE rendered example",
		"examples/doc/README.md":                                "DOC rendered example",
	}
	for path, label := range files {
		content := readRepoFile(t, path)
		whyIdx := strings.Index(content, "## Why")
		overviewIdx := strings.Index(content, "## Overview")
		if whyIdx < 0 {
			t.Errorf("%s: %s must contain ## Why section", path, label)
			continue
		}
		if overviewIdx < 0 {
			t.Errorf("%s: %s must contain ## Overview section", path, label)
			continue
		}
		if whyIdx >= overviewIdx {
			t.Errorf("%s: ## Why (at %d) must come before ## Overview (at %d)", path, whyIdx, overviewIdx)
		}
	}
}

func TestReadmeMissingWhySection(t *testing.T) {
	t.Parallel()

	t.Run("missing why", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		mustWrite(t, filepath.Join(dir, "README.md"), "# My Project\n\n## Overview\n\nSome content.\n")
		if !readmeMissingWhySection(dir) {
			t.Error("expected true for README without ## Why")
		}
	})

	t.Run("has why", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		mustWrite(t, filepath.Join(dir, "README.md"), "# My Project\n\n## Why\n\nBecause reasons.\n\n## Overview\n")
		if readmeMissingWhySection(dir) {
			t.Error("expected false for README with ## Why")
		}
	})

	t.Run("no readme", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if readmeMissingWhySection(dir) {
			t.Error("expected false when README.md does not exist")
		}
	})
}

func TestAdoptEmitsWhyAdvisory(t *testing.T) {
	// Not parallel: captures os.Stdout via pipe and uses repoRoot as template source.
	root := repoRoot(t)
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "README.md"), "# Existing\n\n## Overview\n\nNo why section.\n")
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example\n")
	mustWrite(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS.md\n\n## Purpose\n\nExisting.\n")

	cfg := Config{
		Mode:     ModeSync,
		Target:   dir,
		Type:     RepoTypeCode,
		RepoName: "test-adopt",
		Purpose:  "test purpose",
		Stack:    "Go",
		DryRun:   true,
	}

	// Capture stdout from the full adopt path.
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w
	if err := runSync(templates.DiskFS(root), root, cfg); err != nil {
		w.Close()
		os.Stdout = oldStdout
		t.Fatalf("runSync() error = %v", err)
	}
	w.Close()
	os.Stdout = oldStdout

	captured, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading captured output: %v", err)
	}
	output := string(captured)
	if !strings.Contains(output, "advisory:") || !strings.Contains(output, "## Why") {
		t.Errorf("expected adopt advisory about missing ## Why in full adopt path output, got: %q", output)
	}
}

func TestTemplatesDiskFSCanReadBaseAgents(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	tfs := templates.DiskFS(root)
	content, err := fs.ReadFile(tfs, "base/AGENTS.md")
	if err != nil {
		t.Fatalf("templates.DiskFS cannot read base/AGENTS.md: %v", err)
	}
	if !strings.Contains(string(content), "## Purpose") {
		t.Fatal("base/AGENTS.md missing expected ## Purpose section")
	}
}

func TestOldTemplateDirectoriesRemoved(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	for _, dir := range []string{"base", "overlays"} {
		path := filepath.Join(root, dir)
		if _, err := os.Stat(path); err == nil {
			t.Errorf("%s/ still exists at repo root; should have been moved to internal/templates/", dir)
		}
	}
}

func TestTemplateVersionAtRoot(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	content, err := os.ReadFile(filepath.Join(root, "TEMPLATE_VERSION"))
	if err != nil {
		t.Fatalf("TEMPLATE_VERSION must exist at repo root: %v", err)
	}
	if strings.TrimSpace(string(content)) == "" {
		t.Fatal("TEMPLATE_VERSION at repo root must not be empty")
	}
}

func TestEmbeddedFSCanReadBaseAgents(t *testing.T) {
	t.Parallel()
	content, err := fs.ReadFile(templates.EmbeddedFS, "base/AGENTS.md")
	if err != nil {
		t.Fatalf("EmbeddedFS cannot read base/AGENTS.md: %v", err)
	}
	if !strings.Contains(string(content), "## Purpose") {
		t.Fatal("embedded base/AGENTS.md missing ## Purpose section")
	}
}

func TestEmbeddedFSCanWalkOverlays(t *testing.T) {
	t.Parallel()
	var tmplCount int
	err := fs.WalkDir(templates.EmbeddedFS, "overlays/code/files", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".tmpl") {
			tmplCount++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir on EmbeddedFS overlays/code/files: %v", err)
	}
	if tmplCount == 0 {
		t.Fatal("EmbeddedFS overlays/code/files contains no .tmpl files")
	}
}

func TestTemplateVersionConstMatchesFile(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	fileContent, err := os.ReadFile(filepath.Join(root, "TEMPLATE_VERSION"))
	if err != nil {
		t.Fatalf("read TEMPLATE_VERSION: %v", err)
	}
	fileVersion := strings.TrimSpace(string(fileContent))
	if templates.TemplateVersion != fileVersion {
		t.Fatalf("templates.TemplateVersion = %q but TEMPLATE_VERSION file = %q; they must match", templates.TemplateVersion, fileVersion)
	}
}

func TestParseModeArgsNewMode(t *testing.T) {
	t.Parallel()
	cfg, help, err := ParseModeArgs(ModeSync, []string{
		"-y", "CODE", "-n", "test-repo", "-p", "test purpose", "-s", "Go CLI", "-d",
	})
	if err != nil {
		t.Fatalf("ParseModeArgs() error = %v", err)
	}
	if help {
		t.Fatal("expected help = false")
	}
	if cfg.Mode != ModeSync {
		t.Fatalf("Mode = %q, want new", cfg.Mode)
	}
	if cfg.RepoName != "test-repo" {
		t.Fatalf("RepoName = %q, want test-repo", cfg.RepoName)
	}
	if !cfg.DryRun {
		t.Fatal("expected DryRun = true")
	}
}

func TestModulePathIsGitHub(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	content, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	if !strings.Contains(string(content), "module github.com/kquo/governa") {
		t.Fatal("go.mod module path must be github.com/kquo/governa")
	}
}

func TestImportPathsUseGitHub(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	// Check a representative set of files
	for _, rel := range []string{
		"cmd/build/main.go",
		"cmd/governa/main.go",
		"internal/governance/governance.go",
	} {
		content, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if strings.Contains(string(content), `"governa/internal/`) {
			t.Errorf("%s: import path uses old module name \"governa/internal/\" instead of \"github.com/kquo/governa/internal/\"", rel)
		}
	}
}

func TestParseModeArgsEnhanceEmptyReferenceOK(t *testing.T) {
	t.Parallel()
	cfg, help, err := ParseModeArgs(ModeEnhance, []string{})
	if err != nil {
		t.Fatalf("ParseModeArgs(enhance, []) error = %v, want nil", err)
	}
	if help {
		t.Fatal("expected help = false")
	}
	if cfg.Reference != "" {
		t.Fatalf("Reference = %q, want empty", cfg.Reference)
	}
}

func TestParseModeArgsEnhanceWithReference(t *testing.T) {
	t.Parallel()
	cfg, _, err := ParseModeArgs(ModeEnhance, []string{"-r", "/some/path"})
	if err != nil {
		t.Fatalf("ParseModeArgs(enhance, -r) error = %v", err)
	}
	if cfg.Reference != "/some/path" {
		t.Fatalf("Reference = %q, want /some/path", cfg.Reference)
	}
}

func TestSelfReviewIdenticalFSProducesNoDeltas(t *testing.T) {
	t.Parallel()
	deltas, err := RunSelfReview(templates.EmbeddedFS, templates.EmbeddedFS, templates.TemplateVersion)
	if err != nil {
		t.Fatalf("RunSelfReview() error = %v", err)
	}
	if len(deltas) != 0 {
		t.Fatalf("expected 0 deltas for identical FS, got %d: %v", len(deltas), deltas)
	}
}

func TestSelfReviewDetectsChangedFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	err := fs.WalkDir(templates.EmbeddedFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		content, readErr := fs.ReadFile(templates.EmbeddedFS, path)
		if readErr != nil {
			return readErr
		}
		full := filepath.Join(dir, filepath.FromSlash(path))
		if mkErr := os.MkdirAll(filepath.Dir(full), 0o755); mkErr != nil {
			return mkErr
		}
		return os.WriteFile(full, content, 0o644)
	})
	if err != nil {
		t.Fatalf("copy embedded FS: %v", err)
	}
	mustWrite(t, filepath.Join(dir, "base", "AGENTS.md"), "# Modified AGENTS.md\n\n## Purpose\n\nChanged content.\n")

	deltas, err := RunSelfReview(templates.EmbeddedFS, os.DirFS(dir), templates.TemplateVersion)
	if err != nil {
		t.Fatalf("RunSelfReview() error = %v", err)
	}
	found := false
	for _, d := range deltas {
		if d.Path == "base/AGENTS.md" && d.Kind == "changed" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected delta for base/AGENTS.md, got none")
	}
}

func TestSelfReviewDetectsAddedFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	err := fs.WalkDir(templates.EmbeddedFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		content, readErr := fs.ReadFile(templates.EmbeddedFS, path)
		if readErr != nil {
			return readErr
		}
		full := filepath.Join(dir, filepath.FromSlash(path))
		if mkErr := os.MkdirAll(filepath.Dir(full), 0o755); mkErr != nil {
			return mkErr
		}
		return os.WriteFile(full, content, 0o644)
	})
	if err != nil {
		t.Fatalf("copy embedded FS: %v", err)
	}
	mustWrite(t, filepath.Join(dir, "overlays", "code", "files", "new-file.md.tmpl"), "# New file\n")

	deltas, err := RunSelfReview(templates.EmbeddedFS, os.DirFS(dir), templates.TemplateVersion)
	if err != nil {
		t.Fatalf("RunSelfReview() error = %v", err)
	}
	found := false
	for _, d := range deltas {
		if d.Path == "overlays/code/files/new-file.md.tmpl" && d.Kind == "added" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 'added' delta for new-file.md.tmpl, got: %v", deltas)
	}
}

func TestSelfReviewDoesNotCreateACOrProposalFiles(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	docsDir := filepath.Join(root, "docs")
	beforeACs, _ := filepath.Glob(filepath.Join(docsDir, "ac*.md"))
	_, err := RunSelfReview(templates.EmbeddedFS, templates.EmbeddedFS, templates.TemplateVersion)
	if err != nil {
		t.Fatalf("RunSelfReview() error = %v", err)
	}
	afterACs, _ := filepath.Glob(filepath.Join(docsDir, "ac*.md"))
	if len(afterACs) != len(beforeACs) {
		t.Fatalf("self-review created AC files: before=%d after=%d", len(beforeACs), len(afterACs))
	}

	// Check no .template-proposed files were created anywhere under internal/templates/.
	templateDir := filepath.Join(root, "internal", "templates")
	err = filepath.WalkDir(templateDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() && strings.Contains(d.Name(), ".template-proposed") {
			t.Errorf("self-review created proposal file: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk template dir: %v", err)
	}
}

func TestCmdBootstrapDirectoryRemoved(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "cmd", "bootstrap")); err == nil {
		t.Fatal("cmd/bootstrap/ still exists; should have been removed")
	}
}

func TestScriptOnlyCommandsDoesNotContainBootstrap(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	content, err := os.ReadFile(filepath.Join(root, "internal", "buildtool", "buildtool.go"))
	if err != nil {
		t.Fatalf("read buildtool.go: %v", err)
	}
	if strings.Contains(string(content), `"bootstrap"`) {
		t.Fatal("buildtool.go scriptOnlyCommands still contains \"bootstrap\"")
	}
}

func TestEnhanceMappingDoesNotReferenceCmdBootstrap(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	content, err := os.ReadFile(filepath.Join(root, "internal", "governance", "governance.go"))
	if err != nil {
		t.Fatalf("read governance.go: %v", err)
	}
	if strings.Contains(string(content), `"cmd/bootstrap`) {
		t.Fatal("governance.go enhance mappings still reference cmd/bootstrap")
	}
}

func TestACTemplateEnrichedStructure(t *testing.T) {
	t.Parallel()
	templatePaths := []string{
		"docs/ac-template.md",
		"internal/templates/overlays/code/files/docs/ac-template.md.tmpl",
		"examples/code/docs/ac-template.md",
	}
	for _, path := range templatePaths {
		content := readRepoFile(t, path)

		// Summary before Objective Fit (ordering).
		summaryIdx := strings.Index(content, "## Summary")
		objFitIdx := strings.Index(content, "## Objective Fit")
		if summaryIdx < 0 {
			t.Errorf("%s: missing ## Summary", path)
		} else if objFitIdx < 0 {
			t.Errorf("%s: missing ## Objective Fit", path)
		} else if summaryIdx >= objFitIdx {
			t.Errorf("%s: ## Summary (at %d) must appear before ## Objective Fit (at %d)", path, summaryIdx, objFitIdx)
		}

		// Numbered AT format.
		if !strings.Contains(content, "**AT1**") {
			t.Errorf("%s: should contain numbered AT format (**AT1**)", path)
		}

		// Sub-headed In Scope.
		if !strings.Contains(content, "### New files") {
			t.Errorf("%s: should contain ### New files sub-heading", path)
		}

		// Cross-reference to development-cycle.md.
		if !strings.Contains(content, "development-cycle.md") {
			t.Errorf("%s: should cross-reference development-cycle.md", path)
		}

		// Status states.
		if !strings.Contains(content, "PENDING") || !strings.Contains(content, "DEFERRED") {
			t.Errorf("%s: should document PENDING and DEFERRED status states", path)
		}

		// Filename convention preserved.
		if !strings.Contains(content, "ac<N>-<slug>.md") {
			t.Errorf("%s: should contain filename convention ac<N>-<slug>.md", path)
		}
	}
}

func TestGovernanceImprovementsFromSkout(t *testing.T) {
	t.Parallel()
	agentsPaths := []string{
		"internal/templates/base/AGENTS.md",
		"AGENTS.md",
	}

	rules := []struct {
		marker  string
		section string
	}{
		{"Authorization is per-scope", "## Approval Boundaries"},
		{"terse by default", "## Review Style"},
		{"update affected docs in the same pass", "## File-Change Discipline"},
		{"complete the migration in one pass", "## File-Change Discipline"},
		{"Every AC doc must end with", "## Documentation Update Expectations"},
		{"preserve its semantic intent", "## Governed Sections"},
	}

	for _, path := range agentsPaths {
		content := readRepoFile(t, path)
		for _, rule := range rules {
			if !strings.Contains(content, rule.marker) {
				t.Errorf("%s: missing rule %q", path, rule.marker)
				continue
			}
			// Verify the rule appears after its section header.
			sectionIdx := strings.Index(content, rule.section)
			markerIdx := strings.Index(content, rule.marker)
			if sectionIdx < 0 {
				t.Errorf("%s: missing section %q", path, rule.section)
			} else if markerIdx < sectionIdx {
				t.Errorf("%s: rule %q (at %d) should appear inside section %q (at %d)", path, rule.marker, markerIdx, rule.section, sectionIdx)
			}
		}
	}

	// Propagation integrity: both copies must be identical.
	templateContent := readRepoFile(t, "internal/templates/base/AGENTS.md")
	rootContent := readRepoFile(t, "AGENTS.md")
	if templateContent != rootContent {
		t.Fatal("AGENTS.md root and internal/templates/base/AGENTS.md must have identical content")
	}
}

func TestDevRoleDocEnhanceWorkflow(t *testing.T) {
	t.Parallel()
	content := readRepoFile(t, "docs/roles/dev.md")
	if !strings.Contains(content, "governa enhance -r") {
		t.Fatal("docs/roles/dev.md should contain enhance workflow instructions")
	}
	if !strings.Contains(content, "governa enhance") {
		t.Fatal("docs/roles/dev.md should mention self-review mode")
	}
}

func TestCodeOverlayDevRoleGovernaTemplating(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		"internal/templates/overlays/code/files/docs/roles/dev.md.tmpl",
		"examples/code/docs/roles/dev.md",
	} {
		content := readRepoFile(t, path)
		if !strings.Contains(content, "## Governa Templating Maintenance") {
			t.Errorf("%s: should contain ## Governa Templating Maintenance section", path)
		}
		if !strings.Contains(content, "governa sync") {
			t.Errorf("%s: should reference governa sync", path)
		}
		if strings.Contains(content, "### Enhance") {
			t.Errorf("%s: consumer repo should not have an Enhance subsection", path)
		}
		if !strings.Contains(content, "draft an AC before applying") {
			t.Errorf("%s: should nudge AC workflow for sync cherry-picks", path)
		}
	}
	// Self-hosted DEV role should have both sync and enhance under the same section.
	selfHosted := readRepoFile(t, "docs/roles/dev.md")
	if !strings.Contains(selfHosted, "## Governa Templating Maintenance") {
		t.Fatal("docs/roles/dev.md should contain ## Governa Templating Maintenance section")
	}
	if !strings.Contains(selfHosted, "governa enhance") {
		t.Fatal("docs/roles/dev.md should reference governa enhance")
	}
	if !strings.Contains(selfHosted, "governa sync") {
		t.Fatal("docs/roles/dev.md should reference governa sync")
	}
}

func TestGitignoreTemplatesIgnoreGovernaArtifacts(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		"internal/templates/overlays/code/files/.gitignore.tmpl",
		"internal/templates/overlays/doc/files/.gitignore.tmpl",
		"examples/code/.gitignore",
		"examples/doc/.gitignore",
	} {
		content := readRepoFile(t, path)
		if !strings.Contains(content, "governa-sync-review.md") {
			t.Errorf("%s: should ignore governa-sync-review.md", path)
		}
		if !strings.Contains(content, ".governa-proposed/") {
			t.Errorf("%s: should ignore .governa-proposed/", path)
		}
	}
}

func TestEnhanceDriftSummary(t *testing.T) {
	// Not parallel: captures stdout.
	templateRoot := t.TempDir()
	referenceRoot := t.TempDir()

	mustWrite(t, filepath.Join(templateRoot, "base", "AGENTS.md"), `# AGENTS.md

## Purpose

Base purpose.

## Governed Sections

- Purpose

## Interaction Mode

- Default to discussion first.

## Approval Boundaries

- Do not release without approval.

## Review Style

- Findings first.

## File-Change Discipline

- Prefer targeted edits.

## Release Or Publish Triggers

- Release only on request.

## Documentation Update Expectations

- Update docs with behavior.
`)

	mustWrite(t, filepath.Join(referenceRoot, "AGENTS.md"), `# AGENTS.md

## Purpose

Base purpose.

## Governed Sections

- Purpose

## Interaction Mode

- Default to discussion first.
- Do not create artifacts or make changes unless explicitly authorized.

## Approval Boundaries

- Do not release without approval.

## Review Style

- Findings first.

## File-Change Discipline

- Prefer targeted edits.

## Release Or Publish Triggers

- Release only on request.

## Documentation Update Expectations

- Update docs with behavior.
`)

	report, err := ReviewEnhancement(os.DirFS(templateRoot), templateRoot, referenceRoot)
	if err != nil {
		t.Fatalf("ReviewEnhancement() error = %v", err)
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w
	printEnhancementSummary(report)
	w.Close()
	os.Stdout = oldStdout

	captured, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read captured output: %v", err)
	}
	output := string(captured)
	if !strings.Contains(output, "drift:") {
		t.Fatalf("enhance output should contain drift summary, got: %q", output)
	}
}

func TestAdoptDriftSummary(t *testing.T) {
	// Not parallel: captures stdout via full adopt path.
	root := repoRoot(t)
	dir := t.TempDir()
	// Create a target with an existing AGENTS.md missing some sections.
	mustWrite(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS.md\n\n## Purpose\n\nExisting.\n")
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example\n")

	cfg := Config{
		Mode:     ModeSync,
		Target:   dir,
		Type:     RepoTypeCode,
		RepoName: "test-adopt-drift",
		Purpose:  "test",
		Stack:    "Go",
		DryRun:   true,
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w
	if err := runSync(templates.DiskFS(root), root, cfg); err != nil {
		w.Close()
		os.Stdout = oldStdout
		t.Fatalf("runSync() error = %v", err)
	}
	w.Close()
	os.Stdout = oldStdout

	captured, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read captured output: %v", err)
	}
	output := string(captured)
	if !strings.Contains(output, "drift:") {
		t.Fatalf("adopt output should contain drift summary, got: %q", output)
	}
}

func TestAdoptNoDrift(t *testing.T) {
	// Not parallel: captures stdout.
	root := repoRoot(t)
	dir := t.TempDir()
	// Target with manifest (triggers re-sync) but no overlay files to collide with.
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example\n")
	m := Manifest{
		FormatVersion:   manifestFormatVersion,
		TemplateVersion: "0.8.2",
		Params: ManifestParams{
			RepoName: "test-no-drift",
			Purpose:  "test",
			Type:     "CODE",
			Stack:    "Go",
		},
	}
	os.WriteFile(filepath.Join(dir, manifestFileName), []byte(formatManifest(m)), 0o644)

	cfg := Config{
		Mode:     ModeSync,
		Target:   dir,
		Type:     RepoTypeCode,
		RepoName: "test-no-drift",
		Purpose:  "test",
		Stack:    "Go",
		DryRun:   true,
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w
	if err := runSync(templates.DiskFS(root), root, cfg); err != nil {
		w.Close()
		os.Stdout = oldStdout
		t.Fatalf("runSync() error = %v", err)
	}
	w.Close()
	os.Stdout = oldStdout

	captured, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read captured output: %v", err)
	}
	output := string(captured)
	if !strings.Contains(output, "drift: none detected") {
		t.Fatalf("adopt with no existing files should show 'drift: none detected', got: %q", output)
	}
}

// --- inferRepoName tests ---

func TestInferRepoNameBasic(t *testing.T) {
	t.Parallel()
	got := inferRepoName("/Users/someone/code/myproject")
	if got != "myproject" {
		t.Fatalf("inferRepoName() = %q, want myproject", got)
	}
}

func TestInferRepoNameDot(t *testing.T) {
	t.Parallel()
	got := inferRepoName(".")
	// Should resolve to actual directory name, not "."
	if got == "." || got == "" {
		t.Fatalf("inferRepoName(\".\") should resolve to directory name, got %q", got)
	}
}

// --- inferPurpose tests ---

func TestInferPurposeFromReadme(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# My Project\n\nA tool that does useful things.\n\nMore details here.\n"), 0o644)
	got := inferPurpose(dir)
	if got != "A tool that does useful things." {
		t.Fatalf("inferPurpose() = %q, want first paragraph", got)
	}
}

func TestInferPurposeSkipsBadges(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# My Project\n\n![badge](url)\n\nActual description here.\n"), 0o644)
	got := inferPurpose(dir)
	if got != "Actual description here." {
		t.Fatalf("inferPurpose() = %q, want description after badge", got)
	}
}

func TestInferPurposeNoReadme(t *testing.T) {
	t.Parallel()
	got := inferPurpose(t.TempDir())
	if got != "" {
		t.Fatalf("inferPurpose() = %q, want empty for no README", got)
	}
}

func TestInferPurposeHeadingsOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Title\n## Subtitle\n### Section\n"), 0o644)
	got := inferPurpose(dir)
	if got != "" {
		t.Fatalf("inferPurpose() = %q, want empty for headings-only README", got)
	}
}

func TestInferPurposeTruncatesLong(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	long := strings.Repeat("x", 250)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Title\n\n"+long+"\n"), 0o644)
	got := inferPurpose(dir)
	if len(got) != 200 {
		t.Fatalf("inferPurpose() len = %d, want 200", len(got))
	}
}

// --- inferStack tests ---

func TestInferStackGo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com\n"), 0o644)
	got := inferStack(dir)
	if got != "Go" {
		t.Fatalf("inferStack() = %q, want Go", got)
	}
}

func TestInferStackNode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o644)
	got := inferStack(dir)
	if got != "Node" {
		t.Fatalf("inferStack() = %q, want Node", got)
	}
}

func TestInferStackNone(t *testing.T) {
	t.Parallel()
	got := inferStack(t.TempDir())
	if got != "" {
		t.Fatalf("inferStack() = %q, want empty", got)
	}
}

// --- resolveAdoptParams tests ---

func TestResolveAdoptParamsFlagOverridesAll(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Write manifest with stored params
	m := Manifest{
		FormatVersion:   manifestFormatVersion,
		TemplateVersion: "0.7.1",
		Params: ManifestParams{
			RepoName: "old-name",
			Purpose:  "old purpose",
			Stack:    "Python",
		},
	}
	os.WriteFile(filepath.Join(dir, manifestFileName), []byte(formatManifest(m)), 0o644)
	// Also write a go.mod so inference would say "Go"
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com\n"), 0o644)

	cfg := Config{
		Mode:     ModeSync,
		Target:   dir,
		RepoName: "flag-name",
		Purpose:  "flag purpose",
		Stack:    "Rust",
	}
	resolved, sources := resolveAdoptParams(cfg, dir)
	if resolved.RepoName != "flag-name" {
		t.Fatalf("RepoName = %q, want flag-name", resolved.RepoName)
	}
	if resolved.Purpose != "flag purpose" {
		t.Fatalf("Purpose = %q, want flag purpose", resolved.Purpose)
	}
	if resolved.Stack != "Rust" {
		t.Fatalf("Stack = %q, want Rust", resolved.Stack)
	}
	for _, s := range sources {
		if s.source != "flag" {
			t.Fatalf("source for %s = %q, want flag", s.name, s.source)
		}
	}
}

func TestResolveAdoptParamsFromManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	m := Manifest{
		FormatVersion:   manifestFormatVersion,
		TemplateVersion: "0.7.1",
		Params: ManifestParams{
			RepoName: "stored-name",
			Purpose:  "stored purpose",
			Stack:    "Go",
		},
	}
	os.WriteFile(filepath.Join(dir, manifestFileName), []byte(formatManifest(m)), 0o644)

	cfg := Config{Mode: ModeSync, Target: dir}
	resolved, sources := resolveAdoptParams(cfg, dir)
	if resolved.RepoName != "stored-name" {
		t.Fatalf("RepoName = %q, want stored-name", resolved.RepoName)
	}
	if resolved.Purpose != "stored purpose" {
		t.Fatalf("Purpose = %q, want stored purpose", resolved.Purpose)
	}
	for _, s := range sources {
		if s.source != "manifest" {
			t.Fatalf("source for %s = %q, want manifest", s.name, s.source)
		}
	}
}

func TestResolveAdoptParamsInferred(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# TestProject\n\nA test project for testing.\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com\n"), 0o644)

	cfg := Config{Mode: ModeSync, Target: dir}
	resolved, sources := resolveAdoptParams(cfg, dir)
	if resolved.RepoName == "" {
		t.Fatal("RepoName should be inferred from directory basename")
	}
	if resolved.Purpose != "A test project for testing." {
		t.Fatalf("Purpose = %q, want inferred from README", resolved.Purpose)
	}
	if resolved.Stack != "Go" {
		t.Fatalf("Stack = %q, want Go", resolved.Stack)
	}
	for _, s := range sources {
		if s.source != "inferred" {
			t.Fatalf("source for %s = %q, want inferred", s.name, s.source)
		}
	}
}

func TestResolveAdoptParamsManifestTypeRestored(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	m := Manifest{
		FormatVersion:   manifestFormatVersion,
		TemplateVersion: "0.7.1",
		Params: ManifestParams{
			RepoName: "myrepo",
			Purpose:  "test",
			Type:     "CODE",
			Stack:    "Go",
		},
	}
	os.WriteFile(filepath.Join(dir, manifestFileName), []byte(formatManifest(m)), 0o644)

	cfg := Config{Mode: ModeSync, Target: dir}
	resolved, _ := resolveAdoptParams(cfg, dir)
	if resolved.Type != RepoTypeCode {
		t.Fatalf("Type = %q, want CODE (from manifest)", resolved.Type)
	}
}

func TestAdoptManifestContainsParams(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# testproj\n\nTest purpose.\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com\n"), 0o644)

	tfs := templates.EmbeddedFS
	repoRoot := repoRoot(t)
	cfg := Config{
		Mode:     ModeSync,
		Target:   dir,
		RepoName: "testproj",
		Purpose:  "Test purpose.",
		Type:     RepoTypeCode,
		Stack:    "Go",
	}
	if err := RunWithFS(tfs, repoRoot, cfg); err != nil {
		t.Fatalf("RunWithFS() error = %v", err)
	}
	m, ok, err := readManifest(dir)
	if err != nil {
		t.Fatalf("readManifest() error = %v", err)
	}
	if !ok {
		t.Fatal("expected manifest to exist after adopt")
	}
	if m.Params.RepoName != "testproj" {
		t.Fatalf("manifest Params.RepoName = %q, want testproj", m.Params.RepoName)
	}
	if m.Params.Type != "CODE" {
		t.Fatalf("manifest Params.Type = %q, want CODE", m.Params.Type)
	}
	if m.Params.Stack != "Go" {
		t.Fatalf("manifest Params.Stack = %q, want Go", m.Params.Stack)
	}
}

// AT11: inference fails for required param → error names the missing flag
func TestSyncErrorsWhenRequiredParamMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	root := repoRoot(t)
	cfg := Config{
		Mode:   ModeSync,
		Target: dir,
		Input:  strings.NewReader(""), // empty input: prompts get no answers
	}
	err := RunWithFS(templates.DiskFS(root), root, cfg)
	if err == nil {
		t.Fatal("expected error when required params are missing")
	}
}

// AT12: dry-run adopt does not write or update manifest
func TestAdoptDryRunDoesNotWriteManifest(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n\nA test project.\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com\n"), 0o644)

	tfs := templates.EmbeddedFS
	root := repoRoot(t)
	cfg := Config{
		Mode:     ModeSync,
		Target:   dir,
		RepoName: "drytest",
		Purpose:  "A test project.",
		Type:     RepoTypeCode,
		Stack:    "Go",
		DryRun:   true,
	}
	if err := RunWithFS(tfs, root, cfg); err != nil {
		t.Fatalf("RunWithFS() error = %v", err)
	}
	manifestPath := filepath.Join(dir, manifestFileName)
	if _, err := os.Stat(manifestPath); err == nil {
		t.Fatal("manifest should not exist after dry-run adopt")
	}
}

// --- AC29 tests ---

func TestBaseAgentsMdHasPreamble(t *testing.T) {
	t.Parallel()
	content := readRepoFile(t, "internal/templates/base/AGENTS.md")
	if !strings.Contains(content, "only doc guaranteed to be loaded every agent session") {
		t.Fatal("base AGENTS.md should contain session-loading preamble")
	}
}

func TestBaseAgentsMdHasProjectRules(t *testing.T) {
	t.Parallel()
	content := readRepoFile(t, "internal/templates/base/AGENTS.md")
	if !strings.Contains(content, "## Project Rules") {
		t.Fatal("base AGENTS.md should contain ## Project Rules section")
	}
	// Must be flat bullets, no subsections
	idx := strings.Index(content, "## Project Rules")
	rest := content[idx:]
	nextSection := strings.Index(rest[1:], "\n## ")
	if nextSection > 0 {
		rest = rest[:nextSection+1]
	}
	if strings.Contains(rest, "### ") {
		t.Fatal("Project Rules should use flat bullets, not ### subsections")
	}
}

func TestProjectRulesInGovernedSectionList(t *testing.T) {
	t.Parallel()
	content := readRepoFile(t, "internal/templates/base/AGENTS.md")
	if !strings.Contains(content, "`Project Rules`") {
		t.Fatal("Project Rules should appear in the governed section list")
	}
}

func TestCodeOverlayDevMdHasResponseStyle(t *testing.T) {
	t.Parallel()
	content := readRepoFile(t, "internal/templates/overlays/code/files/docs/roles/dev.md.tmpl")
	if !strings.Contains(content, "terse") || !strings.Contains(content, "Review Style") {
		t.Fatal("CODE overlay dev.md should contain response style expectations referencing Review Style")
	}
}

func TestCodeOverlayBuildReleaseMdHasATLabeling(t *testing.T) {
	t.Parallel()
	content := readRepoFile(t, "internal/templates/overlays/code/files/docs/build-release.md.tmpl")
	if !strings.Contains(content, "[Automated]") || !strings.Contains(content, "[Manual]") {
		t.Fatal("CODE overlay build-release.md should contain AT labeling convention")
	}
}

func TestShouldSkipKnowledgeDirNoDir(t *testing.T) {
	t.Parallel()
	if !shouldSkipKnowledgeDir(t.TempDir()) {
		t.Fatal("should skip when docs/knowledge/ does not exist")
	}
}

func TestShouldSkipKnowledgeDirOnlyReadme(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "docs", "knowledge", "README.md"), "# Index\n")
	if !shouldSkipKnowledgeDir(dir) {
		t.Fatal("should skip when docs/knowledge/ has only README.md")
	}
}

func TestShouldSkipKnowledgeDirWithContent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "docs", "knowledge", "README.md"), "# Index\n")
	mustWrite(t, filepath.Join(dir, "docs", "knowledge", "deep-topic.md"), "# Topic\n")
	if shouldSkipKnowledgeDir(dir) {
		t.Fatal("should not skip when docs/knowledge/ has real content")
	}
}

func TestScoreOverlayCollisionIdentical(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := "# Same\n\n## Section\n\ncontent\n"
	existing := filepath.Join(dir, "doc.md")
	mustWrite(t, existing, content)
	score := scoreOverlayCollision(existing, content, "", "")
	if score.recommendation != "keep" {
		t.Fatalf("recommendation = %q, want keep for identical", score.recommendation)
	}
	if !strings.Contains(score.reason, "identical") {
		t.Fatalf("reason = %q, want 'identical to template'", score.reason)
	}
}

func TestScoreOverlayCollisionSameCountDifferentNames(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	existing := filepath.Join(dir, "doc.md")
	mustWrite(t, existing, "# Doc\n\n## Build\ncontent\n## Release\ncontent\n## Tests\ncontent\n")
	proposed := "# Doc\n\n## Build And Test Rules\ncontent\n## Release Trigger\ncontent\n## Release Checklist\ncontent\n"
	score := scoreOverlayCollision(existing, proposed, "", "")
	if score.recommendation != "keep" {
		t.Fatalf("recommendation = %q, want keep (same section count, different names)", score.recommendation)
	}
}

// --- Content-change detection tests (AC33) ---

func TestScoreOverlayContentChangedMarkdown(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	existing := filepath.Join(dir, "doc.md")
	// Existing has same sections but different body — and existing is "more developed" (longer).
	mustWrite(t, existing, "# Doc\n\n## Rules\n\nold rule 1\nold rule 2\nold rule 3\nold rule 4\nold rule 5\n\n## Notes\n\nextra content here\nextra content here\n")
	proposed := "# Doc\n\n## Rules\n\nnew rule 1\n\n## Notes\n\nnote\n"
	score := scoreOverlayCollision(existing, proposed, "oldchecksum", "newchecksum")
	if score.recommendation != "adopt" {
		t.Fatalf("recommendation = %q, want adopt", score.recommendation)
	}
	if !score.contentChanged {
		t.Fatal("contentChanged should be true")
	}
	if len(score.changedSections) == 0 {
		t.Fatal("changedSections should list changed sections")
	}
	found := false
	for _, s := range score.changedSections {
		if s == "Rules" {
			found = true
		}
	}
	if !found {
		t.Fatalf("changedSections = %v, want to include Rules", score.changedSections)
	}
}

func TestScoreOverlayContentChangedNonMarkdown(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	existing := filepath.Join(dir, "build.sh")
	mustWrite(t, existing, "#!/bin/bash\necho hello\n")
	score := scoreOverlayCollision(existing, "#!/bin/bash\necho world\n", "oldchecksum", "newchecksum")
	if score.recommendation != "adopt" {
		t.Fatalf("recommendation = %q, want adopt", score.recommendation)
	}
	if !score.contentChanged {
		t.Fatal("contentChanged should be true")
	}
}

func TestScoreOverlayNoFalsePositiveWhenAlreadyAbsorbed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	existing := filepath.Join(dir, "doc.md")
	content := "# Doc\n\n## Rules\n\nnew rule\n"
	mustWrite(t, existing, content)
	// Template changed (different checksums) but existing already matches proposed.
	score := scoreOverlayCollision(existing, content, "oldchecksum", "newchecksum")
	if score.recommendation != "keep" {
		t.Fatalf("recommendation = %q, want keep (already absorbed)", score.recommendation)
	}
}

func TestScoreOverlayNoContentChangedWhenTemplateUnchanged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	existing := filepath.Join(dir, "doc.md")
	// Existing differs from proposed, but template hasn't changed (same checksums).
	// The structural heuristic should return "keep" when template unchanged.
	// The key assertion is that contentChanged is false.
	mustWrite(t, existing, "# Doc\n\n## Rules\n\nold rule 1\nold rule 2\nold rule 3\nold rule 4\nold rule 5\n\n## Notes\n\nextra\nextra\n")
	score := scoreOverlayCollision(existing, "# Doc\n\n## Rules\n\nnew rule\n\n## Notes\n\nnote\n", "samechecksum", "samechecksum")
	if score.recommendation != "keep" {
		t.Fatalf("recommendation = %q, want keep when template unchanged", score.recommendation)
	}
	if score.contentChanged {
		t.Fatal("contentChanged should be false when template unchanged")
	}
}

func TestScoreGovernanceContentChanged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	agentsPath := filepath.Join(dir, "AGENTS.md")
	// Existing has all governed sections but with different content in one.
	existing := "# AGENTS.md\n\n## Purpose\n\nmy purpose\n\n## Governed Sections\n\nmy sections\n\n## Interaction Mode\n\nold interaction rules\n\n## Approval Boundaries\n\nmy rules\n\n## Review Style\n\nmy style\n\n## File-Change Discipline\n\nmy discipline\n\n## Release Or Publish Triggers\n\nmy triggers\n\n## Documentation Update Expectations\n\nmy docs\n\n## Project Rules\n\nmy project rules\n"
	mustWrite(t, agentsPath, existing)

	// Template has same sections but different Interaction Mode content.
	template := "# AGENTS.md\n\n## Purpose\n\ntemplate purpose\n\n## Governed Sections\n\ntemplate sections\n\n## Interaction Mode\n\nnew interaction rules with extra guidance\n\n## Approval Boundaries\n\ntemplate rules\n\n## Review Style\n\ntemplate style\n\n## File-Change Discipline\n\ntemplate discipline\n\n## Release Or Publish Triggers\n\ntemplate triggers\n\n## Documentation Update Expectations\n\ntemplate docs\n\n## Project Rules\n\ntemplate project rules\n"

	op := operation{kind: "write", path: agentsPath, content: template, note: "base governance contract"}
	score := scoreGovernanceCollision(op, "oldchecksum", "newchecksum")
	if score.recommendation != "adopt" {
		t.Fatalf("recommendation = %q, want adopt", score.recommendation)
	}
	if !score.contentChanged {
		t.Fatal("contentChanged should be true")
	}
	if len(score.changedSections) == 0 {
		t.Fatal("changedSections should list changed governed sections")
	}
}

func TestScoreGovernanceKeepWhenTemplateUnchanged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	agentsPath := filepath.Join(dir, "AGENTS.md")
	existing := "# AGENTS.md\n\n## Purpose\n\nmy purpose\n\n## Governed Sections\n\nmy sections\n\n## Interaction Mode\n\nold rules\n\n## Approval Boundaries\n\nmy rules\n\n## Review Style\n\nmy style\n\n## File-Change Discipline\n\nmy discipline\n\n## Release Or Publish Triggers\n\nmy triggers\n\n## Documentation Update Expectations\n\nmy docs\n\n## Project Rules\n\nmy project rules\n"
	mustWrite(t, agentsPath, existing)
	template := "# AGENTS.md\n\n## Purpose\n\ntemplate\n\n## Governed Sections\n\ntemplate\n\n## Interaction Mode\n\ntemplate\n\n## Approval Boundaries\n\ntemplate\n\n## Review Style\n\ntemplate\n\n## File-Change Discipline\n\ntemplate\n\n## Release Or Publish Triggers\n\ntemplate\n\n## Documentation Update Expectations\n\ntemplate\n\n## Project Rules\n\ntemplate\n"

	op := operation{kind: "write", path: agentsPath, content: template, note: "base governance contract"}
	// Same checksums — template didn't change.
	score := scoreGovernanceCollision(op, "samechecksum", "samechecksum")
	if score.recommendation != "keep" {
		t.Fatalf("recommendation = %q, want keep (template unchanged)", score.recommendation)
	}
}

func TestRenderSyncReviewMethodology(t *testing.T) {
	t.Parallel()
	scores := []collisionScore{
		{path: "/tmp/repo/file.md", recommendation: "keep", reason: "identical", existingLines: 10, proposedLines: 10},
	}
	output := renderSyncReview("/tmp/repo", scores, "", "")
	if !strings.Contains(output, "## Evaluation Methodology") {
		t.Fatal("review doc should contain Evaluation Methodology section")
	}
	for _, phrase := range []string{
		"Structure pass",
		"Content pass",
		"Residual check",
		"Role files pass",
		"Manifest pass",
		"Report",
		"Feedback",
	} {
		if !strings.Contains(output, phrase) {
			t.Fatalf("review doc should contain %q in methodology", phrase)
		}
	}
	// "What sync writes automatically" is merged into intro + methodology
	if strings.Contains(output, "## What sync writes automatically") {
		t.Fatal("review doc should not contain old bookkeeping section")
	}
	if !strings.Contains(output, "bookkeeping") {
		t.Fatal("review doc intro should mention bookkeeping")
	}
	if !strings.Contains(output, "not intended to be committed") {
		t.Fatal("review doc intro should state artifacts are not intended to be committed")
	}
	if !strings.Contains(output, "Default to adopting") {
		t.Fatal("methodology should contain imperative adoption preamble")
	}
	if !strings.Contains(output, "draft an AC") {
		t.Fatal("methodology should nudge agents to use AC workflow")
	}
}

func TestRenderSyncReviewVersionLine(t *testing.T) {
	t.Parallel()
	scores := []collisionScore{
		{path: "/tmp/repo/file.md", recommendation: "keep", reason: "identical", existingLines: 10, proposedLines: 10},
	}
	output := renderSyncReview("/tmp/repo", scores, "0.17.0", "0.18.0")
	if !strings.Contains(output, "Template version: 0.17.0 → 0.18.0") {
		t.Fatalf("review doc should show version transition, got:\n%s", output)
	}
	// No version line when versions are empty
	outputNoVer := renderSyncReview("/tmp/repo", scores, "", "")
	if strings.Contains(outputNoVer, "Template version:") {
		t.Fatal("review doc should not show version line when versions are empty")
	}
}

func TestRenderSyncReviewAdoptItems(t *testing.T) {
	t.Parallel()
	scores := []collisionScore{
		{
			path:                   "/tmp/repo/docs/dev-cycle.md",
			recommendation:         "adopt",
			reason:                 "template sections changed: Cycle (cosmetic)",
			existingLines:          50,
			proposedLines:          20,
			changedSections:        []string{"Cycle"},
			changedClassifications: map[string]string{"Cycle": "cosmetic"},
			contentChanged:         true,
			proposedContent:        "# Dev Cycle\n\n## Cycle\n\nnew cycle content\n",
		},
		{
			path:            "/tmp/repo/build.sh",
			recommendation:  "adopt",
			reason:          "template changed since last sync",
			contentChanged:  true,
			proposedContent: "#!/bin/bash\ngo run ./cmd/build \"$@\"\n",
		},
	}
	output := renderSyncReview("/tmp/repo", scores, "", "")
	if !strings.Contains(output, "## Adoption Items") {
		t.Fatal("output should contain Adoption Items section")
	}
	if !strings.Contains(output, "**adopt**: 2") {
		t.Fatalf("output should show 2 adopt files, got:\n%s", output)
	}
	if !strings.Contains(output, "changed: Cycle (cosmetic)") {
		t.Fatal("output should list changed sections with classification tag")
	}
	if !strings.Contains(output, ".governa-proposed/") {
		t.Fatal("output should reference .governa-proposed/ for comparison")
	}
	// Must not contain old section names
	for _, old := range []string{"## Cherry-Pick Candidates", "## Content Changes", "## Standing Drift", "## Structural Observations"} {
		if strings.Contains(output, old) {
			t.Fatalf("output should not contain old section %q", old)
		}
	}
}

func TestCompareStructureDetectsSubsections(t *testing.T) {
	t.Parallel()
	existing := "## Interaction Mode\n\n### Role Selection\n\n- rule\n- rule\n"
	proposed := "## Interaction Mode\n\n- rule\n- rule\n"
	notes := compareStructure(existing, proposed)
	if len(notes) != 1 {
		t.Fatalf("expected 1 structural note, got %d", len(notes))
	}
	if notes[0].section != "Interaction Mode" {
		t.Fatalf("section = %q, want Interaction Mode", notes[0].section)
	}
}

func TestCompareStructureSameStructureNoNotes(t *testing.T) {
	t.Parallel()
	content := "## Section\n\n- bullet\n- bullet\n"
	notes := compareStructure(content, content)
	if len(notes) != 0 {
		t.Fatalf("expected 0 structural notes for same structure, got %d", len(notes))
	}
}

// --- AC30 tests ---

// AT1: re-sync (manifest present) enters adopt path without prompts
func TestSyncResyncWithManifest(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	dir := t.TempDir()

	// Write a manifest so detectSyncMode returns "re-sync"
	m := Manifest{
		FormatVersion:   manifestFormatVersion,
		TemplateVersion: "0.8.2",
		Params: ManifestParams{
			RepoName: "test-repo",
			Purpose:  "test purpose",
			Type:     "CODE",
			Stack:    "Go",
		},
	}
	os.WriteFile(filepath.Join(dir, manifestFileName), []byte(formatManifest(m)), 0o644)
	// Pre-create AGENTS.md so adopt scoring runs
	mustWrite(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS.md\n\n## Purpose\n\nExisting.\n")

	cfg := Config{
		Mode:   ModeSync,
		Target: dir,
		Input:  strings.NewReader(""), // no prompts needed
	}
	if err := RunWithFS(templates.DiskFS(root), root, cfg); err != nil {
		t.Fatalf("RunWithFS() error = %v", err)
	}
	// Manifest should be updated
	if _, err := os.Stat(filepath.Join(dir, manifestFileName)); err != nil {
		t.Fatal("manifest should exist after re-sync")
	}
}

func TestSyncResyncUpdatesTemplateVersion(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	dir := t.TempDir()

	// Write a manifest with an old template version
	m := Manifest{
		FormatVersion:   manifestFormatVersion,
		TemplateVersion: "0.8.2",
		Params: ManifestParams{
			RepoName: "test-repo",
			Purpose:  "test purpose",
			Type:     "CODE",
			Stack:    "Go",
		},
	}
	os.WriteFile(filepath.Join(dir, manifestFileName), []byte(formatManifest(m)), 0o644)
	// Pre-create AGENTS.md and TEMPLATE_VERSION at old version
	mustWrite(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS.md\n\n## Purpose\n\nExisting.\n")
	mustWrite(t, filepath.Join(dir, "TEMPLATE_VERSION"), "0.8.2")

	cfg := Config{
		Mode:   ModeSync,
		Target: dir,
		Input:  strings.NewReader(""),
	}
	if err := RunWithFS(templates.DiskFS(root), root, cfg); err != nil {
		t.Fatalf("RunWithFS() error = %v", err)
	}
	// TEMPLATE_VERSION must be updated to current, not left at old version
	tvBytes, err := os.ReadFile(filepath.Join(dir, "TEMPLATE_VERSION"))
	if err != nil {
		t.Fatal("TEMPLATE_VERSION should exist after re-sync")
	}
	got := strings.TrimSpace(string(tvBytes))
	if got == "0.8.2" {
		t.Fatal("TEMPLATE_VERSION should be updated on re-sync, still shows old version 0.8.2")
	}
	if got != templates.TemplateVersion {
		t.Fatalf("TEMPLATE_VERSION = %q, want %q", got, templates.TemplateVersion)
	}
}

// AT2: sync with all flags in empty dir (new path) produces no prompts
func TestSyncNewWithAllFlags(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	dir := t.TempDir()

	cfg := Config{
		Mode:     ModeSync,
		Type:     RepoTypeCode,
		Target:   dir,
		RepoName: "test-repo",
		Purpose:  "test purpose",
		Stack:    "Go CLI",
		Input:    strings.NewReader(""), // no prompts needed, all flags given
	}
	if err := RunWithFS(templates.DiskFS(root), root, cfg); err != nil {
		t.Fatalf("RunWithFS() error = %v", err)
	}
	// AGENTS.md should be written
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Fatal("AGENTS.md should exist after sync new")
	}
	// Manifest should exist
	if _, err := os.Stat(filepath.Join(dir, manifestFileName)); err != nil {
		t.Fatal("manifest should exist after sync new")
	}
}

// AT3: sync with no flags prompts interactively
func TestSyncNewPrompts(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	dir := t.TempDir()

	// Simulate interactive input: accept default name (Enter), CODE, purpose, Go
	input := strings.NewReader("\nCODE\nmy test purpose\nGo\n")
	cfg := Config{
		Mode:   ModeSync,
		Target: dir,
		Input:  input,
	}
	if err := RunWithFS(templates.DiskFS(root), root, cfg); err != nil {
		t.Fatalf("RunWithFS() error = %v", err)
	}
	// Should have bootstrapped successfully
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Fatal("AGENTS.md should exist after prompted sync")
	}
}

// AT4: sync in directory with AGENTS.md but no manifest enters adopt path
func TestSyncFirstAdopt(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	dir := t.TempDir()

	mustWrite(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS.md\n\n## Purpose\n\nExisting.\n")
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n\nA test project.\n"), 0o644)

	cfg := Config{
		Mode:     ModeSync,
		Target:   dir,
		Type:     RepoTypeCode,
		RepoName: "test-adopt",
		Purpose:  "A test project.",
		Stack:    "Go",
		Input:    strings.NewReader(""),
	}
	if err := RunWithFS(templates.DiskFS(root), root, cfg); err != nil {
		t.Fatalf("RunWithFS() error = %v", err)
	}
	// Original AGENTS.md preserved (adopt path)
	content, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !strings.Contains(string(content), "Existing.") {
		t.Fatal("AGENTS.md should be preserved in adopt path")
	}
}

// AT5: old subcommands produce error
func TestDetectSyncModeManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, manifestFileName), []byte("governa-manifest-v1\ntemplate-version: 0.8.2\n"), 0o644)
	if got := detectSyncMode(dir); got != "re-sync" {
		t.Fatalf("detectSyncMode() = %q, want re-sync", got)
	}
}

func TestDetectSyncModeGovernanceArtifacts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS\n")
	if got := detectSyncMode(dir); got != "adopt" {
		t.Fatalf("detectSyncMode() = %q, want adopt", got)
	}
}

func TestDetectSyncModeEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if got := detectSyncMode(dir); got != "new" {
		t.Fatalf("detectSyncMode() = %q, want new", got)
	}
}

// AT6: sync help prints flag listing
func TestSyncHelpPrintsFlags(t *testing.T) {
	t.Parallel()
	output := ModeHelp(ModeSync)
	for _, flag := range []string{"-n", "-y", "-p", "-s", "-d"} {
		if !strings.Contains(output, flag) {
			t.Fatalf("sync help should contain %q, got:\n%s", flag, output)
		}
	}
}

// AT7: enhance help prints enhance-specific flags, not sync flags
func TestEnhanceHelpPrintsFlags(t *testing.T) {
	t.Parallel()
	output := ModeHelp(ModeEnhance)
	for _, flag := range []string{"-r", "-d"} {
		if !strings.Contains(output, flag) {
			t.Fatalf("enhance help should contain %q, got:\n%s", flag, output)
		}
	}
	// Should NOT contain sync-only flags
	for _, flag := range []string{"-n,", "-y,", "-p,"} {
		if strings.Contains(output, flag) {
			t.Fatalf("enhance help should not contain sync flag %q", flag)
		}
	}
}

// --- AC31 tests ---

// AT6: findExistingEnhanceAC matches content marker, ignores hand-written ACs
func TestFindExistingEnhanceACContentMarker(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Enhance-generated AC (matches)
	mustWrite(t, filepath.Join(dir, "ac5-enhance-foo.md"), "# AC5 Enhance: base governance\n\nContent.\n")
	// Hand-written AC about enhance behavior (should NOT match)
	mustWrite(t, filepath.Join(dir, "ac31-enhance-ac-collision.md"), "# AC31 Enhance AC collision detection\n\nContent.\n")
	// Unrelated AC (should not match)
	mustWrite(t, filepath.Join(dir, "ac10-sync-refactor.md"), "# AC10 Sync refactor\n\nContent.\n")
	// Another enhance-generated AC (matches)
	mustWrite(t, filepath.Join(dir, "ac12-enhance-bar.md"), "# AC12 Enhance: overlay improvements\n\nContent.\n")

	results := findExistingEnhanceAC(dir)
	if len(results) != 2 {
		t.Fatalf("expected 2 enhance ACs, got %d", len(results))
	}
	if results[0].acNum != 5 {
		t.Errorf("first result acNum = %d, want 5", results[0].acNum)
	}
	if results[1].acNum != 12 {
		t.Errorf("second result acNum = %d, want 12", results[1].acNum)
	}
}

// AT1: no existing enhance AC → new AC written without prompting
func TestEnhanceNoExistingACWritesNew(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	results := findExistingEnhanceAC(dir)
	if len(results) != 0 {
		t.Fatalf("expected 0 results for empty dir, got %d", len(results))
	}
}

// AT2: single existing enhance AC, input "r" → replace (same number, new slug)
func TestEnhanceCollisionReplace(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "ac5-enhance-foo.md"), "# AC5 Enhance: old stuff\n")

	existing := findExistingEnhanceAC(dir)
	sc := bufio.NewScanner(strings.NewReader("r\n"))
	action := promptEnhanceCollision(existing, 6, sc)
	if action.mode != "replace" {
		t.Fatalf("expected mode replace, got %q", action.mode)
	}
	if action.acNum != 5 {
		t.Fatalf("expected acNum 5, got %d", action.acNum)
	}
	if action.oldPath == "" {
		t.Fatal("expected non-empty oldPath for replace")
	}
}

// AT3: single existing enhance AC, input "u" → update (same path)
func TestEnhanceCollisionUpdate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "ac5-enhance-foo.md"), "# AC5 Enhance: old stuff\n")

	existing := findExistingEnhanceAC(dir)
	sc := bufio.NewScanner(strings.NewReader("u\n"))
	action := promptEnhanceCollision(existing, 6, sc)
	if action.mode != "update" {
		t.Fatalf("expected mode update, got %q", action.mode)
	}
	if action.acNum != 5 {
		t.Fatalf("expected acNum 5, got %d", action.acNum)
	}
	if action.oldPath == "" {
		t.Fatal("expected non-empty oldPath for update")
	}
}

// AT4: single existing enhance AC, input "n" → new (next number)
func TestEnhanceCollisionNew(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "ac5-enhance-foo.md"), "# AC5 Enhance: old stuff\n")

	existing := findExistingEnhanceAC(dir)
	sc := bufio.NewScanner(strings.NewReader("n\n"))
	action := promptEnhanceCollision(existing, 6, sc)
	if action.mode != "new" {
		t.Fatalf("expected mode new, got %q", action.mode)
	}
	if action.acNum != 6 {
		t.Fatalf("expected acNum 6, got %d", action.acNum)
	}
}

// AT5: single existing enhance AC, EOF → defaults to new
func TestEnhanceCollisionEOFDefaultsNew(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "ac5-enhance-foo.md"), "# AC5 Enhance: old stuff\n")

	existing := findExistingEnhanceAC(dir)
	sc := bufio.NewScanner(strings.NewReader(""))
	action := promptEnhanceCollision(existing, 6, sc)
	if action.mode != "new" {
		t.Fatalf("expected mode new on EOF, got %q", action.mode)
	}
	if action.acNum != 6 {
		t.Fatalf("expected acNum 6, got %d", action.acNum)
	}
}

// AT7: multiple existing enhance ACs, select by number
func TestEnhanceCollisionMultiMatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "ac5-enhance-foo.md"), "# AC5 Enhance: first\n")
	mustWrite(t, filepath.Join(dir, "ac12-enhance-bar.md"), "# AC12 Enhance: second\n")

	existing := findExistingEnhanceAC(dir)
	if len(existing) != 2 {
		t.Fatalf("expected 2 existing, got %d", len(existing))
	}

	// Select item 2 (ac12), then replace
	sc := bufio.NewScanner(strings.NewReader("2\nr\n"))
	action := promptEnhanceCollision(existing, 13, sc)
	if action.mode != "replace" || action.acNum != 12 {
		t.Fatalf("expected replace acNum=12, got mode=%q acNum=%d", action.mode, action.acNum)
	}

	// Select "n" for new
	sc2 := bufio.NewScanner(strings.NewReader("n\n"))
	action2 := promptEnhanceCollision(existing, 13, sc2)
	if action2.mode != "new" || action2.acNum != 13 {
		t.Fatalf("expected new acNum=13, got mode=%q acNum=%d", action2.mode, action2.acNum)
	}
}

// AT8: dry-run with existing enhance AC — prompt runs, no files written
func TestEnhanceCollisionDryRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "ac5-enhance-foo.md"), "# AC5 Enhance: old stuff\n\nOld content.\n")

	existing := findExistingEnhanceAC(dir)
	sc := bufio.NewScanner(strings.NewReader("r\n"))
	action := promptEnhanceCollision(existing, 6, sc)
	if action.mode != "replace" || action.acNum != 5 {
		t.Fatalf("expected replace acNum=5, got mode=%q acNum=%d", action.mode, action.acNum)
	}

	// Original file should be untouched (dry-run wouldn't write)
	content, err := os.ReadFile(filepath.Join(dir, "ac5-enhance-foo.md"))
	if err != nil {
		t.Fatalf("original file should still exist: %v", err)
	}
	if !strings.Contains(string(content), "Old content.") {
		t.Fatal("original file content should be preserved in dry-run")
	}
}

// AT2b: replace vs update produce different paths in RunEnhance context
func TestEnhanceReplaceVsUpdatePathDifference(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "ac5-enhance-old-slug.md")
	mustWrite(t, oldPath, "# AC5 Enhance: old slug\n")

	existing := findExistingEnhanceAC(dir)

	// Replace: returns old path (to delete) + same acNum → caller generates new slug path
	scR := bufio.NewScanner(strings.NewReader("r\n"))
	actionR := promptEnhanceCollision(existing, 6, scR)
	if actionR.mode != "replace" {
		t.Fatalf("expected replace, got %q", actionR.mode)
	}
	// Replace: new path would differ because slug is derived from the new candidate
	// The old path is returned for deletion
	if actionR.oldPath != oldPath {
		t.Fatalf("replace should return oldPath=%q, got %q", oldPath, actionR.oldPath)
	}

	// Update: returns old path → caller uses it directly (no new slug)
	scU := bufio.NewScanner(strings.NewReader("u\n"))
	actionU := promptEnhanceCollision(existing, 6, scU)
	if actionU.mode != "update" {
		t.Fatalf("expected update, got %q", actionU.mode)
	}
	if actionU.oldPath != oldPath {
		t.Fatalf("update should return oldPath=%q, got %q", oldPath, actionU.oldPath)
	}
}

// --- AC32 tests ---

// AT1: Governed Sections loading contract
func TestBaseAgentsMdLoadingContract(t *testing.T) {
	t.Parallel()
	content := readRepoFile(t, "internal/templates/base/AGENTS.md")
	if !strings.Contains(content, "loaded every agent session") {
		t.Fatal("Governed Sections should contain loading contract")
	}
	if !strings.Contains(content, "loaded on demand") {
		t.Fatal("Governed Sections should reference on-demand docs")
	}
}

// AT2: Interaction Mode defaults to maintainer
func TestBaseAgentsMdDefaultMaintainer(t *testing.T) {
	t.Parallel()
	content := readRepoFile(t, "internal/templates/base/AGENTS.md")
	if !strings.Contains(content, "default to maintainer immediately") {
		t.Fatal("Interaction Mode should default to maintainer when maintainer.md exists")
	}
	if !strings.Contains(content, "announce the active role") {
		t.Fatal("Interaction Mode should require announcing the active role")
	}
}

// AT3: Review Style output format guidance
func TestBaseAgentsMdTerseOutput(t *testing.T) {
	t.Parallel()
	content := readRepoFile(t, "internal/templates/base/AGENTS.md")
	if !strings.Contains(content, "flat bullets") {
		t.Fatal("Review Style should contain terse output guidance")
	}
}

// AT4: Approval Boundaries release command rule
func TestBaseAgentsMdReleaseCommandRule(t *testing.T) {
	t.Parallel()
	content := readRepoFile(t, "internal/templates/base/AGENTS.md")
	if !strings.Contains(content, "Never run the release command yourself") {
		t.Fatal("Approval Boundaries should contain release command rule")
	}
}

// AT5: Approval Boundaries objective-fit rubric inline
func TestBaseAgentsMdRubricInline(t *testing.T) {
	t.Parallel()
	content := readRepoFile(t, "internal/templates/base/AGENTS.md")
	for _, q := range []string{
		"What user or system outcome",
		"Why is this a better next step",
		"What existing decisions or constraints",
		"Is this direct roadmap work or an intentional pivot",
	} {
		if !strings.Contains(content, q) {
			t.Fatalf("Approval Boundaries should contain rubric question: %q", q)
		}
	}
}

// AT6: File-Change Discipline doc-current rule
func TestBaseAgentsMdDocCurrentRule(t *testing.T) {
	t.Parallel()
	content := readRepoFile(t, "internal/templates/base/AGENTS.md")
	if !strings.Contains(content, "keep docs current") {
		t.Fatal("File-Change Discipline should contain doc-current rule")
	}
}

// AT7: Root AGENTS.md matches base template
func TestRootAgentsMdMatchesBase(t *testing.T) {
	t.Parallel()
	base := readRepoFile(t, "internal/templates/base/AGENTS.md")
	root := readRepoFile(t, "AGENTS.md")
	if base != root {
		t.Fatal("root AGENTS.md should match internal/templates/base/AGENTS.md")
	}
}

// AT8: plan.md and templates do not contain Objective-Fit Rubric section
func TestPlanMdNoRubricSection(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		"plan.md",
		"internal/templates/overlays/code/files/plan.md.tmpl",
		"examples/code/plan.md",
	} {
		content := readRepoFile(t, path)
		if strings.Contains(content, "## Objective-Fit Rubric") {
			t.Fatalf("%s should not contain Objective-Fit Rubric section (moved to AGENTS.md)", path)
		}
	}
}

// AT9: Role READMEs contain default-to-maintainer wording
func TestRoleReadmesDefaultMaintainer(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		"docs/roles/README.md",
		"internal/templates/overlays/code/files/docs/roles/README.md.tmpl",
		"internal/templates/overlays/doc/files/docs/roles/README.md.tmpl",
		"examples/code/docs/roles/README.md",
		"examples/doc/docs/roles/README.md",
	} {
		content := readRepoFile(t, path)
		if strings.Contains(content, "asks which role to assume before doing substantive work") {
			t.Fatalf("%s still has old ask-first wording", path)
		}
		if !strings.Contains(content, "defaults to maintainer") {
			t.Fatalf("%s should contain default-to-maintainer wording", path)
		}
	}
}

// AT10: Development cycle docs reference AGENTS.md for rubric
func TestDevCycleRubricReferencesAgentsMd(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		"docs/development-cycle.md",
		"internal/templates/overlays/code/files/docs/development-cycle.md.tmpl",
		"examples/code/docs/development-cycle.md",
	} {
		content := readRepoFile(t, path)
		if strings.Contains(content, "→ Objective-Fit Rubric →") {
			t.Fatalf("%s still references old Objective-Fit Rubric (should reference AGENTS.md)", path)
		}
		if !strings.Contains(content, "AGENTS.md") {
			t.Fatalf("%s should reference AGENTS.md for rubric", path)
		}
	}
}

// --- AC36 tests ---

// AT1: classifyChange returns "structural" when numbered list step is reordered.
func TestClassifyChangeNumberedReorder(t *testing.T) {
	t.Parallel()
	existing := "1. Check tag\n2. Run build\n3. Update changelog\n"
	proposed := "1. Run build\n2. Check tag\n3. Update changelog\n"
	if got := classifyChange(existing, proposed); got != "structural" {
		t.Fatalf("classifyChange = %q, want structural (numbered list reordered)", got)
	}
}

// AT2: classifyChange returns "structural" when a new subsection is added.
func TestClassifyChangeSubsectionAdded(t *testing.T) {
	t.Parallel()
	existing := "Some content here.\n"
	proposed := "Some content here.\n\n### New Subsection\n\nMore detail.\n"
	if got := classifyChange(existing, proposed); got != "structural" {
		t.Fatalf("classifyChange = %q, want structural (subsection added)", got)
	}
}

// AT3: classifyChange returns "cosmetic" when only wording changes.
func TestClassifyChangeCosmeticWording(t *testing.T) {
	t.Parallel()
	existing := "- Use the build command to compile.\n- Run tests after build.\n"
	proposed := "- Use the canonical build command to compile.\n- Execute tests after build.\n"
	if got := classifyChange(existing, proposed); got != "cosmetic" {
		t.Fatalf("classifyChange = %q, want cosmetic (same structure, different wording)", got)
	}
}

// AT4: classifyChange returns "structural" when bullet count changes by >1.
func TestClassifyChangeBulletCountDelta(t *testing.T) {
	t.Parallel()
	existing := "- rule one\n- rule two\n"
	proposed := "- rule one\n- rule two\n- rule three\n- rule four\n"
	if got := classifyChange(existing, proposed); got != "structural" {
		t.Fatalf("classifyChange = %q, want structural (bullet count delta >1)", got)
	}
}

// AT5: renderSyncReview tags sections with (structural) and (cosmetic).
func TestRenderSyncReviewClassificationTags(t *testing.T) {
	t.Parallel()
	scores := []collisionScore{
		{
			path:            "/tmp/repo/docs/guide.md",
			recommendation:  "adopt",
			reason:          "template sections changed: Checklist (structural), Style (cosmetic)",
			existingLines:   40,
			proposedLines:   30,
			changedSections: []string{"Checklist", "Style"},
			changedClassifications: map[string]string{
				"Checklist": "structural",
				"Style":     "cosmetic",
			},
			contentChanged:  true,
			proposedContent: "# Guide\n\n## Checklist\n\n1. Step A\n2. Step B\n\n## Style\n\nKeep it short.\n",
		},
	}
	output := renderSyncReview("/tmp/repo", scores, "", "")
	if !strings.Contains(output, "(structural)") {
		t.Fatalf("output should contain (structural) tag, got:\n%s", output)
	}
	if !strings.Contains(output, "(cosmetic)") {
		t.Fatalf("output should contain (cosmetic) tag, got:\n%s", output)
	}
	if !strings.Contains(output, "changed: Checklist (structural), Style (cosmetic)") {
		t.Fatalf("output should have tagged changed sections in content changes, got:\n%s", output)
	}
}

// AT6: renderSyncReview adopt items reference .governa-proposed/.
func TestRenderSyncReviewAdoptRefersToProposed(t *testing.T) {
	t.Parallel()
	scores := []collisionScore{
		{
			path:            "/tmp/repo/docs/guide.md",
			recommendation:  "adopt",
			reason:          "template sections changed: Style (cosmetic), Checklist (structural)",
			existingLines:   40,
			proposedLines:   30,
			changedSections: []string{"Style", "Checklist"},
			changedClassifications: map[string]string{
				"Checklist": "structural",
				"Style":     "cosmetic",
			},
			contentChanged:  true,
			proposedContent: "# Guide\n\n## Style\n\nKeep it short.\n\n## Checklist\n\n1. Step A\n2. Step B\n",
		},
	}
	output := renderSyncReview("/tmp/repo", scores, "", "")
	if !strings.Contains(output, ".governa-proposed/") {
		t.Fatalf("content changes should reference .governa-proposed/, got:\n%s", output)
	}
	// Should NOT contain inline diff blocks
	if strings.Contains(output, "**Your version:**") {
		t.Fatal("content changes should not have inline diff blocks — use .governa-proposed/ instead")
	}
}

// AT7: scoreGovernanceCollision populates changedClassifications for governed sections.
func TestGovernanceCollisionClassifications(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	agentsPath := filepath.Join(dir, "AGENTS.md")

	existing := "# AGENTS.md\n\n## Project Rules\n\n1. Run build\n2. Check lint\n3. Deploy\n"
	proposed := "# AGENTS.md\n\n## Project Rules\n\n1. Check lint\n2. Run build\n3. Deploy\n"
	os.WriteFile(agentsPath, []byte(existing), 0o644)

	score := scoreGovernanceCollision(
		operation{path: agentsPath, content: proposed},
		"old-checksum",
		"new-checksum",
	)
	if score.recommendation != "adopt" {
		t.Fatalf("recommendation = %q, want adopt", score.recommendation)
	}
	cls, ok := score.changedClassifications["Project Rules"]
	if !ok {
		t.Fatalf("changedClassifications missing 'Project Rules', got: %v", score.changedClassifications)
	}
	if cls != "structural" {
		t.Fatalf("Project Rules classification = %q, want structural (list reordered)", cls)
	}
}

// --- AC38 tests ---

// AT1: scoreOverlayCollision on markdown with 3 ## sections, 1 changed.
func TestOverlaySectionLevelScoring(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "guide.md")

	existing := "# Guide\n\n## Alpha\n\nAlpha content.\n\n## Beta\n\nBeta content.\n\n## Gamma\n\nGamma content.\n"
	proposed := "# Guide\n\n## Alpha\n\nAlpha content.\n\n## Beta\n\nBeta updated content.\n\n## Gamma\n\nGamma content.\n"
	os.WriteFile(filePath, []byte(existing), 0o644)

	score := scoreOverlayCollision(filePath, proposed, "old-checksum", "new-checksum")
	if len(score.changedSections) != 1 {
		t.Fatalf("expected 1 changed section, got %d: %v", len(score.changedSections), score.changedSections)
	}
	if score.changedSections[0] != "Beta" {
		t.Fatalf("changed section = %q, want Beta", score.changedSections[0])
	}
}

// AT2/AT3: renderSyncReview uses lean format — no inline diffs, points to .governa-proposed/.
func TestRenderSyncReviewLeanFormat(t *testing.T) {
	t.Parallel()
	scores := []collisionScore{
		{
			path:            "/tmp/repo/docs/guide.md",
			recommendation:  "adopt",
			reason:          "template sections changed: Style (cosmetic)",
			existingLines:   20,
			proposedLines:   20,
			changedSections: []string{"Style"},
			changedClassifications: map[string]string{
				"Style": "cosmetic",
			},
			contentChanged:  true,
			proposedContent: "# Guide\n\n## Style\n\n- Keep it short.\n",
		},
	}
	output := renderSyncReview("/tmp/repo", scores, "", "")
	if !strings.Contains(output, ".governa-proposed/") {
		t.Fatal("review doc should reference .governa-proposed/ for comparison")
	}
	// Should NOT contain inline diffs or full blocks
	if strings.Contains(output, "```diff") {
		t.Fatal("review doc should not have inline diff blocks — use .governa-proposed/ instead")
	}
	if strings.Contains(output, "**Your version:**") {
		t.Fatal("review doc should not have full blocks — use .governa-proposed/ instead")
	}
}

// AT4: Recommendations table stays file-level.
func TestRenderSyncReviewTableIsFileLevel(t *testing.T) {
	t.Parallel()
	scores := []collisionScore{
		{
			path:            "/tmp/repo/docs/guide.md",
			recommendation:  "adopt",
			reason:          "template sections changed: Alpha (cosmetic), Beta (structural)",
			existingLines:   30,
			proposedLines:   25,
			changedSections: []string{"Alpha", "Beta"},
			changedClassifications: map[string]string{
				"Alpha": "cosmetic",
				"Beta":  "structural",
			},
			contentChanged:  true,
			proposedContent: "# Guide\n\n## Alpha\n\nnew alpha\n\n## Beta\n\nnew beta\n",
		},
	}
	output := renderSyncReview("/tmp/repo", scores, "", "")
	// Count table data rows (lines starting with "| `")
	tableRows := 0
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(line, "| `") {
			tableRows++
		}
	}
	if tableRows != 1 {
		t.Fatalf("recommendations table should have 1 file-level row, got %d", tableRows)
	}
}

// AT5: Enhance subsection drill-down finds portable ### inside deferred ##.
func TestEnhanceSubsectionAcceptInsideDefer(t *testing.T) {
	t.Parallel()
	templateRoot := t.TempDir()
	referenceRoot := filepath.Join(t.TempDir(), "myproject")
	os.MkdirAll(referenceRoot, 0o755)

	mustWrite(t, filepath.Join(templateRoot, "base", "AGENTS.md"), `# AGENTS.md

## Purpose

Base purpose.

## Project Rules

- Follow existing patterns.
`)
	mustWrite(t, filepath.Join(templateRoot, "docs", "ac-template.md"), "# AC template\n")
	// Reference has project-specific Project Rules with a generic subsection.
	// Parent body mentions repo name "myproject" to trigger project-specific marker.
	mustWrite(t, filepath.Join(referenceRoot, "AGENTS.md"), `# AGENTS.md

## Purpose

Base purpose.

## Project Rules

myproject-specific rule about domain data.

### Shell Tool Efficiency

- Use dedicated CLI tools when available.
- Batch independent shell operations into fewer calls.
`)

	report, err := ReviewEnhancement(os.DirFS(templateRoot), templateRoot, referenceRoot)
	if err != nil {
		t.Fatalf("ReviewEnhancement() error = %v", err)
	}

	var foundSubsection bool
	for _, c := range report.Candidates {
		if strings.Contains(c.Section, "Project Rules > Shell Tool Efficiency") {
			foundSubsection = true
			if c.Disposition != "accept" {
				t.Fatalf("subsection disposition = %q, want accept", c.Disposition)
			}
		}
	}
	if !foundSubsection {
		t.Fatal("expected an accept candidate for 'Project Rules > Shell Tool Efficiency'")
	}
}

// AT6: Enhance all-project-specific subsections produce no subsection candidates.
func TestEnhanceSubsectionAllProjectSpecific(t *testing.T) {
	t.Parallel()
	templateRoot := t.TempDir()
	referenceRoot := filepath.Join(t.TempDir(), "myproject")
	os.MkdirAll(referenceRoot, 0o755)

	mustWrite(t, filepath.Join(templateRoot, "base", "AGENTS.md"), `# AGENTS.md

## Purpose

Base purpose.

## Project Rules

- Follow existing patterns.
`)
	mustWrite(t, filepath.Join(templateRoot, "docs", "ac-template.md"), "# AC template\n")
	// All subsections mention the repo name "myproject", so all are project-specific
	mustWrite(t, filepath.Join(referenceRoot, "AGENTS.md"), `# AGENTS.md

## Purpose

Base purpose.

## Project Rules

myproject-specific rule about domain data.

### myproject Integrity

- Consult myproject docs before changing core code.

### myproject Safety

- Any myproject test must use the mock harness.
`)

	report, err := ReviewEnhancement(os.DirFS(templateRoot), templateRoot, referenceRoot)
	if err != nil {
		t.Fatalf("ReviewEnhancement() error = %v", err)
	}

	for _, c := range report.Candidates {
		if strings.Contains(c.Section, " > ") && c.Disposition == "accept" {
			t.Fatalf("should have no accept subsection candidates, got: %s (%s)", c.Section, c.Disposition)
		}
	}
}

// AT7: Markdown file with no ## sections falls back to whole-file scoring.
func TestOverlayNoSectionsFallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "plain.md")

	existing := "Just some text without any sections.\nLine two.\n"
	proposed := "Just some updated text without any sections.\nLine two.\n"
	os.WriteFile(filePath, []byte(existing), 0o644)

	score := scoreOverlayCollision(filePath, proposed, "old-checksum", "new-checksum")
	// With no ## sections, changedSections should be empty — whole-file scoring
	if len(score.changedSections) != 0 {
		t.Fatalf("expected 0 changedSections for sectionless file, got %d", len(score.changedSections))
	}
	// Should still get a recommendation
	if score.recommendation == "" {
		t.Fatal("expected a recommendation for sectionless file")
	}
}

// AT8: Enhance parent defer and child accept coexist.
func TestEnhanceParentDeferChildAcceptCoexist(t *testing.T) {
	t.Parallel()
	templateRoot := t.TempDir()
	referenceRoot := filepath.Join(t.TempDir(), "myproject")
	os.MkdirAll(referenceRoot, 0o755)

	mustWrite(t, filepath.Join(templateRoot, "base", "AGENTS.md"), `# AGENTS.md

## Purpose

Base purpose.

## Project Rules

- Follow existing patterns.
`)
	mustWrite(t, filepath.Join(templateRoot, "docs", "ac-template.md"), "# AC template\n")
	mustWrite(t, filepath.Join(referenceRoot, "AGENTS.md"), `# AGENTS.md

## Purpose

Base purpose.

## Project Rules

myproject-specific rule about domain data.

### Shell Tool Efficiency

- Use dedicated CLI tools when available.
- Batch independent shell operations into fewer calls.
`)

	report, err := ReviewEnhancement(os.DirFS(templateRoot), templateRoot, referenceRoot)
	if err != nil {
		t.Fatalf("ReviewEnhancement() error = %v", err)
	}

	var parentDefer, childAccept bool
	for _, c := range report.Candidates {
		if c.Section == "Project Rules" && c.Disposition == "defer" {
			parentDefer = true
		}
		if strings.Contains(c.Section, "Project Rules > Shell Tool Efficiency") && c.Disposition == "accept" {
			childAccept = true
		}
	}
	if !parentDefer {
		t.Fatal("expected parent 'Project Rules' with disposition defer")
	}
	if !childAccept {
		t.Fatal("expected child 'Project Rules > Shell Tool Efficiency' with disposition accept")
	}
}

// --- AC39 tests ---

// AT1: parseLevel2Sections captures preamble.
func TestParseLevel2SectionsPreamble(t *testing.T) {
	t.Parallel()
	content := "# Title\n\nIntro text here.\n\n## Section One\n\nBody one.\n"
	sections := parseLevel2Sections(content)
	if len(sections) < 2 {
		t.Fatalf("expected at least 2 sections (preamble + real), got %d", len(sections))
	}
	if sections[0].Name != "(preamble)" {
		t.Fatalf("first section name = %q, want (preamble)", sections[0].Name)
	}
	if !strings.Contains(sections[0].Body, "Intro text here.") {
		t.Fatalf("preamble body should contain intro text, got: %q", sections[0].Body)
	}
}

// AT2: parseLevel2Sections with no preamble content.
func TestParseLevel2SectionsNoPreamble(t *testing.T) {
	t.Parallel()
	content := "## Section One\n\nBody one.\n"
	sections := parseLevel2Sections(content)
	for _, s := range sections {
		if s.Name == "(preamble)" {
			t.Fatal("should not have a preamble section when no pre-## content exists")
		}
	}
}

// AT3: detectChangedSections detects preamble change.
func TestDetectChangedSectionsPreamble(t *testing.T) {
	t.Parallel()
	existing := "# Title\n\nOld intro.\n\n## Rules\n\n- rule one\n"
	proposed := "# Title\n\nNew intro.\n\n## Rules\n\n- rule one\n"
	changed := detectChangedSections(existing, proposed)
	found := false
	for _, name := range changed {
		if name == "(preamble)" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected (preamble) in changed sections, got: %v", changed)
	}
}

// AT4: scoreOverlayCollision "keep" with missing sections.
func TestOverlayKeepWithMissingSections(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "guide.md")

	// Existing is 2x larger but missing a template section.
	existing := "# Guide\n\n## Alpha\n\nAlpha content line 1.\nAlpha content line 2.\nAlpha content line 3.\nAlpha content line 4.\nAlpha content line 5.\nAlpha content line 6.\nAlpha content line 7.\nAlpha content line 8.\n\n## Beta\n\nBeta content line 1.\nBeta content line 2.\nBeta content line 3.\nBeta content line 4.\nBeta content line 5.\nBeta content line 6.\nBeta content line 7.\nBeta content line 8.\n"
	proposed := "# Guide\n\n## Alpha\n\nAlpha content.\n\n## Gamma\n\nGamma content.\n"
	os.WriteFile(filePath, []byte(existing), 0o644)

	score := scoreOverlayCollision(filePath, proposed, "old-checksum", "new-checksum")
	if score.recommendation != "keep" && score.recommendation != "adopt" {
		t.Fatalf("recommendation = %q, expected keep or adopt", score.recommendation)
	}
	found := false
	for _, name := range score.missingSections {
		if name == "Gamma" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missingSections should include Gamma, got: %v", score.missingSections)
	}
}

// AT5: renderSyncReview Advisory Notes for keep files with missing sections.
func TestRenderSyncReviewAdvisoryNotes(t *testing.T) {
	t.Parallel()
	scores := []collisionScore{
		{
			path:            "/tmp/repo/docs/guide.md",
			recommendation:  "keep",
			reason:          "existing is more developed",
			existingLines:   100,
			proposedLines:   40,
			missingSections: []string{"Gamma", "Delta"},
		},
	}
	output := renderSyncReview("/tmp/repo", scores, "", "")
	if !strings.Contains(output, "## Advisory Notes") {
		t.Fatalf("expected Advisory Notes section, got:\n%s", output)
	}
	if !strings.Contains(output, "Gamma") || !strings.Contains(output, "Delta") {
		t.Fatalf("advisory note should list missing sections Gamma and Delta, got:\n%s", output)
	}
	// Should NOT appear in the recommendations table
	var tableSection strings.Builder
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(line, "| `") {
			tableSection.WriteString(line + "\n")
		}
	}
	if strings.Contains(tableSection.String(), "Gamma") {
		t.Fatal("missing sections should not appear in the recommendations table")
	}
}

// AT6: detectSectionRenames finds a rename.
func TestDetectSectionRenamesMatch(t *testing.T) {
	t.Parallel()
	existingNames := []string{"Using Sync", "Rules"}
	proposedNames := []string{"Governa Templating Maintenance", "Rules"}
	existingMap := map[string]string{
		"Using Sync": "- Run governa sync periodically.\n- Review governa-sync-review.md for recommendations.\n- The drift summary shows unchanged vs review.\n",
		"Rules":      "- Start every response with DEV says.\n",
	}
	proposedMap := map[string]string{
		"Governa Templating Maintenance": "- Run governa sync periodically.\n- Review governa-sync-review.md for recommendations.\n- The drift summary shows unchanged vs review.\n",
		"Rules":                          "- Start every response with DEV says.\n",
	}
	renames := detectSectionRenames(existingNames, proposedNames, existingMap, proposedMap)
	if renames == nil {
		t.Fatal("expected rename detection, got nil")
	}
	if renames["Using Sync"] != "Governa Templating Maintenance" {
		t.Fatalf("expected Using Sync → Governa Templating Maintenance, got: %v", renames)
	}
}

// AT7: detectSectionRenames returns nil when bodies don't overlap.
func TestDetectSectionRenamesNoMatch(t *testing.T) {
	t.Parallel()
	existingNames := []string{"Old Section"}
	proposedNames := []string{"New Section"}
	existingMap := map[string]string{
		"Old Section": "completely different content here.\nnothing in common.\n",
	}
	proposedMap := map[string]string{
		"New Section": "totally unrelated text.\nno overlap at all.\n",
	}
	renames := detectSectionRenames(existingNames, proposedNames, existingMap, proposedMap)
	if renames != nil {
		t.Fatalf("expected no renames, got: %v", renames)
	}
}

// AT8: renderSyncReview shows rename note.
func TestRenderSyncReviewRenameNote(t *testing.T) {
	t.Parallel()
	scores := []collisionScore{
		{
			path:           "/tmp/repo/docs/roles/dev.md",
			recommendation: "adopt",
			reason:         "template sections changed",
			existingLines:  26,
			proposedLines:  28,
			sectionRenames: map[string]string{"Using Sync": "Governa Templating Maintenance"},
		},
	}
	output := renderSyncReview("/tmp/repo", scores, "", "")
	if !strings.Contains(output, "Section renamed:") {
		t.Fatalf("expected rename note in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Using Sync") || !strings.Contains(output, "Governa Templating Maintenance") {
		t.Fatal("rename note should contain both old and new section names")
	}
}

// AT9: detectSectionRenames tie-breaking — first by document order wins.
func TestDetectSectionRenamesTieBreaking(t *testing.T) {
	t.Parallel()
	existingNames := []string{"Old Section"}
	proposedNames := []string{"New A", "New B"}
	sharedContent := "- shared line one\n- shared line two\n- shared line three\n"
	existingMap := map[string]string{
		"Old Section": sharedContent,
	}
	proposedMap := map[string]string{
		"New A": sharedContent,
		"New B": sharedContent,
	}
	renames := detectSectionRenames(existingNames, proposedNames, existingMap, proposedMap)
	if renames == nil {
		t.Fatal("expected a rename, got nil")
	}
	// First by document order (New A) should win
	if renames["Old Section"] != "New A" {
		t.Fatalf("expected Old Section → New A (first by doc order), got: %v", renames)
	}
	// Old Section consumed — should not also map to New B
	if len(renames) != 1 {
		t.Fatalf("expected exactly 1 rename pair, got %d: %v", len(renames), renames)
	}
}

// Standing drift: template unchanged but file differs from template.
func TestOverlayStandingDrift(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "dev.md")

	existing := "# DEV Role\n\nOld intro line.\n\n## Rules\n\n- rule one\n"
	proposed := "# DEV Role\n\nNew intro line.\n\n## Rules\n\n- rule one\n"
	os.WriteFile(filePath, []byte(existing), 0o644)

	// Same checksum = template unchanged since last sync
	score := scoreOverlayCollision(filePath, proposed, "same-checksum", "same-checksum")
	if score.standingDrift != true {
		t.Fatal("expected standingDrift=true when template unchanged but content differs")
	}
	if len(score.driftSections) == 0 {
		t.Fatal("expected driftSections to list differing sections")
	}
}

func TestOverlayNoStandingDriftWhenIdentical(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "dev.md")

	content := "# DEV Role\n\nIntro.\n\n## Rules\n\n- rule one\n"
	os.WriteFile(filePath, []byte(content), 0o644)

	score := scoreOverlayCollision(filePath, content, "same-checksum", "same-checksum")
	if score.standingDrift {
		t.Fatal("should not have standing drift when content is identical")
	}
}

func TestRenderSyncReviewStandingDrift(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	devPath := filepath.Join(dir, "dev.md")
	existing := "# DEV\n\nOld intro.\n\n## Governa Templating Maintenance\n\n- old bullet\n"
	proposed := "# DEV\n\nNew intro.\n\n## Governa Templating Maintenance\n\n- new bullet\n"
	os.WriteFile(devPath, []byte(existing), 0o644)

	scores := []collisionScore{
		{
			path:            devPath,
			recommendation:  "adopt",
			reason:          "un-adopted template differences in: (preamble), Governa Templating Maintenance",
			existingLines:   7,
			proposedLines:   7,
			standingDrift:   true,
			driftSections:   []string{"(preamble)", "Governa Templating Maintenance"},
			proposedContent: proposed,
		},
	}
	output := renderSyncReview(dir, scores, "", "")
	if !strings.Contains(output, "## Adoption Items") {
		t.Fatalf("expected Adoption Items section, got:\n%s", output)
	}
	if !strings.Contains(output, "adopt") {
		t.Fatalf("expected 'adopt' in recommendations table, got:\n%s", output)
	}
	if !strings.Contains(output, "(preamble)") {
		t.Fatal("drift should list preamble in drifting sections")
	}
	if !strings.Contains(output, "Governa Templating Maintenance") {
		t.Fatal("drift should list drifting section names")
	}
	if !strings.Contains(output, ".governa-proposed/") {
		t.Fatal("drift should reference .governa-proposed/ for comparison")
	}
	// Verify the diff command uses the correct relative path
	if !strings.Contains(output, "diff dev.md .governa-proposed/dev.md") {
		t.Fatalf("diff command should use repo-relative path, got:\n%s", output)
	}
}

func TestOverlayStandingDriftNonMarkdown(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "build.sh")

	existing := "#!/bin/bash\ngo run ./cmd/build \"$@\"\n"
	proposed := "#!/bin/bash\ngo run ./cmd/build \"$@\"\ngo run ./cmd/rel \"$@\"\n"
	os.WriteFile(filePath, []byte(existing), 0o644)

	score := scoreOverlayCollision(filePath, proposed, "same-checksum", "same-checksum")
	if !score.standingDrift {
		t.Fatal("expected standingDrift=true for non-markdown file with unchanged template")
	}
	// After promotion, recommendation should change
	promoteStandingDrift(&score)
	if score.recommendation != "adopt" {
		t.Fatalf("expected 'adopt' after standing drift promotion, got %q", score.recommendation)
	}
}

func TestRenderSyncReviewAdoptNonMarkdown(t *testing.T) {
	t.Parallel()
	scores := []collisionScore{
		{
			path:            "/tmp/repo/build.sh",
			recommendation:  "adopt",
			reason:          "file differs from template baseline (unchanged since last sync)",
			existingLines:   2,
			proposedLines:   3,
			standingDrift:   true,
			proposedContent: "#!/bin/bash\ngo run ./cmd/build \"$@\"\ngo run ./cmd/rel \"$@\"\n",
		},
	}
	output := renderSyncReview("/tmp/repo", scores, "", "")
	if !strings.Contains(output, "## Adoption Items") {
		t.Fatalf("expected Adoption Items section, got:\n%s", output)
	}
	if !strings.Contains(output, ".governa-proposed/") {
		t.Fatal("non-markdown adopt should reference .governa-proposed/ for comparison")
	}
}

func TestWriteProposedFilesNestedPath(t *testing.T) {
	t.Parallel()
	targetDir := t.TempDir()

	// Create a nested file path to simulate docs/roles/dev.md
	nestedPath := filepath.Join(targetDir, "docs", "roles", "dev.md")

	scores := []collisionScore{
		{
			path:            nestedPath,
			recommendation:  "adopt",
			reason:          "un-adopted template differences",
			proposedContent: "# DEV Role\n\nNew template content.\n",
		},
	}
	if err := writeProposedFiles(targetDir, scores, false); err != nil {
		t.Fatalf("writeProposedFiles() error = %v", err)
	}

	// Verify nested path is preserved
	proposedPath := filepath.Join(targetDir, ".governa-proposed", "docs", "roles", "dev.md")
	content, err := os.ReadFile(proposedPath)
	if err != nil {
		t.Fatalf("proposed file should exist at nested path %s: %v", proposedPath, err)
	}
	if !strings.Contains(string(content), "New template content") {
		t.Fatal("proposed file should contain the template content")
	}

	// Verify README wording
	readmeContent, err := os.ReadFile(filepath.Join(targetDir, ".governa-proposed", "README.md"))
	if err != nil {
		t.Fatal("README.md should exist in .governa-proposed/")
	}
	if !strings.Contains(string(readmeContent), "Repo governance decides cleanup") {
		t.Fatal("README should use softened cleanup wording")
	}
	if strings.Contains(string(readmeContent), "Delete it") {
		t.Fatal("README should NOT use 'Delete it' wording")
	}
}
