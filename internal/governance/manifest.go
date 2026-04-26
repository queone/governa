package governance

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const manifestFormatVersion = "governa-manifest-v1"

const (
	governaDir       = ".governa"
	manifestFileName = ".governa/manifest"
)

// Legacy path names detected and migrated on apply.
const (
	legacyManifestFileName    = ".repokit-manifest"
	legacyPreAC55ManifestFile = ".governa-manifest"
	legacyPreAC55ProposedDir  = ".governa-proposed"
)

const legacyManifestFormatVersion = "repokit-manifest-v1"

const (
	legacyPreAC78ProposedDir = ".governa/proposed"
	legacyPreAC78FeedbackDir = ".governa/feedback"
	legacyPreAC78ConfigFile  = ".governa/config"
)

type ManifestParams struct {
	RepoName string
	Type     string
	Stack    string
}

type Manifest struct {
	FormatVersion   string
	TemplateVersion string
	Params          ManifestParams
}

// buildManifest constructs the post-apply manifest.
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
	if m.Params.Type != "" {
		fmt.Fprintf(&b, "type: %s\n", m.Params.Type)
	}
	if m.Params.Stack != "" {
		fmt.Fprintf(&b, "stack: %s\n", m.Params.Stack)
	}
	return b.String()
}

// parseManifest reads the manifest, tolerating legacy fields by silently
// ignoring them.
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
			// Legacy field — silently ignored.
		case "type":
			m.Params.Type = value
		case "stack":
			m.Params.Stack = value
		case "publishing-platform", "style":
			// Legacy fields — silently ignored.
		}
	}
	return m, nil
}

func readManifest(root string) (Manifest, bool, error) {
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

// migrateGovernaLegacyPaths cleans up pre-AC55 paths and pre-AC78 artifacts.
// Runs at the top of `governa apply` so consumer repos transition transparently.
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
			fmt.Fprintf(os.Stderr, "governa: removed stale %s (superseded by %s)\n", r.legacy, r.current)
			continue
		}
		if err := os.MkdirAll(governaDirPath, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", governaDir, err)
		}
		if err := os.Rename(legacyAbs, currentAbs); err != nil {
			return fmt.Errorf("migrate %s → %s: %w", r.legacy, r.current, err)
		}
		fmt.Fprintf(os.Stderr, "governa: migrated %s → %s\n", r.legacy, r.current)
	}

	// Pre-AC55 proposed/ directory removal.
	legacyProposedAbs := filepath.Join(root, legacyPreAC55ProposedDir)
	if info, err := os.Stat(legacyProposedAbs); err == nil && info.IsDir() {
		if err := os.RemoveAll(legacyProposedAbs); err != nil {
			return fmt.Errorf("remove legacy %s: %w", legacyPreAC55ProposedDir, err)
		}
		fmt.Fprintf(os.Stderr, "governa: removed legacy %s\n", legacyPreAC55ProposedDir)
	}

	// AC78 migration: drop retired artifacts from .governa/.
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
		fmt.Fprintf(os.Stderr, "governa: removed legacy %s\n", rel)
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
		fmt.Fprintf(os.Stderr, "governa: removed legacy %s\n", rel)
	}

	return nil
}
