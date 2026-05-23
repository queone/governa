package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func driftScanBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "driftscan")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build driftscan: %v\n%s", err, out)
	}
	return bin
}

func driftScanCmd(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(driftScanBinary(t), args...)
	cmd.Env = append(os.Environ(), "GOVERNA_NO_UPDATE_CHECK=1")
	return cmd
}

// AC136: drift-scan accepts no positional arguments and runs against cwd.
// When cwd is not a governa-adopted repo, the tool hard-errors with a
// non-zero exit and a recovery-guidance message.
func TestNoArgs(t *testing.T) {
	cmd := driftScanCmd(t)
	out, err := cmd.CombinedOutput()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() == 0 {
		t.Errorf("expected non-zero exit from non-adopted cwd, got: err=%v out=%s", err, out)
	}
	if !strings.Contains(string(out), "not a governa-adopted repo") {
		t.Errorf("expected adoption-check error, got:\n%s", out)
	}
}

// AC136: drift-scan rejects any positional argument with a clear error.
func TestRejectsPositionalArg(t *testing.T) {
	out, err := driftScanCmd(t, "/some/path").CombinedOutput()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() == 0 {
		t.Errorf("expected non-zero exit when passing a positional argument, got: err=%v out=%s", err, out)
	}
	if !strings.Contains(string(out), "no positional arguments accepted") {
		t.Errorf("expected positional-arg rejection message, got:\n%s", out)
	}
}

// AC136: drift-scan succeeds when invoked from an adopted-fixture cwd.
// Build the binary, build a minimal adopted-governa fixture, run the
// binary with the fixture as cwd, and verify the emission files appear.
func TestNoArgsSucceedsInAdoptedRepo(t *testing.T) {
	bin := driftScanBinary(t)

	dir := t.TempDir()
	mustWrite := func(p, content string) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(filepath.Join(dir, "AGENTS.md"), "# AGENTS.md\n")
	mustWrite(filepath.Join(dir, "plan.md"), "# Plan\n\n## Ideas To Explore\n\n- IE1: x\n")
	mustWrite(filepath.Join(dir, "CHANGELOG.md"), "# Changelog\n\n| 0.1.0 | initial |\n")
	mustWrite(filepath.Join(dir, "governa/ac-template.md"), "# AC template\n")
	for _, args := range [][]string{
		{"git", "init", "-q"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "commit.gpgsign", "false"},
		{"git", "add", "-A"},
		{"git", "commit", "-q", "-m", "initial", "--allow-empty"},
	} {
		c := exec.Command(args[0], args[1:]...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(), "GOVERNA_NO_UPDATE_CHECK=1")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("drift-scan no-arg from adopted cwd failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "wrote governa/ac1-drift-scan-v") {
		t.Errorf("expected stdout summary referencing emitted paths, got:\n%s", out)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "governa/ac1-drift-scan-v*.md"))
	if len(matches) == 0 {
		t.Errorf("expected at least one ac1-drift-scan-v*.md file emitted under governa/, found none")
	}
}

func TestHelpFlag(t *testing.T) {
	for _, arg := range []string{"-h", "--help"} {
		out, err := driftScanCmd(t, arg).CombinedOutput()
		if err != nil {
			t.Errorf("driftscan %s: %v", arg, err)
		}
		if !strings.Contains(string(out), "Usage:") {
			t.Errorf("driftscan %s: missing Usage:, got:\n%s", arg, out)
		}
	}
}
