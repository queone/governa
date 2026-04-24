package governance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatParseManifestRoundTrip(t *testing.T) {
	t.Parallel()
	m := buildManifest("0.50.0", ManifestParams{
		RepoName: "example",
		Purpose:  "An example repo.",
		Type:     "CODE",
		Stack:    "Go",
	})
	out := formatManifest(m)
	parsed, err := parseManifest(out)
	if err != nil {
		t.Fatalf("parseManifest: %v", err)
	}
	if parsed.TemplateVersion != "0.50.0" {
		t.Errorf("TemplateVersion = %q; want 0.50.0", parsed.TemplateVersion)
	}
	if parsed.Params.RepoName != "example" {
		t.Errorf("Params.RepoName = %q; want example", parsed.Params.RepoName)
	}
	if parsed.Params.Stack != "Go" {
		t.Errorf("Params.Stack = %q; want Go", parsed.Params.Stack)
	}
}

func TestParseManifestRejectsBadVersion(t *testing.T) {
	t.Parallel()
	_, err := parseManifest("unknown-format-v0\ntemplate-version: 1.0.0\n")
	if err == nil {
		t.Fatal("expected error for bad format version")
	}
}

func TestParseManifestToleratesLegacyEntries(t *testing.T) {
	t.Parallel()
	// AC78 legacy-tolerance: new parser skips per-file sha256 lines and
	// acknowledged blocks silently.
	raw := `governa-manifest-v1
template-version: 0.49.0

AGENTS.md sha256:ab source:base/AGENTS.md source-sha256:cd
README.md sha256:ef source:overlays/code/files/README.md.tmpl

acknowledged:
  - path: docs/build-release.md
    consumer-sha: aaaa
    template-sha: bbbb
    template-version: 0.48.0
    reason: legacy carve-out
`
	m, err := parseManifest(raw)
	if err != nil {
		t.Fatalf("parseManifest: %v", err)
	}
	if m.TemplateVersion != "0.49.0" {
		t.Errorf("TemplateVersion = %q; want 0.49.0", m.TemplateVersion)
	}
}

func TestReadManifestMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, found, err := readManifest(dir)
	if err != nil {
		t.Fatalf("readManifest: %v", err)
	}
	if found {
		t.Error("readManifest found a manifest in an empty dir")
	}
}

func TestReadManifestCorruptReturnsEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".governa"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".governa", "manifest"), []byte("garbage"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	m, found, err := readManifest(dir)
	if err != nil {
		t.Fatalf("readManifest: %v", err)
	}
	if found {
		t.Error("found = true for corrupt manifest; want false")
	}
	if m.TemplateVersion != "" {
		t.Errorf("TemplateVersion = %q; want empty on corrupt read", m.TemplateVersion)
	}
}

func TestReadManifestValid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".governa"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := formatManifest(buildManifest("0.50.0", ManifestParams{RepoName: "x", Type: "CODE"}))
	if err := os.WriteFile(filepath.Join(dir, ".governa", "manifest"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	m, found, err := readManifest(dir)
	if err != nil {
		t.Fatalf("readManifest: %v", err)
	}
	if !found {
		t.Fatal("found = false; want true")
	}
	if m.TemplateVersion != "0.50.0" {
		t.Errorf("TemplateVersion = %q; want 0.50.0", m.TemplateVersion)
	}
	if m.Params.RepoName != "x" {
		t.Errorf("Params.RepoName = %q; want x", m.Params.RepoName)
	}
}

func TestFormatManifestOmitsEmptyParams(t *testing.T) {
	t.Parallel()
	m := buildManifest("0.50.0", ManifestParams{RepoName: "x"})
	out := formatManifest(m)
	if !strings.Contains(out, "repo-name: x") {
		t.Errorf("formatted manifest missing repo-name line; got:\n%s", out)
	}
	for _, absent := range []string{"purpose:", "type:", "stack:", "publishing-platform:", "style:"} {
		if strings.Contains(out, absent) {
			t.Errorf("formatted manifest contains unexpected line %q; got:\n%s", absent, out)
		}
	}
}
