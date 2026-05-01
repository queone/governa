package main

import (
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

// AT7 — `governa drift-scan` with no args prints usage and exits 2.
func TestNoArgs(t *testing.T) {
	bin := driftScanBinary(t)
	cmd := exec.Command(bin)
	out, err := cmd.CombinedOutput()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 2 {
		t.Errorf("expected exit 2, got: err=%v out=%s", err, out)
	}
	if !strings.Contains(string(out), "missing <repo-path>") && !strings.Contains(string(out), "Usage:") {
		t.Errorf("expected usage message, got:\n%s", out)
	}
}

func TestHelpFlag(t *testing.T) {
	bin := driftScanBinary(t)
	for _, arg := range []string{"-h", "--help"} {
		out, err := exec.Command(bin, arg).CombinedOutput()
		if err != nil {
			t.Errorf("driftscan %s: %v", arg, err)
		}
		if !strings.Contains(string(out), "Usage:") {
			t.Errorf("driftscan %s: missing Usage:, got:\n%s", arg, out)
		}
	}
}
