package aquarium

import (
	"fmt"
	"strings"
)

type cell struct {
	r  rune
	fg int // 256-color code, or -1 for the terminal's default
}

// Canvas is a grid of characters with a color each, plus one background
// color per row. Draw on it, then turn rows into text with Line.
type Canvas struct {
	W, H  int
	cells []cell
	bg    []int
}

// NewCanvas is an empty w×h canvas.
func NewCanvas(w, h int) *Canvas {
	c := &Canvas{W: w, H: h, cells: make([]cell, w*h), bg: make([]int, h)}
	for i := range c.cells {
		c.cells[i] = cell{' ', -1}
	}
	for i := range c.bg {
		c.bg[i] = -1
	}
	return c
}

// Put writes s at (x, y). Spaces are see-through and anything outside the
// canvas is clipped.
func (c *Canvas) Put(x, y int, s string, fg int) {
	if y < 0 || y >= c.H {
		return
	}
	for _, r := range s {
		if r != ' ' && x >= 0 && x < c.W {
			c.cells[y*c.W+x] = cell{r, fg}
		}
		x++
	}
}

// SetBG colors the background of a whole row.
func (c *Canvas) SetBG(y, color int) {
	if y >= 0 && y < c.H {
		c.bg[y] = color
	}
}

// At returns the character at (x, y), for tests.
func (c *Canvas) At(x, y int) rune {
	if x < 0 || y < 0 || x >= c.W || y >= c.H {
		return 0
	}
	return c.cells[y*c.W+x].r
}

// Line turns row y into text, with color codes when color is true.
func (c *Canvas) Line(y int, color bool) string {
	row := c.cells[y*c.W : (y+1)*c.W]
	var b strings.Builder
	if !color {
		for _, cl := range row {
			b.WriteRune(cl.r)
		}
		return strings.TrimRight(b.String(), " ")
	}

	b.WriteString("\x1b[0m")
	if c.bg[y] >= 0 {
		fmt.Fprintf(&b, "\x1b[48;5;%dm", c.bg[y])
	}
	cur := -2 // no color chosen yet
	for _, cl := range row {
		if cl.fg != cur && cl.r != ' ' {
			if cl.fg < 0 {
				b.WriteString("\x1b[39m")
			} else {
				fmt.Fprintf(&b, "\x1b[38;5;%dm", cl.fg)
			}
			cur = cl.fg
		}
		b.WriteRune(cl.r)
	}
	b.WriteString("\x1b[0m")
	return b.String()
}
