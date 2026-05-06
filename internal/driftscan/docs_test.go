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
// referencing IE10, (d) name preptool.go as the canonical example, and
// (e) end with the closing line.
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

	// (c) Known-limit caveat with IE10 reference.
	if !strings.Contains(s, "sync-omitted branches that look dormant are real drift") {
		t.Error("missing known-limit caveat phrase `sync-omitted branches that look dormant are real drift`")
	}
	if !strings.Contains(s, "IE10") {
		t.Error("missing IE10 reference in known-limit caveat")
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
