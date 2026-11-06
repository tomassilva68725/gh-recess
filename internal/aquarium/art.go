package aquarium

import (
	"strings"
	"unicode/utf8"
)

// fishArt is every fish facing right. Spaces are see-through.
var fishArt = map[Tier][]string{
	Fry:   {"><>"},
	Small: {"><((°>"},
	Medium: {
		"  /|   ",
		"><(  °>",
		"  \\|   ",
	},
	Large: {
		"    /\\     ",
		" \\ /  \\___ ",
		"  )  ((  °>",
		" / \\  ___/ ",
		"    \\/     ",
	},
}

// mirrors swaps the characters that point one way for their twins.
var mirrors = strings.NewReplacer(
	">", "<", "<", ">", "(", ")", ")", "(", "/", "\\", "\\", "/",
	"[", "]", "]", "[", "{", "}", "}", "{",
)

// Art returns a fish's drawing facing right (dir > 0) or left.
func Art(t Tier, dir int) []string {
	lines := fishArt[t]
	if dir > 0 {
		return lines
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		r := []rune(mirrors.Replace(l))
		for a, b := 0, len(r)-1; a < b; a, b = a+1, b-1 {
			r[a], r[b] = r[b], r[a]
		}
		out[i] = string(r)
	}
	return out
}

// artSize is a drawing's width and height.
func artSize(t Tier) (w, h int) {
	lines := fishArt[t]
	for _, l := range lines {
		w = max(w, utf8.RuneCountInString(l))
	}
	return w, len(lines)
}
