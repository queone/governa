package driftscan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// docs/drift-scan.md "Reachability of canon-only branches" section
// must (a) name host-shape examples, (b) contain exactly one line whose
// stripped content equals ReachabilityHeaderReminder byte-for-byte (the
// constant, not a hardcoded string), (c) carry the known-limit caveat
// about sync-omitted branches, (d) name preptool.go as the canonical
// example, and (e) end with the closing line.
func TestReachabilitySectionInDriftScanDocs(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	docPath := filepath.Join(repoRoot, "docs", "drift-scan.md")
	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read %s: %v", docPath, err)
	}
	s := string(content)

	if !strings.Contains(s, "## Reachability of canon-only branches") {
		t.Error("missing `## Reachability of canon-only branches` H2 section")
	}

	// (b) Exact-line match — exactly one line in the doc, after stripping
	// leading/trailing whitespace, equals ReachabilityHeaderReminder
	// byte-for-byte. Substring matches do not satisfy this check.
	matchCount := 0
	for line := range strings.SplitSeq(s, "\n") {
		if strings.TrimSpace(line) == ReachabilityHeaderReminder {
			matchCount++
		}
	}
	if matchCount != 1 {
		t.Errorf("gate sentence: expected exactly 1 exact-line match for ReachabilityHeaderReminder in drift-scan.md, got %d", matchCount)
	}

	// (a) Host-shape examples named.
	if !strings.Contains(s, "cmd/<repo>/main.go") {
		t.Error("missing host-shape example `cmd/<repo>/main.go`")
	}
	if !strings.Contains(s, "internal/templates/") {
		t.Error("missing host-shape example `internal/templates/` tree")
	}

	// (c) Known-limit caveat about sync-omitted branches.
	if !strings.Contains(s, "sync-omitted branches that look dormant are real drift") {
		t.Error("missing known-limit caveat phrase `sync-omitted branches that look dormant are real drift`")
	}

	// (d) preptool example named.
	if !strings.Contains(s, "internal/preptool/preptool.go") {
		t.Error("missing canonical example `internal/preptool/preptool.go`")
	}

	// (e) Closing line.
	if !strings.Contains(s, "Structurally unreachable branches are not drift.") {
		t.Error("missing closing line `Structurally unreachable branches are not drift.`")
	}
}

// canonCycleSurfacePaths returns absolute paths to the three byte-equal
// canon-cycle.md surfaces: governa root, code-flavor overlay, doc-flavor
// overlay. Editing any one without the other two creates canon-internal
// drift; tests below enforce the byte-equality invariant.
func canonCycleSurfacePaths(t *testing.T) (root, codeOverlay, docOverlay string) {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	root = filepath.Join(repoRoot, "docs", "canon-cycle.md")
	codeOverlay = filepath.Join(repoRoot, "internal", "templates", "overlays", "code", "files", "docs", "canon-cycle.md.tmpl")
	docOverlay = filepath.Join(repoRoot, "internal", "templates", "overlays", "doc", "files", "docs", "canon-cycle.md.tmpl")
	return
}

// All three canon-cycle.md surfaces must exist on disk.
func TestCanonCycleAllThreeSurfacesExist(t *testing.T) {
	root, codeOverlay, docOverlay := canonCycleSurfacePaths(t)
	for _, p := range []string{root, codeOverlay, docOverlay} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing canon-cycle.md surface: %s: %v", p, err)
		}
	}
}

// All three canon-cycle.md surfaces must be byte-equal. Catches drift
// between governa root and the two overlay templates.
func TestCanonCycleAllThreeSurfacesByteEqual(t *testing.T) {
	root, codeOverlay, docOverlay := canonCycleSurfacePaths(t)
	rootBytes, err := os.ReadFile(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	for _, p := range []string{codeOverlay, docOverlay} {
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		if string(got) != string(rootBytes) {
			t.Errorf("byte mismatch: %s differs from governa root canon-cycle.md", p)
		}
	}
}

// docs/canon-cycle.md must contain both audience-explicit headings:
// "## Governa-side commitments" and "## Consumer-side workflow".
func TestCanonCycleAudienceHeadingsPresent(t *testing.T) {
	root, _, _ := canonCycleSurfacePaths(t)
	content, err := os.ReadFile(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	s := string(content)
	for _, h := range []string{"## Governa-side commitments", "## Consumer-side workflow"} {
		if !strings.Contains(s, h) {
			t.Errorf("missing audience-explicit heading: %q", h)
		}
	}
}

// Governa-side section must name the three commitments: semver
// classification, format-defining registry, breaking-change protocol.
func TestCanonCycleGovernaSideCommitmentsNamed(t *testing.T) {
	root, _, _ := canonCycleSurfacePaths(t)
	content, err := os.ReadFile(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	s := string(content)
	checks := []struct {
		alts []string
		name string
	}{
		{[]string{"semver", "Semver"}, "semver classification"},
		{[]string{"Format-defining"}, "format-defining registry"},
		{[]string{"Breaking-change", "breaking-change"}, "breaking-change protocol"},
	}
	for _, c := range checks {
		found := false
		for _, alt := range c.alts {
			if strings.Contains(s, alt) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing %s — none of %v present in canon-cycle.md", c.name, c.alts)
		}
	}
}

// Consumer-side section must name the whole-file rule and the
// mixed-content carve-out: "hand-merging" and "whole-file" (rule
// rationale), "Mixed-content carve-out" (heading), and "orthogonal
// routing signal" (identification — Format-defining is orthogonal
// to mixed-content, not a subset).
func TestCanonCycleConsumerSideRuleAndCarveOutPresent(t *testing.T) {
	root, _, _ := canonCycleSurfacePaths(t)
	content, err := os.ReadFile(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	s := string(content)
	for _, sub := range []string{
		"hand-merging",
		"whole-file",
		"Mixed-content carve-out",
		"orthogonal routing signal",
	} {
		if !strings.Contains(s, sub) {
			t.Errorf("missing consumer-side substring: %q", sub)
		}
	}
}

// Section must name at least one typical pure-canon example and at
// least one typical mixed-content example. cmd/rel/main.go and
// AGENTS.md are universal-subset paths (shipped in both code- and
// doc-flavor overlays).
func TestCanonCycleTypicalExamplesPresent(t *testing.T) {
	root, _, _ := canonCycleSurfacePaths(t)
	content, err := os.ReadFile(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	s := string(content)
	for _, sub := range []string{"cmd/rel/main.go", "AGENTS.md"} {
		if !strings.Contains(s, sub) {
			t.Errorf("missing typical example: %q", sub)
		}
	}
}

// guidelineOverlayPaths returns absolute paths to the two consumer-facing
// guideline overlay templates: development-guidelines.md.tmpl (code-flavor)
// and editing-guidelines.md.tmpl (doc-flavor). Both must follow the
// canon-above-local-below structure with `## Project Practices` as the
// project-extension tail.
func guidelineOverlayPaths(t *testing.T) (codeDev, docEdit string) {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	codeDev = filepath.Join(repoRoot, "internal", "templates", "overlays", "code", "files", "docs", "development-guidelines.md.tmpl")
	docEdit = filepath.Join(repoRoot, "internal", "templates", "overlays", "doc", "files", "docs", "editing-guidelines.md.tmpl")
	return
}

// lastH2Heading returns the text of the last `^## ` heading in s. Returns
// empty string if no H2 heading is present.
func lastH2Heading(s string) string {
	last := ""
	for line := range strings.SplitSeq(s, "\n") {
		if strings.HasPrefix(line, "## ") {
			last = line
		}
	}
	return last
}

// development-guidelines.md.tmpl ends with `## Project Practices` as the
// last top-level heading — the project-extension tail per the
// canon-above-local-below structure.
func TestDevelopmentGuidelinesEndsWithProjectPracticesSection(t *testing.T) {
	codeDev, _ := guidelineOverlayPaths(t)
	content, err := os.ReadFile(codeDev)
	if err != nil {
		t.Fatalf("read %s: %v", codeDev, err)
	}
	got := lastH2Heading(string(content))
	if got != "## Project Practices" {
		t.Errorf("development-guidelines.md.tmpl: last H2 heading = %q, want `## Project Practices`", got)
	}
	if !strings.Contains(string(content), "- Follow existing repo patterns unless an approved improvement says otherwise.") {
		t.Error("development-guidelines.md.tmpl: missing Project Practices placeholder bullet")
	}
}

// editing-guidelines.md.tmpl ends with `## Project Practices` as the
// last top-level heading — same convention as code overlay.
func TestEditingGuidelinesEndsWithProjectPracticesSection(t *testing.T) {
	_, docEdit := guidelineOverlayPaths(t)
	content, err := os.ReadFile(docEdit)
	if err != nil {
		t.Fatalf("read %s: %v", docEdit, err)
	}
	got := lastH2Heading(string(content))
	if got != "## Project Practices" {
		t.Errorf("editing-guidelines.md.tmpl: last H2 heading = %q, want `## Project Practices`", got)
	}
	if !strings.Contains(string(content), "- Follow existing repo patterns unless an approved improvement says otherwise.") {
		t.Error("editing-guidelines.md.tmpl: missing Project Practices placeholder bullet")
	}
}

// Both consumer-facing guideline overlays carry the verbatim
// boundary-explainer preamble sentence (Director-set wording).
func TestGuidelinesPreambleNamesCanonVsLocalBoundary(t *testing.T) {
	codeDev, docEdit := guidelineOverlayPaths(t)
	want := "Sections above ## Project Practices are governa-maintained canon and update via canon syncs; repo-specific practices in ## Project Practices."
	for _, p := range []string{codeDev, docEdit} {
		content, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		if !strings.Contains(string(content), want) {
			t.Errorf("%s: missing boundary-explainer sentence: %q", p, want)
		}
	}
}

// docs/canon-cycle.md carries the doctrine paragraph naming
// canon-above-local-below structure as a reusable pattern. The
// three-surface byte-equality is enforced by
// TestCanonCycleAllThreeSurfacesByteEqual; this test pins the doctrine
// content on the governa-root surface.
func TestCanonCycleProjectTailDoctrinePresent(t *testing.T) {
	root, _, _ := canonCycleSurfacePaths(t)
	content, err := os.ReadFile(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	s := string(content)
	for _, sub := range []string{
		"Canon-above-local-below structure",
		"## Project Rules",
		"## Project Practices",
		"governa-maintained, replaced at sync",
		"repo-maintained, untouched at sync",
	} {
		if !strings.Contains(s, sub) {
			t.Errorf("canon-cycle.md missing doctrine substring: %q", sub)
		}
	}
}
