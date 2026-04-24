package governance

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestRetiredSymbolsNotPresent is a repo-wide regression guard locking down
// AC78/AC79/AC80 retired-symbol cleanup. Walks the repo tree, greps for any
// symbol that should have been removed, fails on hits. Excludes history paths
// (historical AC files, CHANGELOGs, .git) so AC-document prose referencing
// retired conventions as historical record stays allowed.
//
// AC80 AT13. When a future AC retires more symbols, extend the retiredSymbols
// slice here; consumers syncing get the benefit automatically via the overlay
// regression guard.
func TestRetiredSymbolsNotPresent(t *testing.T) {
	t.Parallel()

	retiredSymbols := []string{
		// AC78 — feedback/ack machinery retired
		"moveFeedbackCompanion",
		"validateFeedbackCredits",
		"feedbackCredit",
		"extractFeedbackCreditsFromContent",
		"AcknowledgedEntry",
		"runEnhance",
		"ReviewEnhancement",
		"EnhancementCandidate",
		"EnhancementReport",
		"SelfReviewDelta",
		"ConfigCritiqueMode",
		// AC79 — collision-prompt machinery retired
		"resolveCollision",
		"parseCollisionReply",
		"collisionChoice",
		// AC80 — checkDrift + friends retired
		"checkDrift",
		"relayDriftSummary",
		"resolveGoverna",
	}

	// Compile one pattern that matches any of the retired symbols as whole
	// words (word boundaries prevent false matches on substrings like
	// `resolveCollisionScore`).
	re := regexp.MustCompile(`\b(` + strings.Join(retiredSymbols, `|`) + `)\b`)

	// Walk from the repo root (two levels up from internal/governance/).
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	// Skip paths that are history / generated or known-irrelevant.
	skipDir := func(abs string) bool {
		rel, err := filepath.Rel(repoRoot, abs)
		if err != nil {
			return false
		}
		rel = filepath.ToSlash(rel)
		switch {
		case rel == ".git":
			return true
		case rel == "examples": // regenerated artifacts; cleared by Part C
			return true
		case strings.HasPrefix(rel, "node_modules"):
			return true
		}
		return false
	}

	skipFile := func(abs string) bool {
		rel, err := filepath.Rel(repoRoot, abs)
		if err != nil {
			return false
		}
		rel = filepath.ToSlash(rel)
		base := filepath.Base(rel)
		switch {
		// Historical AC docs (archival; retain prose references to retired
		// conventions as record). Skip anything under docs/ matching
		// ac<N>-*.md.
		case strings.HasPrefix(rel, "docs/ac") && strings.HasSuffix(rel, ".md"):
			return true
		// CHANGELOGs at both locations are history.
		case base == "CHANGELOG.md":
			return true
		// This test file itself lists retired symbols intentionally.
		case strings.HasSuffix(rel, "retired_symbols_test.go"):
			return true
		}
		return false
	}

	// Only scan text-ish files (Go, Markdown, templates, shell, gitignore).
	isTextExt := func(name string) bool {
		ext := filepath.Ext(name)
		switch ext {
		case ".go", ".md", ".tmpl", ".sh", ".txt", ".yaml", ".yml", ".toml", ".gitignore":
			return true
		}
		// Extension-less files at known text paths.
		switch name {
		case ".gitignore", "TEMPLATE_VERSION", "CHANGELOG.md":
			return true
		}
		return false
	}

	var hits []string
	err = filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if skipDir(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if skipFile(path) {
			return nil
		}
		if !isTextExt(d.Name()) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil // unreadable file — skip
		}
		for _, match := range re.FindAllStringIndex(string(content), -1) {
			// Report one hit per file per symbol (the symbol, not every occurrence).
			symbol := string(content[match[0]:match[1]])
			rel, _ := filepath.Rel(repoRoot, path)
			hits = append(hits, rel+": "+symbol)
			break
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}

	if len(hits) > 0 {
		t.Errorf("retired symbols present in non-history files (%d):\n  %s",
			len(hits), strings.Join(hits, "\n  "))
	}
}
