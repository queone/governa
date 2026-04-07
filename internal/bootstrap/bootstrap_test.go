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
	mustWrite(t, filepath.Join(referenceRoot, "AGENTS.md"), `# AGENTS.md

## Purpose

Base purpose.

## Governed Sections

- Purpose

## Interaction Mode

- Treat all input as exploratory discussion.
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

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
