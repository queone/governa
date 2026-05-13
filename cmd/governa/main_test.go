package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func governaCmd(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(governaBinary(t), args...)
	cmd.Env = append(os.Environ(), "GOVERNA_NO_UPDATE_CHECK=1")
	return cmd
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
	for _, arg := range []string{"help", "h", "-h", "--help", "-?"} {
		out, _ := governaCmd(t, arg).CombinedOutput()
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
	out, _ := governaCmd(t, "help").CombinedOutput()
	if !strings.Contains(string(out), "drift-scan") {
		t.Errorf("expected 'drift-scan' in help output, got:\n%s", out)
	}
}

// drift-scan dispatches to the drift-scan handler (not "unknown command").
// Note: dispatch with no args reaches the drift-scan handler, then fails the
// governa-adoption check (the binary's own cwd at test time is the governa
// source tree, but the cwd of the spawned process is the test's TempDir or
// the test working dir; either way, the failure is from drift-scan, not from
// the top-level unknown-command path).
func TestDriftScanDispatch(t *testing.T) {
	t.Parallel()
	out, _ := governaCmd(t, "drift-scan").CombinedOutput()
	if strings.Contains(string(out), "unknown command") {
		t.Errorf("drift-scan should not be unknown, got:\n%s", out)
	}
}

// `governa drift-scan -h` prints drift-scan-specific help.
func TestDriftScanHelp(t *testing.T) {
	t.Parallel()
	out, _ := governaCmd(t, "drift-scan", "-h").CombinedOutput()
	if !strings.Contains(string(out), "Scan an adopted-governa repo") {
		t.Errorf("drift-scan help should describe the command, got:\n%s", out)
	}
}

// AT13: drift-scan rejects positional arguments — no <repo-path> accepted.
func TestDriftScanRejectsPositionalArg(t *testing.T) {
	t.Parallel()
	out, _ := governaCmd(t, "drift-scan", "/some/path").CombinedOutput()
	if !strings.Contains(string(out), "no positional arguments accepted") {
		t.Errorf("expected positional-arg rejection, got:\n%s", out)
	}
}

func TestRMAndDepsListed(t *testing.T) {
	t.Parallel()
	out, _ := governaCmd(t, "help").CombinedOutput()
	for _, want := range []string{"rm", "deps"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("help missing %q:\n%s", want, out)
		}
	}
}

func TestRMAndDepsHelpFlags(t *testing.T) {
	t.Parallel()
	for _, subcmd := range []string{"rm", "deps"} {
		for _, flag := range []string{"-h", "--help", "-?"} {
			out, err := governaCmd(t, subcmd, flag).CombinedOutput()
			if err != nil {
				t.Fatalf("governa %s %s failed: %v\n%s", subcmd, flag, err, out)
			}
			if !strings.Contains(string(out), "Usage:") {
				t.Fatalf("governa %s %s missing Usage:\n%s", subcmd, flag, out)
			}
		}
	}
}

func TestRemoveAliasRejected(t *testing.T) {
	t.Parallel()
	out, err := governaCmd(t, "remove").CombinedOutput()
	if err == nil || !strings.Contains(string(out), "unknown command: remove") {
		t.Fatalf("expected remove alias rejection, err=%v out=%s", err, out)
	}
}

func TestUpdateCheckRunsOnNonZeroReturn(t *testing.T) {
	bin := governaBinary(t)
	cacheRoot := t.TempDir()
	cachePath := filepath.Join(cacheRoot, "governa", "last-check")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{
		"checked_at":     time.Now().UTC(),
		"latest_version": "v9.9.9",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, body, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "drift-scan")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "XDG_CACHE_HOME="+cacheRoot)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero drift-scan exit, output:\n%s", out)
	}
	if !strings.Contains(string(out), "governa v9.9.9 available") {
		t.Fatalf("expected deferred update notice on non-zero path, got:\n%s", out)
	}
}
