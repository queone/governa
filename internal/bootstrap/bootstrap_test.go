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

func TestRenderEnhancementReportIncludesCandidateFields(t *testing.T) {
	t.Parallel()

	report := EnhancementReport{
		ReferenceRoot: "/tmp/reference",
		Candidates: []EnhancementCandidate{{
			Area:            "base governance",
			Path:            "/tmp/reference/AGENTS.md",
			Section:         "Interaction Mode",
			Disposition:     "accept",
			Reason:          "portable delta",
			Portability:     "portable",
			TemplateTarget:  "base/AGENTS.md",
			Summary:         "section differs",
			CollisionImpact: "medium",
		}},
	}

	content := renderEnhancementReport(report)
	for _, want := range []string{
		"# Enhance Report",
		"Reference repo: `<reference-root>`",
		"- `accept`: 1",
		"- Section: `Interaction Mode`",
		"- Template target: `base/AGENTS.md`",
		"- Collision impact: `medium`",
		"- Evidence: section differs",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("report missing %q", want)
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
