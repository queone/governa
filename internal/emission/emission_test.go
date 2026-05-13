package emission

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMarkerRoundTripAndEditDetection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docs", "ac1-test.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteWithMarker(path, "<!-- tool: emitted ", "v1.2.3", "# Body\n"); err != nil {
		t.Fatal(err)
	}
	unedited, err := VerifyUnedited(path, "<!-- tool: emitted ")
	if err != nil || !unedited {
		t.Fatalf("expected unedited marker, unedited=%v err=%v", unedited, err)
	}
	if err := os.WriteFile(path, []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unedited, err = VerifyUnedited(path, "<!-- tool: emitted ")
	if err != nil || unedited {
		t.Fatalf("expected edited marker failure, unedited=%v err=%v", unedited, err)
	}
}

func TestRequireGovernaAdoptedAndPreserveMarkers(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS\n")
	writeTestFile(t, filepath.Join(dir, "docs", "ac-template.md"), "# AC\n")
	writeTestFile(t, filepath.Join(dir, "CHANGELOG.md"), "| 0.1.0 | preserve docs/foo.md customization |\n")
	if err := RequireGovernaAdopted(dir, "tool"); err != nil {
		t.Fatal(err)
	}
	hits := PreserveMarkers(dir, "docs/foo.md")
	if len(hits) != 1 || !strings.Contains(hits[0], "preserve docs/foo.md") {
		t.Fatalf("unexpected preserve markers: %v", hits)
	}
}

func TestIsGovernaCheckout(t *testing.T) {
	dir := t.TempDir()
	if IsGovernaCheckout(dir) {
		t.Fatal("empty tempdir should not register as a governa checkout")
	}
	writeTestFile(t, filepath.Join(dir, "go.mod"), "module github.com/queone/governa\n")
	if err := os.MkdirAll(filepath.Join(dir, "internal", "templates", "base"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !IsGovernaCheckout(dir) {
		t.Fatal("seeded governa-shape tempdir should register as a checkout")
	}
}

func TestAllocateACNumberHandlesEmptyGitHistory(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "docs", "ac3-existing.md"), "# AC3\n")
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	n, reused, err := AllocateACNumber(dir, "drift-scan", "v1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if reused || n != 4 {
		t.Fatalf("AllocateACNumber = n:%d reused:%v, want n:4 reused:false", n, reused)
	}
}
