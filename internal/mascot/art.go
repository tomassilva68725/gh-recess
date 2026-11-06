package mascot

import (
	"strings"
	"unicode/utf8"
)

// face is the placeholder in the art where the 3-character face goes.
const face = "FFF"

// bodies is the mascot at each stage. Rows of one body are the same width.
var bodies = map[Stage][]string{
	Egg: {
		"   ___   ",
		"  /   \\  ",
		" | FFF | ",
		" |     | ",
		"  \\___/  ",
	},
	Baby: {
		"  .---.  ",
		" ( FFF ) ",
		"  '---'  ",
		"  \"   \"  ",
	},
	Kid: { // an antenna
		"    o    ",
		"  .-|-.  ",
		" ( FFF ) ",
		" (     ) ",
		"  `\"-\"'  ",
	},
	Teen: { // a cap, worn sideways
		"  _____    ",
		" |_____|== ",
		"  ( FFF )  ",
		" /(     )\\ ",
		"   (   )   ",
		"   \"   \"   ",
	},
	Adult: { // a crown
		"   \\^^^/   ",
		"  ( FFF )  ",
		" /|     |\\ ",
		"  |  *  |  ",
		"  |_____|  ",
		"   || ||   ",
	},
}

var ghostBody = []string{
	"  .---.  ",
	" / FFF \\ ",
	" |     | ",
	" |     | ",
	" '^^^^^' ",
}

var faces = map[Mood]string{
	Content: "•_•",
	Happy:   "^_^",
	Excited: "*o*",
	Hungry:  "°o°",
	Sad:     "T_T",
	Sick:    "@_@",
	Sleepy:  "-_-",
	Ghost:   "x_x",
}

// Body draws the mascot. blink closes its eyes for one frame.
func Body(st Stage, m Mood, blink bool) []string {
	art := bodies[st]
	if m == Ghost {
		art = ghostBody
	}
	f := faces[m]
	if blink && m != Sleepy && m != Ghost {
		f = "-_-"
	}
	out := make([]string, len(art))
	for i, l := range art {
		out[i] = strings.Replace(l, face, f, 1)
	}
	return out
}

// tallest is the height of the tallest body, so every frame takes the same
// number of rows (and redrawing in place doesn't leave junk behind).
func tallest() int {
	h := len(ghostBody)
	for _, b := range bodies {
		h = max(h, len(b))
	}
	return h
}

func width(lines []string) int {
	w := 0
	for _, l := range lines {
		w = max(w, utf8.RuneCountInString(l))
	}
	return w
}

// wrap breaks text into lines of at most w runes, at spaces.
func wrap(text string, w int) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		switch {
		case line == "":
			line = word
		case utf8.RuneCountInString(line)+1+utf8.RuneCountInString(word) <= w:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
