// Package color provides ANSI 256-color terminal helpers for CLI output.
// All colors use the 256-color escape format (38;5;N for foreground,
// 48;5;N for background). Colors are suppressed when stdout is not a
// terminal (piped output) or when the NO_COLOR environment variable is
// set (https://no-color.org).
//
// Palette reference (256-color index):
//
//	0–7    standard colors (black, red, green, yellow, blue, magenta, cyan, white)
//	8–15   bright variants of 0–7
//	16–231 6×6×6 color cube
//	232–255 grayscale ramp
package color

import (
	"fmt"
	"os"
	"strings"
)

// enabled is true when the terminal supports color output.
var enabled = func() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}()

func wrap(code string, v any) string {
	s := fmt.Sprint(v)
	if !enabled {
		return s
	}
	return "\033[" + code + "m" + s + "\033[0m"
}

// Gra renders v in gray (246).
func Gra(v any) string { return wrap("38;5;246", v) }

// Grn renders v in green (2).
func Grn(v any) string { return wrap("38;5;2", v) }

// GrnR renders v in reverse video green (green background, dark text).
func GrnR(v any) string { return wrap("7;38;5;2", v) }

// GrnD renders v in dark green (28).
func GrnD(v any) string { return wrap("38;5;28", v) }

// Yel renders v in yellow (3).
func Yel(v any) string { return wrap("38;5;3", v) }

// Blu renders v in bright blue (12).
func Blu(v any) string { return wrap("38;5;12", v) }

// Cya renders v in cyan (6).
func Cya(v any) string { return wrap("38;5;6", v) }

// Red renders v in bright red (9).
func Red(v any) string { return wrap("38;5;9", v) }

// RedR renders v in white text on red background (15 on 1).
func RedR(v any) string { return wrap("38;5;15;48;5;1", v) }

// RedD renders v in dark red (124).
func RedD(v any) string { return wrap("38;5;124", v) }

// Whi renders v in white (7).
func Whi(v any) string { return wrap("38;5;7", v) }

// Whi2 renders v in bright white (15).
func Whi2(v any) string { return wrap("38;5;15", v) }

// BoldW renders v in bold bright white (15).
func BoldW(v any) string { return wrap("1;38;5;15", v) }

// ShowPalette prints a labeled swatch of every color function to stdout.
// Useful for verifying terminal rendering and choosing colors.
func ShowPalette() {
	sample := "The quick brown fox"
	entries := []struct {
		name string
		fn   func(any) string
		code string
	}{
		{"Gra ", Gra, "38;5;246"},
		{"Grn ", Grn, "38;5;2"},
		{"GrnR", GrnR, "7;38;5;2"},
		{"GrnD", GrnD, "38;5;28"},
		{"Yel ", Yel, "38;5;3"},
		{"Blu ", Blu, "38;5;12"},
		{"Cya ", Cya, "38;5;6"},
		{"Red ", Red, "38;5;9"},
		{"RedR", RedR, "38;5;15;48;5;1"},
		{"RedD", RedD, "38;5;124"},
		{"Whi ", Whi, "38;5;7"},
		{"Whi2", Whi2, "38;5;15"},
		{"BoldW", BoldW, "1;38;5;15"},
	}
	fmt.Println(BoldW("Color palette (256-color ANSI)"))
	fmt.Println()
	for _, e := range entries {
		fmt.Printf("  %-6s %-20s  %s\n", e.name, e.fn(sample), Gra(e.code))
	}
	fmt.Println()
}

// UsageLine is a single flag+description pair for FormatUsage.
type UsageLine struct {
	Flag string
	Desc string
}

func formatFlag(flag string) (string, int) {
	rawLen := len(flag)
	idx := strings.LastIndex(flag, " ")
	if idx < 0 {
		return flag, rawLen
	}
	suffix := flag[idx+1:]
	switch suffix {
	case "string", "int", "float", "bool", "duration":
		return flag[:idx+1] + Gra(suffix), rawLen
	}
	return flag, rawLen
}

// FormatUsage builds a formatted help string with a heading, flag table, and optional footer.
func FormatUsage(heading string, lines []UsageLine, footer string) string {
	var b strings.Builder
	b.WriteString(BoldW("Usage:"))
	b.WriteString(" ")
	b.WriteString(heading)
	b.WriteString("\n")
	for _, l := range lines {
		flag, flagLen := formatFlag(l.Flag)
		col := 2 + flagLen
		b.WriteString("  ")
		b.WriteString(flag)
		if col < 38 {
			b.WriteString(strings.Repeat(" ", 38-col))
		} else {
			b.WriteString("  ")
		}
		b.WriteString(l.Desc)
		b.WriteString("\n")
	}
	if footer != "" {
		b.WriteString("\n")
		b.WriteString(footer)
		if !strings.HasSuffix(footer, "\n") {
			b.WriteString("\n")
		}
	}
	return b.String()
}
