package render

import (
	"strings"
	"unicode/utf8"
)

// CodeWidth is how wide a line of code may be on screen before it's cut.
const CodeWidth = 76

// TidyCode makes lines of code look good on screen: tabs become 4 spaces,
// the shared indentation goes away, runs of empty lines shrink to one, and
// long lines are cut. It returns a new slice.
func TidyCode(lines []string) []string {
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
		tidy = append(tidy, cutRunes(l, CodeWidth))
	}
	return tidy
}

func cutRunes(s string, w int) string {
	if utf8.RuneCountInString(s) <= w {
		return s
	}
	r := []rune(s)
	return string(r[:w-1]) + "…"
}
