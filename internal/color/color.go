// Package color provides simple ANSI terminal color helpers for CLI output.
// Colors are suppressed when stdout is not a terminal or when NO_COLOR is set.
package color

import (
	"fmt"
	"os"
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

func Gra(v any) string  { return wrap("90", v) }
func Grn(v any) string  { return wrap("32", v) }
func GrnR(v any) string { return wrap("7;32", v) }
func Yel(v any) string  { return wrap("33", v) }
func Blu(v any) string  { return wrap("94", v) }
func Cya(v any) string  { return wrap("36", v) }
func Red(v any) string  { return wrap("91", v) }
func RedR(v any) string { return wrap("97;41", v) }
func Whi(v any) string  { return wrap("37", v) }
func Whi2(v any) string { return wrap("97", v) }
