package governance

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/queone/governa/internal/templates"
)

// Helper: build a fixture target directory with the listed relative-path
// files and contents. Returns the absolute target path.
func newFixtureTarget(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		abs := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return dir
}

func TestParseFlagsSyncDefaults(t *testing.T) {
	t.Parallel()
	cfg, help, err := parseFlags(ModeSync, []string{"--target", "/tmp/nope"})
	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	if help {
		t.Fatal("unexpected help request")
	}
	if cfg.Mode != ModeSync {
		t.Errorf("Mode = %q; want %q", cfg.Mode, ModeSync)
	}
	if cfg.Target != "/tmp/nope" {
		t.Errorf("Target = %q; want /tmp/nope", cfg.Target)
	}
	if cfg.AssumeYes || cfg.AssumeNo {
		t.Errorf("expected no batch flags by default; got yes=%v no=%v", cfg.AssumeYes, cfg.AssumeNo)
	}
}

func TestParseFlagsAssumeYesNoMutuallyExclusive(t *testing.T) {
	t.Parallel()
	_, _, err := parseFlags(ModeSync, []string{"--yes", "--no", "--target", "/tmp/x"})
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("expected mutually-exclusive error; got %v", err)
	}
}

func TestModeHelpSyncDescribesCollisionPrompt(t *testing.T) {
	t.Parallel()
	help := ModeHelp(ModeSync)
	if help == "" {
		t.Fatal("ModeHelp returned empty")
	}
	if !strings.Contains(help, "--yes") || !strings.Contains(help, "--no") {
		t.Errorf("sync help missing --yes/--no flags: %q", help)
	}
	if !strings.Contains(help, "keep") || !strings.Contains(help, "overwrite") || !strings.Contains(help, "skip") {
		t.Errorf("sync help missing collision prompt verbs: %q", help)
	}
}

func TestModeHelpUnknownMode(t *testing.T) {
	t.Parallel()
	if got := ModeHelp(Mode("enhance")); got != "" {
		t.Errorf("removed mode 'enhance' should have empty help; got %q", got)
	}
	if got := ModeHelp(Mode("ack")); got != "" {
		t.Errorf("removed mode 'ack' should have empty help; got %q", got)
	}
}

func TestRunWithFSRejectsUnsupportedMode(t *testing.T) {
	t.Parallel()
	err := RunWithFS(templates.EmbeddedFS, "", Config{Mode: Mode("enhance")})
	if err == nil || !strings.Contains(err.Error(), "unsupported mode") {
		t.Fatalf("expected unsupported-mode error; got %v", err)
	}
}

func TestInferPurposeFromReadme(t *testing.T) {
	t.Parallel()
	dir := newFixtureTarget(t, map[string]string{
		"README.md": "# My Repo\n\nDoes a specific thing for specific users.\n",
	})
	got := inferPurpose(dir)
	want := "Does a specific thing for specific users."
	if got != want {
		t.Errorf("inferPurpose = %q; want %q", got, want)
	}
}

func TestInferPurposeNoReadme(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if got := inferPurpose(dir); got != "" {
		t.Errorf("inferPurpose without README = %q; want empty", got)
	}
}

func TestInferStackFromGoMod(t *testing.T) {
	t.Parallel()
	dir := newFixtureTarget(t, map[string]string{
		"go.mod": "module x\n\ngo 1.25\n",
	})
	if got := inferStack(dir); got != "Go" {
		t.Errorf("inferStack = %q; want Go", got)
	}
}

func TestDetectSyncModeNewRepo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if got := detectSyncMode(dir); got != "new" {
		t.Errorf("detectSyncMode on fresh dir = %q; want new", got)
	}
}

func TestDetectSyncModeReSync(t *testing.T) {
	t.Parallel()
	dir := newFixtureTarget(t, map[string]string{
		".governa/manifest": "governa-manifest-v1\ntemplate-version: 0.49.0\n",
	})
	if got := detectSyncMode(dir); got != "re-sync" {
		t.Errorf("detectSyncMode with manifest = %q; want re-sync", got)
	}
}

func TestBuildManifestMinimalShape(t *testing.T) {
	t.Parallel()
	params := ManifestParams{
		RepoName: "x",
		Purpose:  "p",
		Type:     "CODE",
		Stack:    "Go",
	}
	m := buildManifest("1.2.3", params)
	if m.TemplateVersion != "1.2.3" {
		t.Errorf("TemplateVersion = %q; want 1.2.3", m.TemplateVersion)
	}
	if m.Params.RepoName != "x" {
		t.Errorf("Params.RepoName = %q; want x", m.Params.RepoName)
	}
	if m.FormatVersion != "governa-manifest-v1" {
		t.Errorf("FormatVersion = %q; want governa-manifest-v1", m.FormatVersion)
	}
}

func TestFormatManifestNoEntriesNoAcknowledged(t *testing.T) {
	t.Parallel()
	m := buildManifest("0.50.0", ManifestParams{RepoName: "x", Purpose: "p", Type: "CODE"})
	out := formatManifest(m)
	if strings.Contains(out, "sha256:") {
		t.Errorf("formatted manifest must not contain sha256 entries; got:\n%s", out)
	}
	if strings.Contains(out, "acknowledged:") {
		t.Errorf("formatted manifest must not contain acknowledged block; got:\n%s", out)
	}
	if !strings.Contains(out, "template-version: 0.50.0") {
		t.Errorf("formatted manifest missing template-version line; got:\n%s", out)
	}
}

func TestParseManifestIgnoresLegacyEntries(t *testing.T) {
	t.Parallel()
	raw := `governa-manifest-v1
template-version: 0.49.0
repo-name: x
purpose: p
type: CODE

AGENTS.md sha256:deadbeef source:base/AGENTS.md source-sha256:feedface

acknowledged:
  - path: README.md
    consumer-sha: aaaa
    template-sha: bbbb
    template-version: 0.48.0
    reason: legacy ack
`
	m, err := parseManifest(raw)
	if err != nil {
		t.Fatalf("parseManifest: %v", err)
	}
	if m.TemplateVersion != "0.49.0" {
		t.Errorf("TemplateVersion = %q; want 0.49.0", m.TemplateVersion)
	}
	if m.Params.RepoName != "x" {
		t.Errorf("Params.RepoName = %q; want x", m.Params.RepoName)
	}
	// Legacy per-entry and acknowledged lines should not crash the parser.
}

func TestResolveCollisionDryRunAutoSkips(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	choice, err := resolveCollision(filepath.Join(dir, "f"), dir, Config{DryRun: true})
	if err != nil {
		t.Fatalf("resolveCollision dry-run: %v", err)
	}
	if choice != choiceSkip {
		t.Errorf("dry-run choice = %v; want choiceSkip", choice)
	}
}

func TestResolveCollisionAssumeYes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	choice, err := resolveCollision(filepath.Join(dir, "f"), dir, Config{AssumeYes: true})
	if err != nil {
		t.Fatalf("resolveCollision --yes: %v", err)
	}
	if choice != choiceOverwrite {
		t.Errorf("--yes choice = %v; want choiceOverwrite", choice)
	}
}

func TestResolveCollisionAssumeNo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	choice, err := resolveCollision(filepath.Join(dir, "f"), dir, Config{AssumeNo: true})
	if err != nil {
		t.Fatalf("resolveCollision --no: %v", err)
	}
	if choice != choiceKeep {
		t.Errorf("--no choice = %v; want choiceKeep", choice)
	}
}

func TestResolveCollisionNonTTYErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// bytes.Buffer is not a *os.File, so isTTY reports false.
	in := &bytes.Buffer{}
	_, err := resolveCollision(filepath.Join(dir, "f"), dir, Config{Input: in})
	if err == nil {
		t.Fatal("expected non-TTY error; got nil")
	}
	if !strings.Contains(err.Error(), "--yes") || !strings.Contains(err.Error(), "--no") {
		t.Errorf("non-TTY error should point at --yes/--no; got %v", err)
	}
}

func TestResolveCollisionInteractiveKeep(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"k\n", "keep\n", "KEEP\n", "  k  \n"} {
		got, ok := parseCollisionReply(in)
		if !ok || got != choiceKeep {
			t.Errorf("parseCollisionReply(%q) = (%v, %v); want (choiceKeep, true)", in, got, ok)
		}
	}
}

func TestResolveCollisionInteractiveOverwrite(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"o\n", "overwrite\n", "OVERWRITE\n"} {
		got, ok := parseCollisionReply(in)
		if !ok || got != choiceOverwrite {
			t.Errorf("parseCollisionReply(%q) = (%v, %v); want (choiceOverwrite, true)", in, got, ok)
		}
	}
}

func TestResolveCollisionInteractiveSkip(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"s\n", "skip\n", "SKIP\n"} {
		got, ok := parseCollisionReply(in)
		if !ok || got != choiceSkip {
			t.Errorf("parseCollisionReply(%q) = (%v, %v); want (choiceSkip, true)", in, got, ok)
		}
	}
}

func TestResolveCollisionInteractiveUnknownVerb(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"", "\n", "yes\n", "nope\n", "foo\n"} {
		if _, ok := parseCollisionReply(in); ok {
			t.Errorf("parseCollisionReply(%q) unexpectedly succeeded", in)
		}
	}
}

func TestValidateConfigSyncRequiresParams(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{Mode: ModeSync})
	if err == nil {
		t.Fatal("expected validation error for empty sync config")
	}
}

func TestValidateConfigSyncWithCodeParams(t *testing.T) {
	t.Parallel()
	err := validateConfig(Config{
		Mode:     ModeSync,
		RepoName: "x",
		Purpose:  "p",
		Type:     RepoTypeCode,
		Stack:    "Go",
	})
	if err != nil {
		t.Fatalf("validateConfig: %v", err)
	}
}

func TestDeriveTypeFromShape(t *testing.T) {
	t.Parallel()
	cases := map[string]RepoType{
		"likely CODE": RepoTypeCode,
		"likely DOC":  RepoTypeDoc,
		"mixed":       "",
		"":            "",
	}
	for in, want := range cases {
		if got := deriveTypeFromShape(in); got != want {
			t.Errorf("deriveTypeFromShape(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestMigrateGovernaLegacyPathsRemovesAC78Artifacts(t *testing.T) {
	t.Parallel()
	dir := newFixtureTarget(t, map[string]string{
		".governa/sync-review.md":        "stale review artifact",
		".governa/config":                "critique-mode: integrated",
		".governa/proposed/AGENTS.md":    "stale proposed",
		".governa/feedback/ac60-sync.md": "stale feedback",
	})
	if err := migrateGovernaLegacyPaths(dir); err != nil {
		t.Fatalf("migrateGovernaLegacyPaths: %v", err)
	}
	for _, rel := range []string{".governa/sync-review.md", ".governa/config", ".governa/proposed", ".governa/feedback"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); !os.IsNotExist(err) {
			t.Errorf("%s should have been removed by AC78 migration; got err=%v", rel, err)
		}
	}
}

func TestReadManifestMissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, found, err := readManifest(dir)
	if err != nil {
		t.Fatalf("readManifest: %v", err)
	}
	if found {
		t.Error("found = true for empty dir; want false")
	}
}
