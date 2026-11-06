package whodunit

import (
	"math/rand/v2"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tomassilva68725/gh-recess/internal/render"
)

const (
	minLines      = 3   // shorter snippets are too hard (or too easy) to guess
	maxLines      = 8   // longer ones flood the screen
	minMeaningful = 3   // lines with real words, not just "}" or ""
	maxLineLen    = 200 // longer lines are minified or generated
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
	return render.TidyCode(windows[rng.IntN(len(windows))])
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
