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

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
