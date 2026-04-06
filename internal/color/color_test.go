package color

import (
	"strings"
	"testing"
)

func TestColorFunctionsContainInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		fn   func(any) string
	}{
		{"Gra", Gra},
		{"Grn", Grn},
		{"GrnR", GrnR},
		{"Yel", Yel},
		{"Blu", Blu},
		{"Cya", Cya},
		{"Red", Red},
		{"RedR", RedR},
		{"Whi", Whi},
		{"Whi2", Whi2},
	}
	for _, tc := range cases {
		got := tc.fn("hello")
		if !strings.Contains(got, "hello") {
			t.Fatalf("%s(%q) = %q, does not contain input", tc.name, "hello", got)
		}
	}
}

func TestColorFunctionsNoTTY(t *testing.T) {
	t.Parallel()

	if enabled {
		t.Skip("TTY detected; skipping no-color path test")
	}
	cases := []struct {
		name string
		fn   func(any) string
	}{
		{"Gra", Gra},
		{"Grn", Grn},
		{"GrnR", GrnR},
		{"Yel", Yel},
		{"Blu", Blu},
		{"Cya", Cya},
		{"Red", Red},
		{"RedR", RedR},
		{"Whi", Whi},
		{"Whi2", Whi2},
	}
	for _, tc := range cases {
		if got := tc.fn("test"); got != "test" {
			t.Fatalf("%s(%q) = %q, want %q", tc.name, "test", got, "test")
		}
	}
}

func TestWrapEmptyString(t *testing.T) {
	t.Parallel()

	_ = wrap("32", "")
}
