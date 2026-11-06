package graveyard

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/tomassilva68725/gh-recess/internal/render"
)

// Logo is the big GRAVEYARD: pale stone fading into the dark.
var Logo = render.Logo{
	Text:     "graveyard",
	Gradient: []int{255, 252, 249, 246, 243, 240},
	Shadow:   238,
}

const (
	width       = 75 // the width of the GRAVEYARD logo
	perRow      = 3  // small tombstones side by side
	recentCount = 6  // tombstones in the overview
	listLimit   = 40 // rows `list` prints before asking to narrow it down

	// LastLines is how many lines of a dead file Visit shows.
	LastLines = 10
)

// Show draws the graveyard's screens.
type Show struct {
	Out     io.Writer
	Animate bool
	Yard    *Yard
	short   map[string]string
}

// NewShow prepares the screens for a yard.
func NewShow(y *Yard, out io.Writer, animate bool) *Show {
	var names []string
	for _, l := range y.Dead {
		names = append(names, l.Creator, l.Killer)
	}
	return &Show{Out: out, Animate: animate, Yard: y, short: render.ShortNames(names)}
}

// name is a person's short display name: "André" for "André Narquel".
func (s *Show) name(full string) string {
	if n, ok := s.short[full]; ok {
		return n
	}
	return full
}

func (s *Show) printf(format string, args ...any) { fmt.Fprintf(s.Out, format, args...) }

func (s *Show) section(label string) {
	s.printf("\n  %s\n\n", render.Color256(stoneColor, render.Title(label, width)))
}

// Overview is the graveyard's front gate: stats, the newest tombstones, the
// hall of records and the killers.
func (s *Show) Overview() {
	s.printf("\n")
	var delay time.Duration
	if s.Animate {
		delay = 45 * time.Millisecond
	}
	Logo.Draw(s.Out, delay)

	st := s.Yard.Stats()
	if st.Files == 0 {
		s.printf("\n  🌱 %s\n", render.Bold("Nobody has deleted a file yet. Suspiciously peaceful."))
		return
	}
	line := "Here lie " + render.Bold(render.Plural(st.Files, "file"))
	if st.Lines > 0 {
		line += render.Dim(" · ") + render.Number(st.Lines) + " lines buried"
	}
	if st.CameBack > 0 {
		line += render.Dim(" · ") + render.Yellow(strconv.Itoa(st.CameBack)+" came back from the dead")
	}
	s.printf("\n  🪦 %s\n", line)

	s.section("RECENTLY DEPARTED")
	graves := s.Yard.Recent(recentCount)
	for i := 0; i < len(graves); i += perRow {
		var stones [][]string
		for _, g := range graves[i:min(i+perRow, len(graves))] {
			stones = append(stones, smallStone(g, s.name))
		}
		for _, l := range row(stones) {
			s.printf("  %s\n", l)
		}
		s.printf("\n")
	}

	if recs := s.Yard.Records(); len(recs) > 0 {
		s.section("HALL OF RECORDS")
		for _, r := range recs {
			s.printf("   %s %s %s\n", render.Dim(render.Pad(r.Label, 17)),
				render.Pad(render.Cyan(render.Trunc(recordName(r), 28)), 28), render.Dim(s.recordDetail(r)))
		}
	}

	s.section("KILLERS")
	s.killers(5)

	s.printf("\n  %s %s %s %s %s %s\n",
		render.Dim("visit a grave:"), render.Bold("graveyard visit <name>"),
		render.Dim("· every grave:"), render.Bold("graveyard list"),
		render.Dim("·"), render.Bold("graveyard killers"))
}

func recordName(r Record) string {
	if r.Label == "biggest massacre" {
		return render.Plural(r.Count, "file")
	}
	return r.Life.Name()
}

func (s *Show) recordDetail(r Record) string {
	l := r.Life
	switch r.Label {
	case "longest life", "shortest life":
		a, _ := l.Age()
		return "lived " + render.Age(a)
	case "heaviest coffin":
		return render.Plural(l.Lines, "line")
	case "biggest massacre":
		return render.Date(l.Died) + " · by " + s.name(l.Killer)
	case "zombie":
		return fmt.Sprintf("died %d times", r.Count)
	}
	return ""
}

// KillersTable is the full ranking of who deleted what.
func (s *Show) KillersTable() {
	s.section("KILLERS")
	s.killers(0)
}

func (s *Show) killers(limit int) {
	list := s.Yard.Killers()
	if len(list) == 0 {
		s.printf("   %s\n", render.Dim("Nobody. Yet."))
		return
	}
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	nameW := 6
	for _, k := range list {
		nameW = max(nameW, render.Width(s.name(k.Name)))
	}
	top := float64(list[0].Files)
	for i, k := range list {
		s.printf("   %s  %s  %s  %s\n",
			render.Dim(strconv.Itoa(i+1)+"."),
			render.Red(render.Pad(s.name(k.Name), nameW)),
			render.Pad(render.Color256(stoneColor, render.Bar(float64(k.Files), top, 16)), 16),
			render.Dim(fmt.Sprintf("%s · %s lines · %s", render.Plural(k.Files, "file"),
				render.Number(k.Lines), render.Plural(k.Commits, "spree"))))
	}
}

// List prints every grave whose path contains filter, newest first.
func (s *Show) List(filter string) {
	lives := s.Yard.Filter(filter)
	title := "EVERY GRAVE"
	if filter != "" {
		title = fmt.Sprintf("GRAVES MATCHING %q", filter)
	}
	s.section(title)
	if len(lives) == 0 {
		s.printf("   %s\n", render.Dim("Nothing buried here."))
		return
	}
	s.printf("   %s  %s  %s  %s  %s\n", render.Dim(render.Pad("died", 7)), render.Dim(render.Pad("file", 40)),
		render.Dim(render.Pad("lived", 10)), render.Dim(render.Pad("size", 11)), render.Dim("killed by"))
	for _, l := range lives[:min(len(lives), listLimit)] {
		lived := "?"
		if a, ok := l.Age(); ok {
			lived = render.Age(a)
		}
		size := render.Plural(l.Lines, "line")
		if l.Binary {
			size = "binary"
		}
		s.printf("   %s  %s  %s  %s  %s\n", render.Pad(l.Died.Format("Jan 2"), 7),
			render.Cyan(render.Pad(tail(l.Path, 40), 40)), render.Pad(lived, 10), render.Pad(size, 11),
			render.Red(s.name(l.Killer)))
	}
	if extra := len(lives) - listLimit; extra > 0 {
		s.printf("\n   %s\n", render.Dim(fmt.Sprintf("+ %s more · narrow it down: graveyard list <part of a path>",
			render.Number(extra))))
	}
}

// Choices is shown when a visit matches several graves.
func (s *Show) Choices(query string, lives []Life) {
	s.printf("\n  %s\n\n", render.Bold(fmt.Sprintf("%s match %q. Which one?", render.Plural(len(lives), "grave"), query)))
	for _, l := range lives[:min(len(lives), 12)] {
		s.printf("   %s %s\n", render.Cyan(render.Pad(tail(l.Path, 56), 56)), render.Dim("died "+render.Date(l.Died)))
	}
	if extra := len(lives) - 12; extra > 0 {
		s.printf("   %s\n", render.Dim(fmt.Sprintf("+ %d more", extra)))
	}
	s.printf("\n  %s\n", render.Dim("Type more of the path: graveyard visit <name>"))
}

// Visit draws one big tombstone, the file's last lines and how to bring it
// back. lfs is the real size of a Git LFS file, or -1 when it isn't one.
func (s *Show) Visit(l Life, code []string, lfs int64) {
	s.printf("\n")
	for _, line := range bigStone(l, s.name) {
		s.printf("  %s\n", line)
	}

	s.printf("\n  %s\n", render.Bold("Its last lines:"))
	switch {
	case lfs >= 0:
		s.printf("  %s\n", render.Dim("A binary soul kept in Git LFS ("+render.Bytes(lfs)+"): there's nothing to read."))
	case l.Binary:
		s.printf("  %s\n", render.Dim("A binary soul: there's nothing to read."))
	case len(code) == 0:
		s.printf("  %s\n", render.Dim("It left nothing behind (the file was empty)."))
	default:
		s.printf("  %s\n", render.Dim("┌─"))
		for _, c := range code {
			s.printf("  %s %s\n", render.Dim("│"), c)
		}
		s.printf("  %s\n", render.Dim("└─"))
	}

	p := l.Path
	if strings.ContainsAny(p, " '\"") {
		p = `"` + p + `"`
	}
	short := l.KillHash
	if len(short) > 7 {
		short = short[:7]
	}
	s.printf("\n  %s %s\n", render.Dim("🧟 bring it back:"), render.Bold("git checkout "+short+"^ -- "+p))
}

// FirstLines picks the first n lines of a file worth showing, tidied up.
func FirstLines(content string, n int) []string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	lines = lines[:min(n, len(lines))]
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return nil
	}
	return render.TidyCode(lines)
}
