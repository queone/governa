package depscheck

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withDepsCWD(t *testing.T, dir string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

func writeDepsFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func adoptedDepsFixture(t *testing.T, goMod bool) string {
	t.Helper()
	dir := t.TempDir()
	writeDepsFile(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS.md\n")
	writeDepsFile(t, filepath.Join(dir, "governa", "ac-template.md"), "# AC\n")
	if goMod {
		writeDepsFile(t, filepath.Join(dir, "go.mod"), "module example.com/test\n\ngo 1.25\n")
	}
	return dir
}

func TestDepsNoGoModAdoptedDocExitsZero(t *testing.T) {
	dir := adoptedDepsFixture(t, false)
	withDepsCWD(t, dir)
	var out, errOut bytes.Buffer
	exit, err := RunCLI(nil, &out, &errOut)
	if err != nil || exit != ExitOK {
		t.Fatalf("exit=%d err=%v", exit, err)
	}
	if !strings.Contains(errOut.String(), "deps is CODE-only") {
		t.Fatalf("expected CODE-only message, got %q", errOut.String())
	}
}

func TestDepsAdoptionRequired(t *testing.T) {
	dir := t.TempDir()
	writeDepsFile(t, filepath.Join(dir, "go.mod"), "module example.com/test\n\ngo 1.25\n")
	withDepsCWD(t, dir)
	exit, err := RunCLI(nil, &bytes.Buffer{}, &bytes.Buffer{})
	if exit == ExitOK || err == nil || !strings.Contains(err.Error(), "not a governa-adopted repo") {
		t.Fatalf("expected adoption error, exit=%d err=%v", exit, err)
	}
}

func TestDepsAllowsGovernaSource(t *testing.T) {
	dir := t.TempDir()
	writeDepsFile(t, filepath.Join(dir, "go.mod"), "module github.com/queone/governa\n\ngo 1.25\n")
	if err := os.MkdirAll(filepath.Join(dir, "internal", "templates", "base"), 0o755); err != nil {
		t.Fatal(err)
	}
	withDepsCWD(t, dir)
	oldRun := runGoList
	runGoList = func(string) ([]byte, error) {
		return []byte(`{"Path":"github.com/queone/governa","Main":true}
{"Path":"github.com/queone/governa-color","Version":"v1.0.0"}`), nil
	}
	t.Cleanup(func() { runGoList = oldRun })
	var out bytes.Buffer
	exit, err := RunCLI(nil, &out, &bytes.Buffer{})
	if err != nil || exit != ExitOK {
		t.Fatalf("exit=%d err=%v", exit, err)
	}
	if !strings.Contains(out.String(), "github.com/queone/governa-color") {
		t.Fatalf("expected governa source deps report, got:\n%s", out.String())
	}
}

func TestDepsReportHighlightsGovernaHelpers(t *testing.T) {
	dir := adoptedDepsFixture(t, true)
	writeDepsFile(t, filepath.Join(dir, "go.sum"), "example checksum\n")
	beforeMod := fileSHA(t, filepath.Join(dir, "go.mod"))
	beforeSum := fileSHA(t, filepath.Join(dir, "go.sum"))
	withDepsCWD(t, dir)
	oldRun := runGoList
	runGoList = func(string) ([]byte, error) {
		return []byte(`{"Path":"example.com/test","Main":true}
{"Path":"github.com/queone/governa-color","Version":"v1.0.0","Update":{"Version":"v1.2.0"}}
{"Path":"example.com/other","Version":"v1.0.0"}`), nil
	}
	t.Cleanup(func() { runGoList = oldRun })
	var out bytes.Buffer
	exit, err := RunCLI(nil, &out, &bytes.Buffer{})
	if err != nil || exit != ExitOK {
		t.Fatalf("exit=%d err=%v", exit, err)
	}
	got := out.String()
	for _, want := range []string{"governa helper libraries", "github.com/queone/governa-color", "direct dependencies", "example.com/other"} {
		if !strings.Contains(got, want) {
			t.Fatalf("report missing %q:\n%s", want, got)
		}
	}
	if got := fileSHA(t, filepath.Join(dir, "go.mod")); got != beforeMod {
		t.Fatal("go.mod changed during deps report")
	}
	if got := fileSHA(t, filepath.Join(dir, "go.sum")); got != beforeSum {
		t.Fatal("go.sum changed during deps report")
	}
	for _, want := range []string{
		"github.com/queone/governa-color  v1.0.0  v1.2.0  ",
		"example.com/other                v1.0.0  v1.0.0  ",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("aligned report missing %q:\n%s", want, got)
		}
	}
}

func fileSHA(t *testing.T, path string) [32]byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(data)
}
