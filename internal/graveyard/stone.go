package graveyard

import (
	"strings"
	"time"

	"github.com/tomassilva68725/gh-recess/internal/render"
)

const (
	stoneColor = 250 // light gray granite
	smallInner = 16  // columns inside a small tombstone
	bigInner   = 46  // columns inside the tombstone of `visit`
	stoneGap   = 3   // spaces between tombstones in a row
)

// frame draws a tombstone around rows. Each row must already fit in inner
// columns; it's centered unless it fills them.
//
//	   ╭──────────╮
//	╭──╯  R.I.P   ╰──╮
//	│   FryingPan.h  │
func frame(inner int, rows []string) []string {
	capW := inner + 2 - 8
	stone := func(s string) string { return render.Color256(stoneColor, s) }
	out := []string{
		"   " + stone("╭"+strings.Repeat("─", capW)+"╮") + "   ",
		stone("╭──╯") + render.Bold(render.Center("R.I.P", capW)) + stone("╰──╮"),
	}
	for _, r := range rows {
		out = append(out, stone("│")+render.Pad(render.Center(r, inner), inner)+stone("│"))
	}
	return out
}

// lawn is the strip of grass the tombstones stand on.
func lawn(w int) string {
	return render.Green(strings.Repeat("▀", w))
}

// row puts tombstones side by side on one lawn.
func row(stones [][]string) []string {
	if len(stones) == 0 {
		return nil
	}
	gap := strings.Repeat(" ", stoneGap)
	var lines []string
	for i := range stones[0] {
		parts := make([]string, len(stones))
		for k, s := range stones {
			parts[k] = s[i]
		}
		lines = append(lines, strings.Join(parts, gap))
	}
	w := len(stones)*(smallInner+2) + (len(stones)-1)*stoneGap
	return append(lines, lawn(w))
}

// smallStone is the tombstone of the overview: a file, or a mass grave.
func smallStone(g Grave, short func(string) string) []string {
	l := g.Lives[0]
	fit := smallInner - 2
	var name, dates, age string
	if g.Mass() {
		name = render.Plural(len(g.Lives), "file")
		dates = render.Date(l.Died)
		age = "mass grave"
	} else {
		name = l.Name()
		dates = dateRange(l.Born, l.Died)
		age = "age unknown"
		if a, ok := l.Age(); ok {
			age = render.Age(a)
		}
	}
	return frame(smallInner, []string{
		"",
		render.Bold(render.Trunc(name, fit)),
		render.Trunc(dates, fit),
		render.Dim(render.Trunc(age, fit)),
		"",
		render.Red("† " + render.Trunc(short(l.Killer), fit-2)),
	})
}

// bigStone is the tombstone of `visit`, with everything we know.
func bigStone(l Life, short func(string) string) []string {
	field := func(label, value string) string {
		return render.Pad("   "+render.Dim(render.Pad(label, 8))+value, bigInner)
	}
	fit := bigInner - 14 // room for a value after the label

	born := render.Dim("before the history began")
	if !l.Born.IsZero() {
		born = render.Date(l.Born) + render.Dim(" · by ") + render.Green(render.Trunc(short(l.Creator), fit-16))
	}
	died := render.Date(l.Died) + render.Dim(" · by ") + render.Red(render.Trunc(short(l.Killer), fit-16))
	lived := "nobody knows how long"
	if a, ok := l.Age(); ok {
		lived = render.Age(a)
	}
	size := render.Plural(l.Lines, "line")
	if l.Binary {
		size = "a binary file"
	}

	rows := []string{
		"",
		render.Bold(tail(l.Path, bigInner-4)),
		"",
		field("born", born),
		field("died", died),
		field("lived", lived),
		field("size", size),
	}
	if l.MassSize > 1 {
		rows = append(rows, field("", render.Dim("buried with "+render.Plural(l.MassSize-1, "other"))))
	}
	if l.CameBack {
		rows = append(rows, field("", render.Yellow("later rose from the dead")))
	}
	rows = append(rows, "", render.Italic("“"+render.Trunc(l.LastWords, bigInner-6)+"”"), "")
	return append(frame(bigInner, rows), lawn(bigInner+2))
}

// dateRange fits a life in a small tombstone: "Oct 31 – Nov 4",
// "Mar – Jun 2025" or "2024 – 2026".
func dateRange(born, died time.Time) string {
	now := time.Now().Year()
	switch {
	case born.IsZero():
		return "? – " + render.Date(died)
	case born.Year() == now && died.Year() == now:
		return born.Format("Jan 2") + " – " + died.Format("Jan 2")
	case born.Year() == died.Year():
		return born.Format("Jan") + " – " + died.Format("Jan 2006")
	default:
		return born.Format("2006") + " – " + died.Format("2006")
	}
}

// tail keeps the end of a long path, where the file name is:
// "…/Kitchen/FryingPan.cpp".
func tail(p string, w int) string {
	r := []rune(p)
	if len(r) <= w {
		return p
	}
	return "…" + string(r[len(r)-w+1:])
}
