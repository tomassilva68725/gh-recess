package render

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

// Width is how many columns s takes on screen, ignoring color codes.
// (It counts runes, so keep emoji out of text that needs to line up.)
func Width(s string) int {
	return utf8.RuneCountInString(ansiRE.ReplaceAllString(s, ""))
}

// Pad adds spaces on the right until s is w columns wide.
func Pad(s string, w int) string {
	if d := w - Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

// Trunc cuts s to at most w runes, ending in "…" when it had to cut.
func Trunc(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w <= 1 {
		return string(r[:max(w, 0)])
	}
	return string(r[:w-1]) + "…"
}

// Plural writes "1 commit" or "3 commits".
func Plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return Number(n) + " " + word + "s"
}

// Number writes n with thousands separators: 12345 → "12,345".
func Number(n int) string {
	s := strconv.Itoa(int(math.Abs(float64(n))))
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	if n < 0 {
		return "-" + s
	}
	return s
}

// Date writes "Nov 3", adding the year when it isn't this year.
func Date(t time.Time) string {
	if t.Year() != time.Now().Year() {
		return t.Format("Jan 2, 2006")
	}
	return t.Format("Jan 2")
}

// Age writes a length of time the way people say it: "12 minutes",
// "3 hours", "4 days", "5 months", "2 years".
func Age(d time.Duration) string {
	switch {
	case d < time.Hour:
		return Plural(max(int(d.Minutes()), 1), "minute")
	case d < 24*time.Hour:
		return Plural(int(d.Hours()), "hour")
	case d < 60*24*time.Hour:
		return Plural(int(d.Hours()/24), "day")
	case d < 730*24*time.Hour:
		return Plural(int(d.Hours()/24/30), "month")
	default:
		return Plural(int(d.Hours()/24/365), "year")
	}
}

// Bytes writes a file size: "512 B", "11.6 KB", "3.2 MB".
func Bytes(n int64) string {
	const unit = 1024
	if n < unit {
		return strconv.FormatInt(n, 10) + " B"
	}
	f, i := float64(n), 0
	for f >= unit && i < 4 {
		f /= unit
		i++
	}
	return strconv.FormatFloat(f, 'f', 1, 64) + " " + string("BKMGT"[i]) + "B"
}

// Center pads s on both sides to w columns.
func Center(s string, w int) string {
	d := w - Width(s)
	if d <= 0 {
		return s
	}
	return strings.Repeat(" ", d/2) + s + strings.Repeat(" ", d-d/2)
}

// Rule is a horizontal line of w heavy dashes.
func Rule(w int) string {
	return strings.Repeat("━", max(w, 0))
}

// Title draws a section header that fills w columns: "━━ CASE 3/10 ━━━━━━━━".
func Title(label string, w int) string {
	head := "━━ " + label + " "
	return head + Rule(w-Width(head))
}
