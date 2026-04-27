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
		// AC88 — collision/review/sync machinery retired
		"ModeSync",
		"collisionRecord",
		"renderSyncReview",
		"unifiedDiffPreview",
		"printReviewSummary",
		"mergeAgentsSections",
		"parseAgentsSections",
		"symlinkConflict",
		"ErrConflictsPresent",
		"syncReviewFile",
		"detectSyncMode",
		// AC91 — collision/recommendation struct fields retired
		"CollidingArtifacts",
		"CollisionRisk",
		// AC89 — manifest/version-check/ownership machinery retired
		"migrateGovernaLegacyPaths",
		"readManifest",
		"buildManifest",
		"formatManifest",
		"parseManifest",
		"ManifestParams",
		"governaOwnedPaths",
		"isGovernaOwnedPath",
		"readTemplateVersion",
		"checkLatestVersion",
		"parseSemver",
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

// TestRetiredProseNotPresent is the prose-drift companion to
// TestRetiredSymbolsNotPresent (AC81 Part E). Walks the same scope and skip
// set, but matches retired-convention phrases in docs, comments, and help
// text — drift the Go-symbol guard cannot catch because the offending text
// is prose, not an identifier.
//
// When a future AC retires a user-visible convention (a flag, a help-text
// claim, a doc phrase that ages out), extend retiredPhrases below; the
// guard then prevents the same retirement-cleanup audit from recurring.
func TestRetiredProseNotPresent(t *testing.T) {
	t.Parallel()

	// Literal substrings, matched case-sensitively. Each phrase is
	// distinctive enough that false positives are unlikely; if one surfaces,
	// add a targeted file exemption below rather than weakening the phrase.
	retiredPhrases := []string{
		// AC79 — interactive collision prompt + --no flag retired.
		"[k]eep / [o]verwrite / [s]kip",
		"--yes and --no",
		// AC78 — enhance/ack subcommands + RunEnhance entry point retired.
		"governa enhance",
		"governa ack",
		"enhance mode",
		"enhance references",
		"enhance semantics",
		// AC78 — preptool feedback-companion move retired.
		"moves -feedback.md companions",
		"moving -feedback.md companions",
		// AC88 — sync subcommand + collision/review prose retired.
		"governa sync",
		"sync-review.md",
		".governa/sync-review",
		// AC89 — manifest/bookkeeping prose retired.
		".governa/manifest",
	}

	// Word-boundary patterns. Use these for phrases generic enough that a
	// raw substring match could false-positive in unrelated prose.
	retiredWordPatterns := regexp.MustCompile(`\b(move feedback)\b`)

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	skipDir := func(abs string) bool {
		rel, err := filepath.Rel(repoRoot, abs)
		if err != nil {
			return false
		}
		rel = filepath.ToSlash(rel)
		switch {
		case rel == ".git":
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
		// Historical AC docs: archival; retain prose for the record.
		case strings.HasPrefix(rel, "docs/ac") && strings.HasSuffix(rel, ".md"):
			return true
		// CHANGELOGs at both locations are history.
		case base == "CHANGELOG.md":
			return true
		// This test file itself lists retired phrases intentionally.
		case strings.HasSuffix(rel, "retired_symbols_test.go"):
			return true
		}
		return false
	}

	isTextExt := func(name string) bool {
		ext := filepath.Ext(name)
		switch ext {
		case ".go", ".md", ".tmpl", ".sh", ".txt", ".yaml", ".yml", ".toml", ".gitignore":
			return true
		}
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
			return nil
		}
		text := string(content)
		rel, _ := filepath.Rel(repoRoot, path)
		for _, phrase := range retiredPhrases {
			if strings.Contains(text, phrase) {
				hits = append(hits, rel+": "+phrase)
				return nil // one hit per file
			}
		}
		if m := retiredWordPatterns.FindString(text); m != "" {
			hits = append(hits, rel+": "+m)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}

	if len(hits) > 0 {
		t.Errorf("retired prose phrases present in non-history files (%d):\n  %s",
			len(hits), strings.Join(hits, "\n  "))
	}
}
