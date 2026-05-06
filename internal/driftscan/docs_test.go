package driftscan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AT1 — docs/drift-scan.md "Reachability of canon-only branches" section
// must (a) name host-shape examples, (b) contain exactly one line whose
// stripped content equals ReachabilityHeaderReminder byte-for-byte (the
// constant, not a hardcoded string — Round 9 F-new-13/14/15), (c) carry the
// known-limit caveat referencing IE10, (d) name preptool.go as the
// canonical example, and (e) end with the closing line.
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
	// byte-for-byte. Substring matches do not satisfy this AT.
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
