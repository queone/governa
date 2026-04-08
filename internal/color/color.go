// Package color provides simple ANSI terminal color helpers for CLI output.
// Colors are suppressed when stdout is not a terminal or when NO_COLOR is set.
package color

import (
	"fmt"
	"os"
	"strings"
)

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

func Gra(v any) string   { return wrap("90", v) }
func Grn(v any) string   { return wrap("32", v) }
func GrnR(v any) string  { return wrap("7;32", v) }
func Yel(v any) string   { return wrap("33", v) }
func Blu(v any) string   { return wrap("94", v) }
func Cya(v any) string   { return wrap("36", v) }
func Red(v any) string   { return wrap("91", v) }
func RedR(v any) string  { return wrap("97;41", v) }
func Whi(v any) string   { return wrap("37", v) }
func Whi2(v any) string  { return wrap("97", v) }
func BoldW(v any) string { return wrap("1;97", v) }

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
