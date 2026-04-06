package main

import (
	"fmt"
	"os"
)

var colorEnabled = func() bool {
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

func wrapColor(code string, v any) string {
	s := fmt.Sprint(v)
	if !colorEnabled {
		return s
	}
	return "\033[" + code + "m" + s + "\033[0m"
}

func gra(v any) string { return wrapColor("90", v) }
func grn(v any) string { return wrapColor("32", v) }
func yel(v any) string { return wrapColor("33", v) }
func cya(v any) string { return wrapColor("36", v) }
func red(v any) string { return wrapColor("91", v) }
