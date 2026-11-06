// Package render draws things in the terminal: colors, the big logos,
// bars, spinners and small text helpers.
package render

import "fmt"

// ColorEnabled turns colors on or off for the whole program. The cli package
// sets it at startup (off when output goes to a file or NO_COLOR is set).
var ColorEnabled = false

// ANSI escape codes. The terminal reads "\x1b[36m" as "switch to cyan" and
// "\x1b[0m" as "back to normal".
const (
	reset   = "\x1b[0m"
	bold    = "\x1b[1m"
	dim     = "\x1b[2m"
	italic  = "\x1b[3m"
	red     = "\x1b[31m"
	green   = "\x1b[32m"
	yellow  = "\x1b[33m"
	blue    = "\x1b[34m"
	magenta = "\x1b[35m"
	cyan    = "\x1b[36m"
)

func paint(code, s string) string {
	if !ColorEnabled || s == "" {
		return s
	}
	return code + s + reset
}

func Bold(s string) string    { return paint(bold, s) }
func Dim(s string) string     { return paint(dim, s) }
func Italic(s string) string  { return paint(italic, s) }
func Red(s string) string     { return paint(red, s) }
func Green(s string) string   { return paint(green, s) }
func Yellow(s string) string  { return paint(yellow, s) }
func Blue(s string) string    { return paint(blue, s) }
func Magenta(s string) string { return paint(magenta, s) }
func Cyan(s string) string    { return paint(cyan, s) }

// Color256 paints s with one of the terminal's 256 colors
// (see https://en.wikipedia.org/wiki/ANSI_escape_code#8-bit).
func Color256(n int, s string) string {
	return paint(fmt.Sprintf("\x1b[38;5;%dm", n), s)
}
