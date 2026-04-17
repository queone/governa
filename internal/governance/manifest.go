package governance

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const manifestFormatVersion = "governa-manifest-v1"

// governa-managed metadata lives under a single `.governa/` directory in
// consumer repos (AC55). Primary paths:
const (
	governaDir       = ".governa"
	manifestFileName = ".governa/manifest"
	proposedDirName  = ".governa/proposed"
	syncReviewFile   = ".governa/sync-review.md"
	feedbackDirName  = ".governa/feedback"
)

// Legacy path names detected and migrated on sync. Kept as constants so the
// migration helper and backward-compatible readers reference one source.
const (
	legacyManifestFileName      = ".repokit-manifest"
	legacyPreAC55ManifestFile   = ".governa-manifest"
	legacyPreAC55ProposedDir    = ".governa-proposed"
	legacyPreAC55SyncReviewFile = "governa-sync-review.md"
)

const legacyManifestFormatVersion = "repokit-manifest-v1"

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
	Entries         []ManifestEntry
	Acknowledged    []AcknowledgedEntry
}

type ManifestEntry struct {
	Path           string
	Checksum       string
	SourcePath     string
	SourceChecksum string
	Kind           string // "file" or "symlink"
	SymlinkTarget  string
}

type AcknowledgedEntry struct {
	Path            string
	ConsumerSHA     string
	TemplateSHA     string
	TemplateVersion string
	Reason          string
}

func computeChecksum(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

func buildManifest(ops []operation, templateVersion string, tfs fs.FS, repoRoot string, targetRoot string) Manifest {
	m := Manifest{
		FormatVersion:   manifestFormatVersion,
		TemplateVersion: templateVersion,
	}

	for _, op := range ops {
		var repoRel string
		if rel, err := filepath.Rel(targetRoot, op.path); err == nil {
			repoRel = filepath.ToSlash(rel)
		} else {
			repoRel = filepath.ToSlash(op.path)
		}

		switch op.kind {
		case "write":
			entry := ManifestEntry{
				Path:       repoRel,
				Checksum:   computeChecksum(op.content),
				SourcePath: filepath.ToSlash(op.source),
				Kind:       "file",
			}
			if op.source != "" {
				sourceContent, err := readTemplateOrRoot(tfs, repoRoot, op.source)
				if err == nil {
					entry.SourceChecksum = computeChecksum(string(sourceContent))
				}
			}
			m.Entries = append(m.Entries, entry)
		case "symlink":
			entry := ManifestEntry{
				Path:          repoRel,
				Kind:          "symlink",
				SymlinkTarget: op.linkTo,
				SourcePath:    filepath.ToSlash(op.source),
			}
			m.Entries = append(m.Entries, entry)
		}
	}

	sort.Slice(m.Entries, func(i, j int) bool {
		return m.Entries[i].Path < m.Entries[j].Path
	})
	return m
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
	b.WriteString("\n")

	for _, e := range m.Entries {
		if e.Kind == "symlink" {
			fmt.Fprintf(&b, "%s symlink:%s", e.Path, e.SymlinkTarget)
			if e.SourcePath != "" {
				fmt.Fprintf(&b, " source:%s", e.SourcePath)
			}
			b.WriteString("\n")
			continue
		}
		fmt.Fprintf(&b, "%s sha256:%s", e.Path, e.Checksum)
		if e.SourcePath != "" {
			fmt.Fprintf(&b, " source:%s", e.SourcePath)
		}
		if e.SourceChecksum != "" {
			fmt.Fprintf(&b, " source-sha256:%s", e.SourceChecksum)
		}
		b.WriteString("\n")
	}
	if len(m.Acknowledged) > 0 {
		b.WriteString("\nacknowledged:\n")
		for _, a := range m.Acknowledged {
			fmt.Fprintf(&b, "  - path: %s\n", a.Path)
			fmt.Fprintf(&b, "    consumer-sha: %s\n", a.ConsumerSHA)
			fmt.Fprintf(&b, "    template-sha: %s\n", a.TemplateSHA)
			fmt.Fprintf(&b, "    template-version: %s\n", a.TemplateVersion)
			fmt.Fprintf(&b, "    reason: %s\n", a.Reason)
		}
	}
	return b.String()
}

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
	i := 1
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		i++
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

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		i++
		if line == "" {
			continue
		}
		if line == "acknowledged:" {
			for i < len(lines) {
				raw := lines[i]
				line = strings.TrimSpace(raw)
				if line == "" {
					i++
					continue
				}
				if !strings.HasPrefix(raw, "  - ") && !strings.HasPrefix(raw, "\t- ") {
					break
				}
				entry := AcknowledgedEntry{}
				fields := []string{strings.TrimSpace(strings.TrimPrefix(line, "- "))}
				i++
				for i < len(lines) {
					nextRaw := lines[i]
					next := strings.TrimSpace(nextRaw)
					if next == "" {
						i++
						continue
					}
					if strings.HasPrefix(nextRaw, "  - ") || strings.HasPrefix(nextRaw, "\t- ") {
						break
					}
					if strings.HasPrefix(nextRaw, "    ") || strings.HasPrefix(nextRaw, "\t\t") {
						fields = append(fields, next)
						i++
						continue
					}
					break
				}
				for _, field := range fields {
					key, value, ok := strings.Cut(field, ": ")
					if !ok {
						return Manifest{}, fmt.Errorf("malformed acknowledged field %q", field)
					}
					switch key {
					case "path":
						entry.Path = value
					case "consumer-sha":
						entry.ConsumerSHA = value
					case "template-sha":
						entry.TemplateSHA = value
					case "template-version":
						entry.TemplateVersion = value
					case "reason":
						entry.Reason = value
					default:
						return Manifest{}, fmt.Errorf("unknown acknowledged field %q", key)
					}
				}
				if entry.Path == "" || entry.ConsumerSHA == "" || entry.TemplateSHA == "" || entry.TemplateVersion == "" || entry.Reason == "" {
					return Manifest{}, fmt.Errorf("acknowledged entry %q missing required fields", entry.Path)
				}
				m.Acknowledged = append(m.Acknowledged, entry)
			}
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			return Manifest{}, fmt.Errorf("malformed manifest entry: %q", line)
		}

		entry := ManifestEntry{Path: parts[0]}
		for _, part := range parts[1:] {
			key, value, ok := strings.Cut(part, ":")
			if !ok {
				return Manifest{}, fmt.Errorf("malformed manifest field %q in entry %q", part, entry.Path)
			}
			switch key {
			case "sha256":
				entry.Kind = "file"
				entry.Checksum = value
			case "symlink":
				entry.Kind = "symlink"
				entry.SymlinkTarget = value
			case "source":
				entry.SourcePath = value
			case "source-sha256":
				entry.SourceChecksum = value
			}
		}
		if entry.Kind == "" {
			return Manifest{}, fmt.Errorf("manifest entry %q has no type (sha256 or symlink)", entry.Path)
		}
		m.Entries = append(m.Entries, entry)
	}

	return m, nil
}

func readManifest(root string) (Manifest, bool, error) {
	// Try current name first, then legacy fallbacks:
	//   - .governa/manifest (current, AC55)
	//   - .governa-manifest (pre-AC55 flat layout)
	//   - .repokit-manifest (pre-governa rename)
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

func manifestEntryMap(m Manifest) map[string]ManifestEntry {
	out := make(map[string]ManifestEntry, len(m.Entries))
	for _, e := range m.Entries {
		out[e.Path] = e
	}
	return out
}

func acknowledgedEntryMap(m Manifest) map[string]AcknowledgedEntry {
	out := make(map[string]AcknowledgedEntry, len(m.Acknowledged))
	for _, e := range m.Acknowledged {
		out[e.Path] = e
	}
	return out
}

// migrateGovernaLegacyPaths consolidates pre-AC55 metadata paths under the
// `.governa/` directory. Runs at the top of `governa sync` so consumer repos
// transition transparently on their next sync. Emits one stderr log line per
// rename. The ephemeral `.governa-proposed/` tree is removed; sync regenerates
// its replacement under `.governa/proposed/`.
func migrateGovernaLegacyPaths(root string) error {
	governaDirPath := filepath.Join(root, governaDir)

	renames := []struct {
		legacy  string
		current string
	}{
		{legacyPreAC55ManifestFile, manifestFileName},
		{legacyPreAC55SyncReviewFile, syncReviewFile},
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
			// Current path already exists — prefer the current one and remove the legacy.
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

	legacyProposedAbs := filepath.Join(root, legacyPreAC55ProposedDir)
	if info, err := os.Stat(legacyProposedAbs); err == nil && info.IsDir() {
		if err := os.RemoveAll(legacyProposedAbs); err != nil {
			return fmt.Errorf("remove legacy %s: %w", legacyPreAC55ProposedDir, err)
		}
		fmt.Fprintf(os.Stderr, "governa sync: removed legacy %s (regenerated under %s)\n", legacyPreAC55ProposedDir, proposedDirName)
	}

	return nil
}
