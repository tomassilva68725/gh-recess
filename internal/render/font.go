package render

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// glyphs is a big 6-row block font ("ANSI Shadow"). Only the letters the
// logos use are here; add more as new games arrive.
var glyphs = map[rune][]string{
	'A': {" █████╗ ", "██╔══██╗", "███████║", "██╔══██║", "██║  ██║", "╚═╝  ╚═╝"},
	'G': {" ██████╗ ", "██╔════╝ ", "██║  ███╗", "██║   ██║", "╚██████╔╝", " ╚═════╝ "},
	'V': {"██╗   ██╗", "██║   ██║", "██║   ██║", "╚██╗ ██╔╝", " ╚████╔╝ ", "  ╚═══╝  "},
	'Y': {"██╗   ██╗", "╚██╗ ██╔╝", " ╚████╔╝ ", "  ╚██╔╝  ", "   ██║   ", "   ╚═╝   "},
	'C': {" ██████╗", "██╔════╝", "██║     ", "██║     ", "╚██████╗", " ╚═════╝"},
	'D': {"██████╗ ", "██╔══██╗", "██║  ██║", "██║  ██║", "██████╔╝", "╚═════╝ "},
	'E': {"███████╗", "██╔════╝", "█████╗  ", "██╔══╝  ", "███████╗", "╚══════╝"},
	'H': {"██╗  ██╗", "██║  ██║", "███████║", "██╔══██║", "██║  ██║", "╚═╝  ╚═╝"},
	'I': {"██╗", "██║", "██║", "██║", "██║", "╚═╝"},
	'N': {"███╗   ██╗", "████╗  ██║", "██╔██╗ ██║", "██║╚██╗██║", "██║ ╚████║", "╚═╝  ╚═══╝"},
	'O': {" ██████╗ ", "██╔═══██╗", "██║   ██║", "██║   ██║", "╚██████╔╝", " ╚═════╝ "},
	'R': {"██████╗ ", "██╔══██╗", "██████╔╝", "██╔══██╗", "██║  ██║", "╚═╝  ╚═╝"},
	'S': {"███████╗", "██╔════╝", "███████╗", "╚════██║", "███████║", "╚══════╝"},
	'T': {"████████╗", "╚══██╔══╝", "   ██║   ", "   ██║   ", "   ██║   ", "   ╚═╝   "},
	'U': {"██╗   ██╗", "██║   ██║", "██║   ██║", "██║   ██║", "╚██████╔╝", " ╚═════╝ "},
	'W': {"██╗    ██╗", "██║    ██║", "██║ █╗ ██║", "██║███╗██║", "╚███╔███╔╝", " ╚══╝╚══╝ "},
}

const glyphRows = 6

// Big spells text in the block font, one string per row.
func Big(text string) []string {
	rows := make([]string, glyphRows)
	for _, r := range strings.ToUpper(text) {
		g, ok := glyphs[r]
		if !ok {
			g = []string{"   ", "   ", "   ", "   ", "   ", "   "} // space or unknown letter
		}
		for i := range rows {
			rows[i] += g[i]
		}
	}
	return rows
}

// Logo is a big word with a vertical gradient: the solid blocks take one
// color per row and the thin "shadow" lines take a darker one.
type Logo struct {
	Text     string
	Gradient []int // 256-color codes, top row first
	Shadow   int
}

// Draw prints the logo indented by two spaces. With a delay, rows appear one
// at a time, which looks like the logo sliding in.
func (l Logo) Draw(w io.Writer, delay time.Duration) {
	for i, row := range Big(l.Text) {
		fmt.Fprintln(w, "  "+paintLogoRow(row, l.Gradient[i%len(l.Gradient)], l.Shadow))
		if delay > 0 {
			time.Sleep(delay)
		}
	}
}

func paintLogoRow(row string, color, shadow int) string {
	if !ColorEnabled {
		return row
	}
	var b strings.Builder
	cur := -1 // the color currently switched on; -1 = none
	for _, r := range row {
		c := -1
		switch {
		case r == '█':
			c = color
		case r != ' ':
			c = shadow
		}
		if c != cur && c != -1 {
			fmt.Fprintf(&b, "\x1b[38;5;%dm", c)
			cur = c
		}
		b.WriteRune(r)
	}
	if cur != -1 {
		b.WriteString(reset)
	}
	return b.String()
}
