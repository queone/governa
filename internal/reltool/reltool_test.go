package reltool

import (
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	t.Parallel()

	cfg, help, err := ParseArgs([]string{"v1.2.3", "release message"})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if help {
		t.Fatal("did not expect help mode")
	}
	if cfg.Tag != "v1.2.3" {
		t.Fatalf("tag = %q, want v1.2.3", cfg.Tag)
	}
	if cfg.Message != "release message" {
		t.Fatalf("message = %q, want release message", cfg.Message)
	}
}

func TestParseArgsRejectsBadTag(t *testing.T) {
	t.Parallel()

	if _, _, err := ParseArgs([]string{"1.2.3", "msg"}); err == nil {
		t.Fatal("expected invalid tag to be rejected")
	}
}

func TestParseArgsHelp(t *testing.T) {
	t.Parallel()

	_, help, err := ParseArgs([]string{"--help"})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if !help {
		t.Fatal("expected help mode")
	}
}

func TestParseArgsNoArgs(t *testing.T) {
	t.Parallel()

	_, help, err := ParseArgs(nil)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if !help {
		t.Fatal("expected no-args usage mode")
	}
}

func TestParseArgsEmptyMessage(t *testing.T) {
	t.Parallel()

	if _, _, err := ParseArgs([]string{"v1.0.0", "  "}); err == nil {
		t.Fatal("expected error for empty message")
	}
}

func TestParseArgsExtraArgs(t *testing.T) {
	t.Parallel()

	if _, _, err := ParseArgs([]string{"v1.0.0", "msg", "extra"}); err == nil {
		t.Fatal("expected error for extra args")
	}
}

func TestParseArgsFlagInMiddle(t *testing.T) {
	t.Parallel()

	if _, _, err := ParseArgs([]string{"v1.0.0", "--help"}); err == nil {
		t.Fatal("expected error for help flag mixed with args")
	}
}

func TestParseArgsUnsupportedOption(t *testing.T) {
	t.Parallel()

	if _, _, err := ParseArgs([]string{"--verbose", "v1.0.0", "msg"}); err == nil {
		t.Fatal("expected error for unsupported option")
	}
}

func TestParseArgsWhitespaceTag(t *testing.T) {
	t.Parallel()

	cfg, _, err := ParseArgs([]string{" v1.0.0 ", "msg"})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if cfg.Tag != "v1.0.0" {
		t.Fatalf("tag = %q, want v1.0.0 (trimmed)", cfg.Tag)
	}
}

// --- confirm tests ---

func TestConfirmYes(t *testing.T) {
	t.Parallel()

	var out strings.Builder
	ok, err := confirm(strings.NewReader("y\n"), &out, "proceed? ")
	if err != nil {
		t.Fatalf("confirm() error = %v", err)
	}
	if !ok {
		t.Fatal("expected confirmation")
	}
}

func TestConfirmNo(t *testing.T) {
	t.Parallel()

	var out strings.Builder
	ok, err := confirm(strings.NewReader("n\n"), &out, "proceed? ")
	if err != nil {
		t.Fatalf("confirm() error = %v", err)
	}
	if ok {
		t.Fatal("expected rejection")
	}
}

func TestConfirmEmpty(t *testing.T) {
	t.Parallel()

	var out strings.Builder
	ok, err := confirm(strings.NewReader("\n"), &out, "proceed? ")
	if err != nil {
		t.Fatalf("confirm() error = %v", err)
	}
	if ok {
		t.Fatal("expected empty input to be treated as no")
	}
}

func TestConfirmUpperY(t *testing.T) {
	t.Parallel()

	var out strings.Builder
	ok, err := confirm(strings.NewReader("Y\n"), &out, "proceed? ")
	if err != nil {
		t.Fatalf("confirm() error = %v", err)
	}
	if !ok {
		t.Fatal("expected Y to be accepted")
	}
}

func TestConfirmEOF(t *testing.T) {
	t.Parallel()

	var out strings.Builder
	ok, err := confirm(strings.NewReader(""), &out, "proceed? ")
	if err != nil {
		t.Fatalf("confirm() error = %v", err)
	}
	if ok {
		t.Fatal("expected EOF to be treated as no")
	}
}

// --- IsHelpArg / Usage tests ---

func TestIsHelpArgValues(t *testing.T) {
	t.Parallel()

	for _, arg := range []string{"-h", "-?", "--help"} {
		if !IsHelpArg(arg) {
			t.Fatalf("expected %q to be help arg", arg)
		}
	}
	for _, arg := range []string{"-v", "help", "--version"} {
		if IsHelpArg(arg) {
			t.Fatalf("did not expect %q to be help arg", arg)
		}
	}
}

func TestUsageContainsBasicInfo(t *testing.T) {
	t.Parallel()

	usage := Usage()
	if !strings.Contains(usage, "rel") {
		t.Fatal("usage should mention rel command")
	}
	if !strings.Contains(usage, "vX.Y.Z") {
		t.Fatal("usage should show version format")
	}
}
