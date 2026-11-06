package whodunit

import (
	"math/rand/v2"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	minLines      = 3   // shorter snippets are too hard (or too easy) to guess
	maxLines      = 8   // longer ones flood the screen
	minMeaningful = 3   // lines with real words, not just "}" or ""
	maxLineLen    = 200 // longer lines are minified or generated
	maxWidth      = 76  // lines are cut to this width on screen
)

// PickSnippet chooses a few lines from the added hunks of a file, or nil when
// nothing is worth showing.
func PickSnippet(hunks [][]string, rng *rand.Rand) []string {
	var windows [][]string
	for _, h := range hunks {
		h = trimBlank(h)
		if len(h) < minLines || longest(h) > maxLineLen {
			continue
		}
		// Every window of up to maxLines that is still a good snippet.
		for start := 0; start+minLines <= len(h); start++ {
			w := trimBlank(h[start:min(start+maxLines, len(h))])
			if len(w) >= minLines && meaningful(w) >= minMeaningful && !closes(w[0]) {
				windows = append(windows, w)
			}
		}
	}
	if len(windows) == 0 {
		return nil
	}
	return Tidy(windows[rng.IntN(len(windows))])
}

// Tidy makes a snippet look good on screen: tabs become 4 spaces, the shared
// indentation goes away, and long lines are cut. It returns a new slice.
func Tidy(lines []string) []string {
	out := make([]string, len(lines))
	indent := -1
	for i, l := range lines {
		l = strings.TrimRight(strings.ReplaceAll(l, "\t", "    "), " \r")
		out[i] = l
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, " "))
		if indent < 0 || n < indent {
			indent = n
		}
	}
	tidy := out[:0]
	for _, l := range out {
		if len(l) >= indent && indent > 0 {
			l = l[indent:]
		}
		if l == "" && len(tidy) > 0 && tidy[len(tidy)-1] == "" {
			continue // one empty line is enough
		}
		tidy = append(tidy, cut(l, maxWidth))
	}
	return tidy
}

func cut(s string, w int) string {
	if utf8.RuneCountInString(s) <= w {
		return s
	}
	r := []rune(s)
	return string(r[:w-1]) + "…"
}

// trimBlank drops empty lines at both ends.
func trimBlank(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func longest(lines []string) int {
	n := 0
	for _, l := range lines {
		n = max(n, utf8.RuneCountInString(l))
	}
	return n
}

// meaningful counts the lines with at least 3 letters in them.
func meaningful(lines []string) int {
	n := 0
	for _, l := range lines {
		letters := 0
		for _, r := range l {
			if unicode.IsLetter(r) {
				letters++
			}
		}
		if letters >= 3 {
			n++
		}
	}
	return n
}

// closes reports whether a line only closes a block ("}", ");", "end"), which
// makes a confusing first line.
func closes(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "}") || strings.HasPrefix(t, ")") || strings.HasPrefix(t, "]") || t == "end"
}
