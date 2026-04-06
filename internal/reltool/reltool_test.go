package reltool

import "testing"

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
