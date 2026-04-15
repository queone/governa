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
const manifestFileName = ".governa-manifest"
const legacyManifestFileName = ".repokit-manifest"
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
}

type ManifestEntry struct {
	Path           string
	Checksum       string
	SourcePath     string
	SourceChecksum string
	Kind           string // "file" or "symlink"
	SymlinkTarget  string
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
	// Try current name first, then legacy fallback for repos bootstrapped before the rename.
	for _, name := range []string{manifestFileName, legacyManifestFileName} {
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
