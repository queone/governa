package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProposalPath(t *testing.T) {
	t.Parallel()

	got := proposalPath(filepath.Join("tmp", "README.md"))
	want := filepath.Join("tmp", "README.template-proposed.md")
	if got != want {
		t.Fatalf("proposalPath() = %q, want %q", got, want)
	}
}

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

	report, err := ReviewEnhancement(templateRoot, referenceRoot)
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

	report, err := ReviewEnhancement(templateRoot, referenceRoot)
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

	report, err := ReviewEnhancement(templateRoot, referenceRoot)
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
	mustWrite(t, filepath.Join(dir, "ac-001-first.md"), "# AC\n")
	mustWrite(t, filepath.Join(dir, "ac-003-third.md"), "# AC\n")
	mustWrite(t, filepath.Join(dir, "ac-template.md"), "# Template\n")
	mustWrite(t, filepath.Join(dir, "other.md"), "# Other\n")

	num, err := nextACNumber(dir)
	if err != nil {
		t.Fatalf("nextACNumber() error = %v", err)
	}
	if num != 4 {
		t.Fatalf("nextACNumber() = %d, want 4", num)
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
		"# AC-002 Enhance: base governance — Interaction Mode",
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
	if err := RunEnhance(templateRoot, cfg); err != nil {
		t.Fatalf("RunEnhance() error = %v", err)
	}

	entries, _ := os.ReadDir(filepath.Join(templateRoot, "docs"))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "ac-") && entry.Name() != "ac-template.md" {
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
	if err := RunEnhance(templateRoot, cfg); err != nil {
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
	for _, want := range []string{"# AC-001", "## Summary", "## Status", "PENDING"} {
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
	if err := RunEnhance(templateRoot, cfg); err != nil {
		t.Fatalf("RunEnhance() error = %v", err)
	}

	docsDir := filepath.Join(templateRoot, "docs")
	entries, _ := os.ReadDir(docsDir)
	acCount := 0
	var acFile string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "ac-") && entry.Name() != "ac-template.md" {
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
	if err := RunEnhance(templateRoot, cfg); err != nil {
		t.Fatalf("RunEnhance() error = %v", err)
	}

	docsDir := filepath.Join(templateRoot, "docs")
	entries, _ := os.ReadDir(docsDir)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "ac-") && entry.Name() != "ac-template.md" {
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
		if strings.HasPrefix(entry.Name(), "ac-") && entry.Name() != "ac-template.md" {
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

func TestProposalPathNoExtension(t *testing.T) {
	t.Parallel()
	got := proposalPath(filepath.Join("tmp", "TEMPLATE_VERSION"))
	want := filepath.Join("tmp", "TEMPLATE_VERSION.template-proposed")
	if got != want {
		t.Fatalf("proposalPath() = %q, want %q", got, want)
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

// --- validateConfig tests ---

func TestValidateConfigNewCodeValid(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeNew, Type: RepoTypeCode, RepoName: "r", Purpose: "p", Stack: "Go CLI"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateConfigNewDocValid(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeNew, Type: RepoTypeDoc, RepoName: "r", Purpose: "p", PublishingPlatform: "Hugo", Style: "concise"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateConfigNewMissingName(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeNew, Type: RepoTypeCode, Purpose: "p", Stack: "Go"})
	if err == nil {
		t.Fatal("expected error for missing repo name")
	}
}

func TestValidateConfigNewMissingPurpose(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeNew, Type: RepoTypeCode, RepoName: "r", Stack: "Go"})
	if err == nil {
		t.Fatal("expected error for missing purpose")
	}
}

func TestValidateConfigNewBadType(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeNew, Type: "INVALID", RepoName: "r", Purpose: "p"})
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestValidateConfigNewCodeMissingStack(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeNew, Type: RepoTypeCode, RepoName: "r", Purpose: "p"})
	if err == nil {
		t.Fatal("expected error for missing stack")
	}
}

func TestValidateConfigNewDocMissingPlatform(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeNew, Type: RepoTypeDoc, RepoName: "r", Purpose: "p", Style: "concise"})
	if err == nil {
		t.Fatal("expected error for missing publishing platform")
	}
}

func TestValidateConfigNewDocMissingStyle(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeNew, Type: RepoTypeDoc, RepoName: "r", Purpose: "p", PublishingPlatform: "Hugo"})
	if err == nil {
		t.Fatal("expected error for missing style")
	}
}

func TestValidateConfigAdoptAllowsEmptyType(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeAdopt, RepoName: "r", Purpose: "p"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateConfigAdoptRejectsBadType(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeAdopt, Type: "WRONG", RepoName: "r", Purpose: "p"})
	if err == nil {
		t.Fatal("expected error for invalid adopt type")
	}
}

func TestValidateConfigEnhanceValid(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeEnhance, Reference: "/tmp/ref"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateConfigEnhanceMissingRef(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeEnhance})
	if err == nil {
		t.Fatal("expected error for missing reference")
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

	result, err := readAndRender(path, map[string]string{
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
	_, err := readAndRender("/nonexistent/file.md", nil)
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

// --- proposeIfExists / skipIfExists tests ---

func TestProposeIfExistsFileExists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	existing := filepath.Join(dir, "README.md")
	mustWrite(t, existing, "content")

	op := operation{kind: "write", path: existing, note: "overlay file"}
	result := proposeIfExists(op)
	if result.path != filepath.Join(dir, "README.template-proposed.md") {
		t.Fatalf("got path %q, expected proposal path", result.path)
	}
	if !strings.Contains(result.note, "existing target preserved") {
		t.Fatal("expected note to mention existing target preserved")
	}
}

func TestProposeIfExistsFileDoesNotExist(t *testing.T) {
	t.Parallel()
	op := operation{kind: "write", path: "/nonexistent/README.md", note: "overlay file"}
	result := proposeIfExists(op)
	if result.path != "/nonexistent/README.md" {
		t.Fatalf("path should be unchanged, got %q", result.path)
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
		Mode:     ModeNew,
		Type:     RepoTypeCode,
		RepoName: "test-repo",
		Purpose:  "test purpose",
		Stack:    "Go CLI",
	}

	ops, err := planRender(root, cfg, targetRoot, false)
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

func TestPlanRenderAdoptProposesExistingFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	targetRoot := t.TempDir()

	mustWrite(t, filepath.Join(root, "base", "AGENTS.md"), "# governance\n")
	mustWrite(t, filepath.Join(root, "TEMPLATE_VERSION"), "0.1.0\n")
	mustWrite(t, filepath.Join(root, "overlays", "code", "files", "README.md.tmpl"), "# Template README\n")

	// Pre-existing file in target
	mustWrite(t, filepath.Join(targetRoot, "AGENTS.md"), "# Existing governance\n")

	cfg := Config{
		Mode:     ModeAdopt,
		Type:     RepoTypeCode,
		RepoName: "test-repo",
		Purpose:  "test purpose",
		Stack:    "Go CLI",
	}

	ops, err := planRender(root, cfg, targetRoot, true)
	if err != nil {
		t.Fatalf("planRender() error = %v", err)
	}

	// AGENTS.md should be proposed, not overwritten
	for _, op := range ops {
		if strings.Contains(op.path, "AGENTS") && op.kind == "write" {
			if !strings.Contains(op.path, "template-proposed") {
				t.Fatalf("existing AGENTS.md should get proposal path, got %q", op.path)
			}
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
	mustWrite(t, filepath.Join(root, "overlays", "code", "files", "cmd", "build", "color.go.tmpl"), "package main\n")
	mustWrite(t, filepath.Join(root, "overlays", "code", "files", "cmd", "rel", "main.go.tmpl"), "package main\n")
	mustWrite(t, filepath.Join(root, "overlays", "code", "files", "cmd", "rel", "color.go.tmpl"), "package main\n")

	cfg := Config{
		Mode:     ModeNew,
		Type:     RepoTypeCode,
		RepoName: "test-repo",
		Purpose:  "test purpose",
		Stack:    "Rust service",
	}

	ops, err := planRender(root, cfg, targetRoot, false)
	if err != nil {
		t.Fatalf("planRender() error = %v", err)
	}

	for _, op := range ops {
		if strings.HasSuffix(op.path, "main.go") || strings.HasSuffix(op.path, "color.go") {
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

	report, err := ReviewEnhancement(templateRoot, referenceRoot)
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
	candidate, ok, err := reviewMappedFile(templateRoot, referenceRoot, item, nil)
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
	candidate, ok, err := reviewMappedFile(templateRoot, referenceRoot, item, nil)
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
		Mode:     ModeNew,
		Type:     RepoTypeCode,
		Target:   targetDir,
		RepoName: "test-repo",
		Purpose:  "test purpose",
		Stack:    "Go CLI",
	}
	if err := runNewOrAdopt(templateRoot, cfg, false); err != nil {
		t.Fatalf("runNewOrAdopt() error = %v", err)
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
		Mode:     ModeAdopt,
		Type:     RepoTypeCode,
		Target:   targetDir,
		RepoName: "test-repo",
		Purpose:  "test purpose",
		Stack:    "Go CLI",
	}
	if err := runNewOrAdopt(templateRoot, cfg, true); err != nil {
		t.Fatalf("runNewOrAdopt() error = %v", err)
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

	report, err := ReviewEnhancement(templateRoot, referenceRoot)
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

	report, err := ReviewEnhancement(templateRoot, referenceRoot)
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

	report, err := ReviewEnhancement(templateRoot, referenceRoot)
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

	report, err := ReviewEnhancement(templateRoot, referenceRoot)
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
	report, err := ReviewEnhancement(templateRoot, referenceRoot)
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
	candidate, ok, err := reviewMappedFile(templateRoot, referenceRoot, item, mmap)
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
	markers := projectSpecificMarkers("path is /Users/dev/code", "/tmp/ref")
	if len(markers) != 1 || markers[0] != "contains absolute user path" {
		t.Fatalf("markers = %v, want [contains absolute user path]", markers)
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

// --- AC-006 Phase 4: assisted apply ---

// setupEnhanceFixture creates a template root and reference root for enhance tests.
// Returns (templateRoot, referenceRoot). The reference has a modified Interaction Mode.
func setupEnhanceFixture(t *testing.T) (string, string) {
	t.Helper()
	templateRoot := t.TempDir()
	referenceRoot := filepath.Join(t.TempDir(), "ref")
	if err := os.MkdirAll(referenceRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(templateRoot, "base", "AGENTS.md"), "# AGENTS.md\n\n## Purpose\n\nBase purpose.\n\n## Interaction Mode\n\n- Default to discussion first.\n")
	mustWrite(t, filepath.Join(templateRoot, "docs", "ac-template.md"), "# AC template\n")
	mustWrite(t, filepath.Join(referenceRoot, "AGENTS.md"), "# AGENTS.md\n\n## Purpose\n\nBase purpose.\n\n## Interaction Mode\n\n- Default to discussion first.\n- Added authorization rule.\n")
	return templateRoot, referenceRoot
}

func TestRunEnhanceApplyWritesProposal(t *testing.T) {
	t.Parallel()
	templateRoot, referenceRoot := setupEnhanceFixture(t)

	cfg := Config{Mode: ModeEnhance, Reference: referenceRoot, Apply: true}
	if err := RunEnhance(templateRoot, cfg); err != nil {
		t.Fatalf("RunEnhance() error = %v", err)
	}

	// AC doc should still be created
	acDoc := findACDoc(t, filepath.Join(templateRoot, "docs"))
	if acDoc == "" {
		t.Fatal("expected AC doc to be created even with --apply")
	}

	// Proposal should exist for AGENTS.md
	proposal := proposalPath(filepath.Join(templateRoot, "base", "AGENTS.md"))
	if _, err := os.Stat(proposal); err != nil {
		t.Fatalf("expected proposal file at %s, got error: %v", proposal, err)
	}

	content, _ := os.ReadFile(proposal)
	if !strings.Contains(string(content), "Added authorization rule") {
		t.Fatal("proposal should contain reference content")
	}
}

func TestRunEnhanceApplyStillCreatesACDoc(t *testing.T) {
	t.Parallel()
	templateRoot, referenceRoot := setupEnhanceFixture(t)

	cfg := Config{Mode: ModeEnhance, Reference: referenceRoot, Apply: true}
	if err := RunEnhance(templateRoot, cfg); err != nil {
		t.Fatalf("RunEnhance() error = %v", err)
	}

	acDoc := findACDoc(t, filepath.Join(templateRoot, "docs"))
	if acDoc == "" {
		t.Fatal("--apply should not skip AC doc creation")
	}
}

func TestRunEnhanceApplyNewTargetStillWritesProposal(t *testing.T) {
	t.Parallel()

	templateRoot := t.TempDir()
	referenceRoot := filepath.Join(t.TempDir(), "ref")
	if err := os.MkdirAll(referenceRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	mustWrite(t, filepath.Join(templateRoot, "base", "AGENTS.md"), "# AGENTS.md\n\n## Purpose\n\nBase purpose.\n\n## Interaction Mode\n\n- Default to discussion first.\n")
	mustWrite(t, filepath.Join(templateRoot, "docs", "ac-template.md"), "# AC template\n")
	// Reference has a file that maps to a template target that does NOT exist
	mustWrite(t, filepath.Join(referenceRoot, "AGENTS.md"), "# AGENTS.md\n\n## Purpose\n\nBase purpose.\n\n## Interaction Mode\n\n- Default to discussion first.\n- New constraint.\n")

	cfg := Config{Mode: ModeEnhance, Reference: referenceRoot, Apply: true}
	if err := RunEnhance(templateRoot, cfg); err != nil {
		t.Fatalf("RunEnhance() error = %v", err)
	}

	// Even though base/AGENTS.md exists, proposal should be at the proposal path, not overwriting
	proposal := proposalPath(filepath.Join(templateRoot, "base", "AGENTS.md"))
	if _, err := os.Stat(proposal); err != nil {
		t.Fatalf("expected proposal file, got error: %v", err)
	}
	// Live file should be unchanged
	live, _ := os.ReadFile(filepath.Join(templateRoot, "base", "AGENTS.md"))
	if strings.Contains(string(live), "New constraint") {
		t.Fatal("--apply should not overwrite the live template file")
	}
}

func TestRunEnhanceApplyDryRunWritesNothing(t *testing.T) {
	t.Parallel()
	templateRoot, referenceRoot := setupEnhanceFixture(t)

	cfg := Config{Mode: ModeEnhance, Reference: referenceRoot, Apply: true, DryRun: true}
	if err := RunEnhance(templateRoot, cfg); err != nil {
		t.Fatalf("RunEnhance() error = %v", err)
	}

	// No AC doc
	acDoc := findACDoc(t, filepath.Join(templateRoot, "docs"))
	if acDoc != "" {
		t.Fatalf("dry-run should not create AC doc, found: %s", acDoc)
	}

	// No proposal files
	proposal := proposalPath(filepath.Join(templateRoot, "base", "AGENTS.md"))
	if _, err := os.Stat(proposal); err == nil {
		t.Fatal("dry-run should not create proposal files")
	}
}

func TestValidateConfigApplyRequiresEnhance(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeNew, Type: RepoTypeCode, RepoName: "r", Purpose: "p", Stack: "Go", Apply: true})
	if err == nil {
		t.Fatal("expected error for --apply with non-enhance mode")
	}
	if !strings.Contains(err.Error(), "--apply") {
		t.Fatalf("error should mention --apply, got: %v", err)
	}
}

func TestRunEnhanceWithoutApplyNoProposals(t *testing.T) {
	t.Parallel()
	templateRoot, referenceRoot := setupEnhanceFixture(t)

	cfg := Config{Mode: ModeEnhance, Reference: referenceRoot}
	if err := RunEnhance(templateRoot, cfg); err != nil {
		t.Fatalf("RunEnhance() error = %v", err)
	}

	// Should create AC doc but no proposals
	proposal := proposalPath(filepath.Join(templateRoot, "base", "AGENTS.md"))
	if _, err := os.Stat(proposal); err == nil {
		t.Fatal("enhance without --apply should not create proposal files")
	}
}

func TestRunEnhanceApplyGovernanceProposesWholeFile(t *testing.T) {
	t.Parallel()
	templateRoot, referenceRoot := setupEnhanceFixture(t)

	cfg := Config{Mode: ModeEnhance, Reference: referenceRoot, Apply: true}
	if err := RunEnhance(templateRoot, cfg); err != nil {
		t.Fatalf("RunEnhance() error = %v", err)
	}

	proposal := proposalPath(filepath.Join(templateRoot, "base", "AGENTS.md"))
	content, err := os.ReadFile(proposal)
	if err != nil {
		t.Fatalf("expected governance proposal, got error: %v", err)
	}
	// Should contain the entire reference AGENTS.md, not just the changed section
	if !strings.Contains(string(content), "## Purpose") {
		t.Fatal("governance proposal should contain the full file, not just the changed section")
	}
	if !strings.Contains(string(content), "Added authorization rule") {
		t.Fatal("governance proposal should contain the reference content")
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
		Mode:     ModeAdopt,
		Type:     RepoTypeCode,
		Target:   targetDir,
		RepoName: "test-repo",
		Purpose:  "test purpose",
		Stack:    "Go CLI",
	}
	if err := runNewOrAdopt(templateRoot, cfg, true); err != nil {
		t.Fatalf("runNewOrAdopt() error = %v", err)
	}

	// Original file should be untouched
	original, _ := os.ReadFile(filepath.Join(targetDir, "AGENTS.md"))
	if !strings.Contains(string(original), "Existing purpose.") {
		t.Fatal("original AGENTS.md should be preserved")
	}

	// Proposal should exist with patched content
	proposal := proposalPath(filepath.Join(targetDir, "AGENTS.md"))
	content, err := os.ReadFile(proposal)
	if err != nil {
		t.Fatalf("expected patched proposal at %s, got error: %v", proposal, err)
	}
	if !strings.Contains(string(content), "Existing purpose.") {
		t.Fatal("proposal should preserve existing Purpose")
	}
	if !strings.Contains(string(content), "## Interaction Mode") {
		t.Fatal("proposal should include missing governed sections")
	}
}

func TestAdoptSkipsWhenAllSectionsPresent(t *testing.T) {
	t.Parallel()

	templateRoot, _ := filepath.Abs("../..")
	targetDir := t.TempDir()

	// Read the template AGENTS.md to get all governed sections
	templateAgents, _ := os.ReadFile(filepath.Join(templateRoot, "base", "AGENTS.md"))
	mustWrite(t, filepath.Join(targetDir, "AGENTS.md"), string(templateAgents))

	cfg := Config{
		Mode:     ModeAdopt,
		Type:     RepoTypeCode,
		Target:   targetDir,
		RepoName: "test-repo",
		Purpose:  "test purpose",
		Stack:    "Go CLI",
	}
	if err := runNewOrAdopt(templateRoot, cfg, true); err != nil {
		t.Fatalf("runNewOrAdopt() error = %v", err)
	}

	// No proposal should be created
	proposal := proposalPath(filepath.Join(targetDir, "AGENTS.md"))
	if _, err := os.Stat(proposal); err == nil {
		t.Fatal("should not create proposal when all governed sections present")
	}
}

func TestAdoptNoExistingAgentsWritesDirectly(t *testing.T) {
	t.Parallel()

	templateRoot, _ := filepath.Abs("../..")
	targetDir := t.TempDir()

	cfg := Config{
		Mode:     ModeAdopt,
		Type:     RepoTypeCode,
		Target:   targetDir,
		RepoName: "test-repo",
		Purpose:  "test purpose",
		Stack:    "Go CLI",
	}
	if err := runNewOrAdopt(templateRoot, cfg, true); err != nil {
		t.Fatalf("runNewOrAdopt() error = %v", err)
	}

	// AGENTS.md should be written directly (no proposal)
	content, err := os.ReadFile(filepath.Join(targetDir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("expected AGENTS.md to be written directly, got error: %v", err)
	}
	if !strings.Contains(string(content), "## Interaction Mode") {
		t.Fatal("directly written AGENTS.md should have full template content")
	}

	proposal := proposalPath(filepath.Join(targetDir, "AGENTS.md"))
	if _, err := os.Stat(proposal); err == nil {
		t.Fatal("should not create proposal when no existing AGENTS.md")
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
