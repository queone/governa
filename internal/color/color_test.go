package color

import (
	"io"
	"os"
	"strings"
	"testing"
)

// In test environments stdout is not a TTY, so enabled == false and all
// color functions return the input string unchanged. The tests below verify
// both the no-color path (input preserved) and the wrap helper directly.

func TestColorFunctionsContainInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		fn   func(any) string
	}{
		{"Gra", Gra},
		{"Grn", Grn},
		{"GrnR", GrnR},
		{"GrnD", GrnD},
		{"Yel", Yel},
		{"Blu", Blu},
		{"Cya", Cya},
		{"Red", Red},
		{"RedR", RedR},
		{"RedD", RedD},
		{"Whi", Whi},
		{"Whi2", Whi2},
		{"BoldW", BoldW},
	}
	for _, tc := range cases {
		got := tc.fn("hello")
		if !strings.Contains(got, "hello") {
			t.Errorf("%s(%q) = %q, does not contain input", tc.name, "hello", got)
		}
		if got == "" {
			t.Errorf("%s(%q) returned empty string", tc.name, "hello")
		}
	}
}

func TestColorFunctionsNoTTY(t *testing.T) {
	t.Parallel()

	// In test environment, enabled is false (stdout is not a char device).
	// Functions must return the bare input string.
	if enabled {
		t.Skip("TTY detected — skipping no-color path test")
	}
	cases := []struct {
		name string
		fn   func(any) string
	}{
		{"Gra", Gra},
		{"Grn", Grn},
		{"GrnR", GrnR},
		{"GrnD", GrnD},
		{"Yel", Yel},
		{"Blu", Blu},
		{"Cya", Cya},
		{"Red", Red},
		{"RedR", RedR},
		{"RedD", RedD},
		{"Whi", Whi},
		{"Whi2", Whi2},
		{"BoldW", BoldW},
	}
	for _, tc := range cases {
		got := tc.fn("test")
		if got != "test" {
			t.Errorf("%s(%q) = %q, want %q (no-TTY path)", tc.name, "test", got, "test")
		}
	}
}

func TestWrapEmptyString(t *testing.T) {
	t.Parallel()

	// wrap with an empty input should not panic regardless of TTY state.
	_ = wrap("32", "")
}

// TestWrapProduces256ColorEscapes verifies wrap() emits the exact ANSI
// 256-color escape format. This test calls wrap() directly so results are
// deterministic regardless of TTY state.
func TestWrapProduces256ColorEscapes(t *testing.T) {
	t.Parallel()

	origEnabled := enabled
	enabled = true
	defer func() { enabled = origEnabled }()

	got := wrap("38;5;2", "ok")
	want := "\033[38;5;2mok\033[0m"
	if got != want {
		t.Fatalf("wrap(\"38;5;2\", \"ok\") = %q, want %q", got, want)
	}
}

// TestColorFunctions256Codes verifies every color function uses the
// documented 256-color escape code. A regression to basic ANSI (e.g. "32"
// instead of "38;5;2") would fail this test.
func TestColorFunctions256Codes(t *testing.T) {
	t.Parallel()

	// Force-enable so we get escape sequences regardless of TTY.
	origEnabled := enabled
	enabled = true
	defer func() { enabled = origEnabled }()

	cases := []struct {
		name string
		fn   func(any) string
		code string // expected escape code between \033[ and m
	}{
		{"Gra", Gra, "38;5;246"},
		{"Grn", Grn, "38;5;2"},
		{"GrnR", GrnR, "7;38;5;2"},
		{"GrnD", GrnD, "38;5;28"},
		{"Yel", Yel, "38;5;3"},
		{"Blu", Blu, "38;5;12"},
		{"Cya", Cya, "38;5;6"},
		{"Red", Red, "38;5;9"},
		{"RedR", RedR, "38;5;15;48;5;1"},
		{"RedD", RedD, "38;5;124"},
		{"Whi", Whi, "38;5;7"},
		{"Whi2", Whi2, "38;5;15"},
		{"BoldW", BoldW, "1;38;5;15"},
	}
	for _, tc := range cases {
		got := tc.fn("x")
		wantPrefix := "\033[" + tc.code + "m"
		if !strings.HasPrefix(got, wantPrefix) {
			t.Errorf("%s: got %q, want prefix %q", tc.name, got, wantPrefix)
		}
		wantSuffix := "\033[0m"
		if !strings.HasSuffix(got, wantSuffix) {
			t.Errorf("%s: got %q, want suffix %q", tc.name, got, wantSuffix)
		}
	}
}

// TestShowPaletteCoversAllFunctions captures ShowPalette output and verifies
// all 13 color function labels are present.
func TestShowPaletteCoversAllFunctions(t *testing.T) {
	t.Parallel()

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	ShowPalette()
	w.Close()
	os.Stdout = oldStdout

	buf, _ := io.ReadAll(r)
	output := string(buf)

	for _, label := range []string{
		"Gra", "Grn", "GrnR", "GrnD",
		"Yel", "Blu", "Cya",
		"Red", "RedR", "RedD",
		"Whi", "Whi2", "BoldW",
	} {
		if !strings.Contains(output, label) {
			t.Errorf("ShowPalette() output missing label %q", label)
		}
	}
}
