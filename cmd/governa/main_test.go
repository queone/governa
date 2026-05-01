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

// AT for cmd/governa/main_test.go subcommand registration coverage:
// drift-scan appears in printUsage() output.
func TestDriftScanSubcommandListed(t *testing.T) {
	t.Parallel()
	bin := governaBinary(t)
	out, _ := exec.Command(bin, "help").CombinedOutput()
	if !strings.Contains(string(out), "drift-scan") {
		t.Errorf("expected 'drift-scan' in help output, got:\n%s", out)
	}
}

// drift-scan dispatches to the drift-scan handler (not "unknown command").
func TestDriftScanDispatch(t *testing.T) {
	t.Parallel()
	bin := governaBinary(t)
	out, _ := exec.Command(bin, "drift-scan").CombinedOutput()
	if strings.Contains(string(out), "unknown command") {
		t.Errorf("drift-scan should not be unknown, got:\n%s", out)
	}
}

// `governa drift-scan -h` prints drift-scan-specific help.
func TestDriftScanHelp(t *testing.T) {
	t.Parallel()
	bin := governaBinary(t)
	out, _ := exec.Command(bin, "drift-scan", "-h").CombinedOutput()
	if !strings.Contains(string(out), "Scan an adopted-governa repo") {
		t.Errorf("drift-scan help should describe the command, got:\n%s", out)
	}
}
