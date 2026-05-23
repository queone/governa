package governarm

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/queone/governa/internal/templates"
)

func writeRMFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func rmFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeRMFile(t, filepath.Join(dir, "AGENTS.md"), "# custom AGENTS.md\n")
	writeRMFile(t, filepath.Join(dir, "README.md"), "# consumer README\n")
	writeRMFile(t, filepath.Join(dir, "CHANGELOG.md"), "# Changelog\n\n| 0.1.0 | preserve README.md customization |\n")
	writeRMFile(t, filepath.Join(dir, "plan.md"), "# Plan\n")
	writeRMFile(t, filepath.Join(dir, "notes", "local.md"), "# local\n")
	writeRMFile(t, filepath.Join(dir, "governa", "ac-template.md"), "# AC\n")
	if err := os.Symlink("AGENTS.md", filepath.Join(dir, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"git", "init", "-q"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "commit.gpgsign", "false"},
		{"git", "add", "-A"},
		{"git", "commit", "-q", "-m", "initial governa apply", "--allow-empty"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	return dir
}

func TestRunEmitsStubAndDiffs(t *testing.T) {
	dir := rmFixture(t)
	exit, err := Run(dir, templates.EmbeddedFS, os.Stdout)
	if err != nil || exit != ExitOK {
		t.Fatalf("exit=%d err=%v", exit, err)
	}
	stem := "governa/ac1-governa-rm-v" + templates.TemplateVersion
	for _, rel := range []string{stem + ".md", stem + "-diffs.md"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("expected %s: %v", rel, err)
		}
	}
	stub, err := os.ReadFile(filepath.Join(dir, stem+".md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"### Routing Decisions",
		"`AGENTS.md`",
		"`CLAUDE.md` — delete symlink",
		"`README.md` — keep; preserve marker:",
		"`notes/local.md` — keep; target-only repo-owned file",
	} {
		if !strings.Contains(string(stub), want) {
			t.Fatalf("stub missing %q:\n%s", want, stub)
		}
	}
	for _, absent := range []string{
		"## Objective Fit",
		"## Director Review",
		"## Implementation Notes",
		"## Documentation Updates",
		"## Critique",
	} {
		if strings.Contains(string(stub), absent) {
			t.Fatalf("stub must not contain %q (retired in AC138):\n%s", absent, stub)
		}
	}
}

func TestRunRefusesEditedEmission(t *testing.T) {
	dir := rmFixture(t)
	if exit, err := Run(dir, templates.EmbeddedFS, os.Stdout); exit != ExitOK || err != nil {
		t.Fatalf("first run exit=%d err=%v", exit, err)
	}
	stub := filepath.Join(dir, "governa/ac1-governa-rm-v"+templates.TemplateVersion+".md")
	if err := os.WriteFile(stub, []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	exit, err := Run(dir, templates.EmbeddedFS, os.Stdout)
	if exit == ExitOK || err == nil || !strings.Contains(err.Error(), "edited since last governa-rm emission") {
		t.Fatalf("expected edit-detection refusal, exit=%d err=%v", exit, err)
	}
}
