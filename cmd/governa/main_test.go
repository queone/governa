package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func governaBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "governa")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = filepath.Join(testRepoRoot(t), "cmd", "governa")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build governa binary: %v\n%s", err, out)
	}
	return bin
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	// cmd/governa is two levels below the repo root.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(wd, "..", "..")
}

func TestCLIHelpAlias(t *testing.T) {
	t.Parallel()
	bin := governaBinary(t)
	for _, arg := range []string{"help", "h", "-h", "--help", "-?"} {
		out, _ := exec.Command(bin, arg).CombinedOutput()
		output := string(out)
		if !strings.Contains(output, "governa v") {
			t.Errorf("governa %s: output should contain version header, got:\n%s", arg, output)
		}
		if !strings.Contains(output, "help, h") {
			t.Errorf("governa %s: output should list 'help, h', got:\n%s", arg, output)
		}
		if !strings.Contains(output, "Repo governance templates") {
			t.Errorf("governa %s: output should contain description, got:\n%s", arg, output)
		}
	}
}
