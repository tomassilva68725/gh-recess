package render

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// Every glyph's rows must be the same width, or the logos come out crooked.
func TestGlyphsAreRectangles(t *testing.T) {
	for r, g := range glyphs {
		if len(g) != glyphRows {
			t.Errorf("%c has %d rows, want %d", r, len(g), glyphRows)
			continue
		}
		w := utf8.RuneCountInString(g[0])
		for i, row := range g {
			if n := utf8.RuneCountInString(row); n != w {
				t.Errorf("%c row %d is %d wide, row 0 is %d", r, i, n, w)
			}
		}
	}
}

func TestBigKnowsTheLogos(t *testing.T) {
	for _, word := range []string{"RECESS", "WHODUNIT"} {
		for _, r := range word {
			if _, ok := glyphs[r]; !ok {
				t.Errorf("missing glyph %c for %s", r, word)
			}
		}
	}
	if got := utf8.RuneCountInString(Big("whodunit")[0]); got != 66 {
		t.Errorf("WHODUNIT is %d wide, the game layout expects 66", got)
	}
}

func TestTidyCode(t *testing.T) {
	got := TidyCode([]string{"\t\tfoo();", "", "  ", "\t\t\tbar();   ", "\t\t" + strings.Repeat("x", 100)})
	want := []string{"foo();", "", "    bar();", strings.Repeat("x", CodeWidth-1) + "…"}
	if !slices.Equal(got, want) {
		t.Errorf("TidyCode = %q, want %q", got, want)
	}
}

func TestBar(t *testing.T) {
	cases := []struct {
		v, m float64
		w    int
		want string
	}{
		{10, 10, 4, "████"},
		{5, 10, 4, "██"},
		{1, 10, 4, "▍"}, // 3.2/8 of a cell rounds to 3 eighths
		{0, 10, 4, ""},
		{20, 10, 4, "████"}, // capped
	}
	for _, c := range cases {
		if got := Bar(c.v, c.m, c.w); got != c.want {
			t.Errorf("Bar(%v, %v, %d) = %q, want %q", c.v, c.m, c.w, got, c.want)
		}
	}
}

func TestTextHelpers(t *testing.T) {
	ColorEnabled = true
	defer func() { ColorEnabled = false }()

	if got := Width(Red("abc")); got != 3 {
		t.Errorf("Width ignores colors: got %d", got)
	}
	if got := Pad(Red("ab"), 4); Width(got) != 4 {
		t.Errorf("Pad: width %d", Width(got))
	}
	if got := Trunc("abcdef", 4); got != "abc…" {
		t.Errorf("Trunc = %q", got)
	}
	if got := Number(1234567); got != "1,234,567" {
		t.Errorf("Number = %q", got)
	}
	if got := Number(-1200); got != "-1,200" {
		t.Errorf("Number = %q", got)
	}
	if got := Plural(1, "commit") + "/" + Plural(2, "commit"); got != "1 commit/2 commits" {
		t.Errorf("Plural = %q", got)
	}
	if got := Width(Title("CASE 1/10", 30)); got != 30 {
		t.Errorf("Title width = %d", got)
	}
}
