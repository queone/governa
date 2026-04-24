package governance

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const manifestFormatVersion = "governa-manifest-v1"

// governa-managed metadata lives under a single `.governa/` directory in
// consumer repos (AC55). Primary path:
const (
	governaDir       = ".governa"
	manifestFileName = ".governa/manifest"
)

// Legacy path names detected and migrated on sync. Kept for backward
// compatibility so pre-AC55 repos still detect as re-sync before the migration
// helper runs.
const (
	legacyManifestFileName      = ".repokit-manifest"
	legacyPreAC55ManifestFile   = ".governa-manifest"
	legacyPreAC55ProposedDir    = ".governa-proposed"
	legacyPreAC55SyncReviewFile = "governa-sync-review.md"
)

const legacyManifestFormatVersion = "repokit-manifest-v1"

// AC79 reinstates .governa/sync-review.md as a current artifact, so it is not
// in the AC78 legacy-cleanup list. The three below remain retired.
const (
	legacyPreAC78ProposedDir = ".governa/proposed"
	legacyPreAC78FeedbackDir = ".governa/feedback"
	legacyPreAC78ConfigFile  = ".governa/config"
)

// syncReviewFile is the path (repo-relative) of the DEV/QA/Director review
// artifact produced by runSync. Rewritten on every sync that doesn't use --yes.
const syncReviewFile = ".governa/sync-review.md"

type ManifestParams struct {
	RepoName           string
	Purpose            string
	Type               string
	Stack              string
	PublishingPlatform string
	Style              string
}

type Manifest struct {
	FormatVersion   string
	TemplateVersion string
	Params          ManifestParams
}

// buildManifest constructs the post-sync manifest. AC78 reduces it to template
// bookkeeping: the format-version marker, the template version the consumer
// now tracks, and the params used for rendering. Per-file checksums and the
// acknowledged-drift ledger are retired.
func buildManifest(templateVersion string, params ManifestParams) Manifest {
	return Manifest{
		FormatVersion:   manifestFormatVersion,
		TemplateVersion: templateVersion,
		Params:          params,
	}
}

func formatManifest(m Manifest) string {
	var b strings.Builder
	fmt.Fprintln(&b, m.FormatVersion)
	fmt.Fprintf(&b, "template-version: %s\n", m.TemplateVersion)
	if m.Params.RepoName != "" {
		fmt.Fprintf(&b, "repo-name: %s\n", m.Params.RepoName)
	}
	if m.Params.Purpose != "" {
		fmt.Fprintf(&b, "purpose: %s\n", m.Params.Purpose)
	}
	if m.Params.Type != "" {
		fmt.Fprintf(&b, "type: %s\n", m.Params.Type)
	}
	if m.Params.Stack != "" {
		fmt.Fprintf(&b, "stack: %s\n", m.Params.Stack)
	}
	if m.Params.PublishingPlatform != "" {
		fmt.Fprintf(&b, "publishing-platform: %s\n", m.Params.PublishingPlatform)
	}
	if m.Params.Style != "" {
		fmt.Fprintf(&b, "style: %s\n", m.Params.Style)
	}
	return b.String()
}

// parseManifest reads the minimal AC78 manifest shape plus tolerates legacy
// fields (per-entry sha256, acknowledged blocks) left over from pre-AC78
// repos by silently ignoring them — the file is rewritten on every sync, so
// the stale data disappears on first AC78 sync.
func parseManifest(content string) (Manifest, error) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(lines) == 0 {
		return Manifest{}, fmt.Errorf("unrecognized manifest format: expected %s", manifestFormatVersion)
	}
	formatLine := strings.TrimSpace(lines[0])
	if formatLine != manifestFormatVersion && formatLine != legacyManifestFormatVersion {
		return Manifest{}, fmt.Errorf("unrecognized manifest format: expected %s", manifestFormatVersion)
	}

	m := Manifest{FormatVersion: formatLine}
	// Only the header block (before the first blank line) carries the minimal
	// AC78 fields. Anything past the first blank line is legacy content
	// (per-file sha256 entries, acknowledged blocks) that this sync will
	// overwrite when it rewrites the manifest.
	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)
		if line == "" {
			break
		}
		key, value, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		switch key {
		case "template-version":
			m.TemplateVersion = value
		case "repo-name":
			m.Params.RepoName = value
		case "purpose":
			m.Params.Purpose = value
		case "type":
			m.Params.Type = value
		case "stack":
			m.Params.Stack = value
		case "publishing-platform":
			m.Params.PublishingPlatform = value
		case "style":
			m.Params.Style = value
		}
	}
	return m, nil
}

func readManifest(root string) (Manifest, bool, error) {
	// Try current name first, then legacy fallbacks.
	for _, name := range []string{manifestFileName, legacyPreAC55ManifestFile, legacyManifestFileName} {
		path := filepath.Join(root, name)
		content, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return Manifest{}, false, fmt.Errorf("read manifest %s: %w", path, err)
		}
		m, err := parseManifest(string(content))
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: ignoring malformed manifest at %s: %v\n", path, err)
			return Manifest{}, false, nil
		}
		return m, true, nil
	}
	return Manifest{}, false, nil
}

// migrateGovernaLegacyPaths cleans up pre-AC55 paths (moved under `.governa/`)
// and pre-AC78 artifacts (sync-review.md, proposed/, feedback/, config). Runs
// at the top of `governa sync` so consumer repos transition transparently on
// their next sync. Emits one stderr log line per rename or removal.
func migrateGovernaLegacyPaths(root string) error {
	governaDirPath := filepath.Join(root, governaDir)

	// Pre-AC55 manifest rename into .governa/.
	renames := []struct {
		legacy  string
		current string
	}{
		{legacyPreAC55ManifestFile, manifestFileName},
	}
	for _, r := range renames {
		legacyAbs := filepath.Join(root, r.legacy)
		currentAbs := filepath.Join(root, r.current)
		legacyInfo, err := os.Stat(legacyAbs)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("stat legacy %s: %w", r.legacy, err)
		}
		if legacyInfo.IsDir() {
			continue
		}
		if _, err := os.Stat(currentAbs); err == nil {
			if err := os.Remove(legacyAbs); err != nil {
				return fmt.Errorf("remove stale legacy %s: %w", r.legacy, err)
			}
			fmt.Fprintf(os.Stderr, "governa sync: removed stale %s (superseded by %s)\n", r.legacy, r.current)
			continue
		}
		if err := os.MkdirAll(governaDirPath, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", governaDir, err)
		}
		if err := os.Rename(legacyAbs, currentAbs); err != nil {
			return fmt.Errorf("migrate %s → %s: %w", r.legacy, r.current, err)
		}
		fmt.Fprintf(os.Stderr, "governa sync: migrated %s → %s\n", r.legacy, r.current)
	}

	// Pre-AC55 proposed/ directory removal.
	legacyProposedAbs := filepath.Join(root, legacyPreAC55ProposedDir)
	if info, err := os.Stat(legacyProposedAbs); err == nil && info.IsDir() {
		if err := os.RemoveAll(legacyProposedAbs); err != nil {
			return fmt.Errorf("remove legacy %s: %w", legacyPreAC55ProposedDir, err)
		}
		fmt.Fprintf(os.Stderr, "governa sync: removed legacy %s\n", legacyPreAC55ProposedDir)
	}
	// Pre-AC55 sync-review.md removal (superseded by .governa/sync-review.md
	// which itself is removed below as part of the AC78 migration).
	legacyReviewAbs := filepath.Join(root, legacyPreAC55SyncReviewFile)
	if info, err := os.Stat(legacyReviewAbs); err == nil && !info.IsDir() {
		if err := os.Remove(legacyReviewAbs); err != nil {
			return fmt.Errorf("remove legacy %s: %w", legacyPreAC55SyncReviewFile, err)
		}
		fmt.Fprintf(os.Stderr, "governa sync: removed legacy %s\n", legacyPreAC55SyncReviewFile)
	}

	// AC78 migration: drop the rich-sync artifacts from the `.governa/` dir.
	// These lived at .governa/sync-review.md, .governa/proposed/, .governa/feedback/,
	// .governa/config under pre-AC78 governa and are all retired.
	for _, rel := range []string{legacyPreAC78ProposedDir, legacyPreAC78FeedbackDir} {
		abs := filepath.Join(root, rel)
		info, err := os.Stat(abs)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("stat legacy %s: %w", rel, err)
		}
		if !info.IsDir() {
			continue
		}
		if err := os.RemoveAll(abs); err != nil {
			return fmt.Errorf("remove legacy %s: %w", rel, err)
		}
		fmt.Fprintf(os.Stderr, "governa sync: removed legacy %s (AC78 migration)\n", rel)
	}
	for _, rel := range []string{legacyPreAC78ConfigFile} {
		abs := filepath.Join(root, rel)
		info, err := os.Stat(abs)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("stat legacy %s: %w", rel, err)
		}
		if info.IsDir() {
			continue
		}
		if err := os.Remove(abs); err != nil {
			return fmt.Errorf("remove legacy %s: %w", rel, err)
		}
		fmt.Fprintf(os.Stderr, "governa sync: removed legacy %s (AC78 migration)\n", rel)
	}

	return nil
}
