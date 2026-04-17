package governance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComputeChecksum(t *testing.T) {
	t.Parallel()
	got := computeChecksum("hello world\n")
	want := "a948904f2f0f479b8f8197694b30184b0d2ed1c1cd2a1ec0fb85d299a192a447"
	if got != want {
		t.Fatalf("computeChecksum() = %q, want %q", got, want)
	}
}

func TestFormatParseManifestRoundTrip(t *testing.T) {
	t.Parallel()
	m := Manifest{
		FormatVersion:   manifestFormatVersion,
		TemplateVersion: "0.1.5",
		Entries: []ManifestEntry{
			{Path: "AGENTS.md", Kind: "file", Checksum: "aaa", SourcePath: "base/AGENTS.md", SourceChecksum: "bbb"},
			{Path: "CLAUDE.md", Kind: "symlink", SymlinkTarget: "AGENTS.md", SourcePath: "base/AGENTS.md"},
			{Path: "README.md", Kind: "file", Checksum: "ccc", SourcePath: "overlays/code/files/README.md.tmpl", SourceChecksum: "ddd"},
		},
	}

	text := formatManifest(m)
	if !strings.HasPrefix(text, manifestFormatVersion+"\n") {
		t.Fatalf("formatted manifest should start with format version, got:\n%s", text)
	}

	parsed, err := parseManifest(text)
	if err != nil {
		t.Fatalf("parseManifest() error = %v", err)
	}
	if parsed.TemplateVersion != m.TemplateVersion {
		t.Fatalf("TemplateVersion = %q, want %q", parsed.TemplateVersion, m.TemplateVersion)
	}
	if len(parsed.Entries) != len(m.Entries) {
		t.Fatalf("len(Entries) = %d, want %d", len(parsed.Entries), len(m.Entries))
	}
	for i, got := range parsed.Entries {
		want := m.Entries[i]
		if got.Path != want.Path || got.Kind != want.Kind || got.Checksum != want.Checksum || got.SourcePath != want.SourcePath || got.SourceChecksum != want.SourceChecksum || got.SymlinkTarget != want.SymlinkTarget {
			t.Fatalf("Entry[%d] = %+v, want %+v", i, got, want)
		}
	}
}

func TestParseManifestRejectsBadVersion(t *testing.T) {
	t.Parallel()
	_, err := parseManifest("governa-manifest-v99\ntemplate-version: 1.0\n")
	if err == nil {
		t.Fatal("expected error for unrecognized format version")
	}
}

func TestParseManifestRejectsMalformedEntry(t *testing.T) {
	t.Parallel()
	_, err := parseManifest(manifestFormatVersion + "\ntemplate-version: 1.0\n\nBADLINE\n")
	if err == nil {
		t.Fatal("expected error for malformed entry")
	}
}

func TestReadManifestMissing(t *testing.T) {
	t.Parallel()
	_, ok, err := readManifest(t.TempDir())
	if err != nil {
		t.Fatalf("readManifest() error = %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for missing manifest")
	}
}

func TestReadManifestCorrupt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, manifestFileName)
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, ok, err := readManifest(dir)
	if err != nil {
		t.Fatalf("readManifest() error = %v (should gracefully fall back)", err)
	}
	if ok {
		t.Fatal("expected ok=false for corrupt manifest")
	}
}

func TestReadManifestValid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	m := Manifest{
		FormatVersion:   manifestFormatVersion,
		TemplateVersion: "0.1.5",
		Entries: []ManifestEntry{
			{Path: "AGENTS.md", Kind: "file", Checksum: "abc123", SourcePath: "base/AGENTS.md", SourceChecksum: "def456"},
		},
	}
	manifestPath := filepath.Join(dir, manifestFileName)
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, []byte(formatManifest(m)), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok, err := readManifest(dir)
	if err != nil {
		t.Fatalf("readManifest() error = %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got.TemplateVersion != "0.1.5" {
		t.Fatalf("TemplateVersion = %q, want 0.1.5", got.TemplateVersion)
	}
	if len(got.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(got.Entries))
	}
}

func TestManifestEntryMap(t *testing.T) {
	t.Parallel()
	m := Manifest{
		Entries: []ManifestEntry{
			{Path: "AGENTS.md", Kind: "file", Checksum: "aaa"},
			{Path: "README.md", Kind: "file", Checksum: "bbb"},
		},
	}
	em := manifestEntryMap(m)
	if em["AGENTS.md"].Checksum != "aaa" {
		t.Fatal("expected AGENTS.md entry")
	}
	if em["README.md"].Checksum != "bbb" {
		t.Fatal("expected README.md entry")
	}
	if _, ok := em["missing.md"]; ok {
		t.Fatal("should not have entry for missing.md")
	}
}

func TestBuildManifestFromOperations(t *testing.T) {
	t.Parallel()

	templateRoot := t.TempDir()
	targetRoot := t.TempDir()

	// Create source files in template root
	mustWriteHelper(t, filepath.Join(templateRoot, "base", "AGENTS.md"), "# AGENTS template\n")
	mustWriteHelper(t, filepath.Join(templateRoot, "TEMPLATE_VERSION"), "0.1.5\n")

	ops := []operation{
		{kind: "write", path: filepath.Join(targetRoot, "AGENTS.md"), content: "# AGENTS rendered\n", source: filepath.Join("base", "AGENTS.md")},
		{kind: "write", path: filepath.Join(targetRoot, "TEMPLATE_VERSION"), content: "0.1.5\n", source: "TEMPLATE_VERSION"},
		{kind: "symlink", path: filepath.Join(targetRoot, "CLAUDE.md"), linkTo: "AGENTS.md", source: filepath.Join("base", "AGENTS.md")},
	}

	m := buildManifest(ops, "0.1.5", os.DirFS(templateRoot), templateRoot, targetRoot)
	if m.TemplateVersion != "0.1.5" {
		t.Fatalf("TemplateVersion = %q, want 0.1.5", m.TemplateVersion)
	}
	if len(m.Entries) != 3 {
		t.Fatalf("len(Entries) = %d, want 3", len(m.Entries))
	}

	em := manifestEntryMap(m)
	agents := em["AGENTS.md"]
	if agents.Kind != "file" {
		t.Fatalf("AGENTS.md kind = %q, want file", agents.Kind)
	}
	if agents.Checksum != computeChecksum("# AGENTS rendered\n") {
		t.Fatal("AGENTS.md checksum mismatch")
	}
	if agents.SourceChecksum != computeChecksum("# AGENTS template\n") {
		t.Fatal("AGENTS.md source checksum mismatch")
	}

	claude := em["CLAUDE.md"]
	if claude.Kind != "symlink" {
		t.Fatalf("CLAUDE.md kind = %q, want symlink", claude.Kind)
	}
	if claude.SymlinkTarget != "AGENTS.md" {
		t.Fatalf("CLAUDE.md symlink target = %q, want AGENTS.md", claude.SymlinkTarget)
	}
}

func TestFormatParseManifestWithParams(t *testing.T) {
	t.Parallel()
	m := Manifest{
		FormatVersion:   manifestFormatVersion,
		TemplateVersion: "0.7.1",
		Params: ManifestParams{
			RepoName: "skout",
			Purpose:  "decision-support CLI for Yahoo Fantasy Baseball",
			Type:     "CODE",
			Stack:    "Go",
		},
		Entries: []ManifestEntry{
			{Path: "AGENTS.md", Kind: "file", Checksum: "aaa", SourcePath: "base/AGENTS.md", SourceChecksum: "bbb"},
		},
	}

	text := formatManifest(m)
	if !strings.Contains(text, "repo-name: skout") {
		t.Fatalf("formatted manifest should contain repo-name, got:\n%s", text)
	}
	if !strings.Contains(text, "stack: Go") {
		t.Fatalf("formatted manifest should contain stack, got:\n%s", text)
	}

	parsed, err := parseManifest(text)
	if err != nil {
		t.Fatalf("parseManifest() error = %v", err)
	}
	if parsed.Params.RepoName != "skout" {
		t.Fatalf("Params.RepoName = %q, want skout", parsed.Params.RepoName)
	}
	if parsed.Params.Purpose != "decision-support CLI for Yahoo Fantasy Baseball" {
		t.Fatalf("Params.Purpose = %q", parsed.Params.Purpose)
	}
	if parsed.Params.Type != "CODE" {
		t.Fatalf("Params.Type = %q, want CODE", parsed.Params.Type)
	}
	if parsed.Params.Stack != "Go" {
		t.Fatalf("Params.Stack = %q, want Go", parsed.Params.Stack)
	}
	if len(parsed.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(parsed.Entries))
	}
}

func TestFormatParseManifestWithoutParams(t *testing.T) {
	t.Parallel()
	// Simulate a pre-AC26 manifest with no params
	m := Manifest{
		FormatVersion:   manifestFormatVersion,
		TemplateVersion: "0.6.0",
		Entries: []ManifestEntry{
			{Path: "AGENTS.md", Kind: "file", Checksum: "aaa"},
		},
	}
	text := formatManifest(m)
	parsed, err := parseManifest(text)
	if err != nil {
		t.Fatalf("parseManifest() error = %v", err)
	}
	if parsed.Params.RepoName != "" {
		t.Fatalf("Params.RepoName should be empty for legacy manifest, got %q", parsed.Params.RepoName)
	}
	if len(parsed.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(parsed.Entries))
	}
}

func TestFormatParseManifestDocParams(t *testing.T) {
	t.Parallel()
	m := Manifest{
		FormatVersion:   manifestFormatVersion,
		TemplateVersion: "0.7.1",
		Params: ManifestParams{
			RepoName:           "myblog",
			Purpose:            "personal blog",
			Type:               "DOC",
			PublishingPlatform: "Hugo",
			Style:              "casual",
		},
	}
	text := formatManifest(m)
	parsed, err := parseManifest(text)
	if err != nil {
		t.Fatalf("parseManifest() error = %v", err)
	}
	if parsed.Params.PublishingPlatform != "Hugo" {
		t.Fatalf("Params.PublishingPlatform = %q, want Hugo", parsed.Params.PublishingPlatform)
	}
	if parsed.Params.Style != "casual" {
		t.Fatalf("Params.Style = %q, want casual", parsed.Params.Style)
	}
	if parsed.Params.Stack != "" {
		t.Fatalf("Params.Stack should be empty for DOC, got %q", parsed.Params.Stack)
	}
}

func mustWriteHelper(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
