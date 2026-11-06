package graveyard

import (
	"bytes"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/tomassilva68725/gh-recess/internal/gitlog"
)

var day0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func ev(hash, who string, days int, subject string, changes ...gitlog.Change) gitlog.Event {
	return gitlog.Event{Hash: hash, Author: who, When: day0.AddDate(0, 0, days), Subject: subject, Changes: changes}
}

func add(p string) gitlog.Change       { return gitlog.Change{Status: 'A', Path: p} }
func del(p string) gitlog.Change       { return gitlog.Change{Status: 'D', Path: p} }
func mv(from, to string) gitlog.Change { return gitlog.Change{Status: 'R', From: from, Path: to} }

// A small story: Pan.h is born, moved, killed, brought back and killed again;
// Old.cpp existed before the history; a cleanup kills five files at once.
func story() *Yard {
	events := []gitlog.Event{
		ev("h1", "João Rodrigues", 0, "pan", add("src/Pan.h")),
		ev("h2", "André Narquel", 2, "move pan", mv("src/Pan.h", "Kitchen/Pan.h")),
		ev("h3", "tomas", 4, "pan is useless", del("Kitchen/Pan.h"), del("Old.cpp")),
		ev("h4", "João Rodrigues", 5, "pan is back", add("Kitchen/Pan.h")),
		ev("h5", "André Narquel", 5, "make tmp", add("tmp/a.txt"), add("tmp/b.txt"), add("tmp/c.txt"),
			add("tmp/d.txt"), add("tmp/e.png")),
		ev("h6", "tomas", 9, "cleanup", del("tmp/a.txt"), del("tmp/b.txt"), del("tmp/c.txt"),
			del("tmp/d.txt"), del("tmp/e.png")),
		ev("h7", "André Narquel", 30, "pan again", del("Kitchen/Pan.h")),
	}
	sizes := map[string]gitlog.Deletion{
		"h3:Kitchen/Pan.h": {Lines: 40},
		"h3:Old.cpp":       {Lines: 900},
		"h6:tmp/e.png":     {Binary: true},
		"h7:Kitchen/Pan.h": {Lines: 55},
	}
	return Build(events, sizes)
}

func TestBuild(t *testing.T) {
	y := story()
	if len(y.Dead) != 8 {
		t.Fatalf("%d lives ended, want 8", len(y.Dead))
	}
	newest := y.Dead[0]
	if newest.Path != "Kitchen/Pan.h" || newest.Killer != "André Narquel" || newest.Lines != 55 {
		t.Errorf("newest death: %+v", newest)
	}
	if newest.Creator != "João Rodrigues" || !newest.Born.Equal(day0.AddDate(0, 0, 5)) {
		t.Errorf("second life should start at the resurrection: %+v", newest)
	}

	var firstPan, old Life
	for _, l := range y.Dead {
		if l.KillHash == "h3" && l.Path == "Kitchen/Pan.h" {
			firstPan = l
		}
		if l.Path == "Old.cpp" {
			old = l
		}
	}
	if firstPan.Creator != "João Rodrigues" || !firstPan.Born.Equal(day0) {
		t.Errorf("a move keeps the birth: %+v", firstPan)
	}
	if !firstPan.CameBack {
		t.Error("Pan.h came back after its first death")
	}
	if age, ok := firstPan.Age(); !ok || age != 4*24*time.Hour {
		t.Errorf("first Pan.h lived %v", age)
	}
	if _, ok := old.Age(); ok || old.Creator != "" {
		t.Errorf("Old.cpp was born before the history: %+v", old)
	}
	if old.MassSize != 2 || old.LastWords != "pan is useless" {
		t.Errorf("Old.cpp: %+v", old)
	}
}

func TestRecentGroupsMassGraves(t *testing.T) {
	graves := story().Recent(6)
	// newest first: Pan.h (h7) alone, then the h6 cleanup as one mass grave,
	// then the two h3 deaths separately (2 < massGrave).
	if len(graves) != 4 {
		t.Fatalf("%d graves, want 4", len(graves))
	}
	if graves[0].Mass() || !graves[1].Mass() || len(graves[1].Lives) != 5 || graves[2].Mass() {
		t.Errorf("grouping is wrong: %d/%d/%d", len(graves[0].Lives), len(graves[1].Lives), len(graves[2].Lives))
	}
	if got := story().Recent(2); len(got) != 2 {
		t.Errorf("Recent(2) gave %d", len(got))
	}
}

func TestStatsKillersRecords(t *testing.T) {
	y := story()
	st := y.Stats()
	if st.Files != 8 || st.Lines != 995 || st.CameBack != 1 || st.Killers != 2 {
		t.Errorf("stats = %+v", st)
	}

	ks := y.Killers()
	if ks[0].Name != "tomas" || ks[0].Files != 7 || ks[0].Commits != 2 {
		t.Errorf("top killer = %+v", ks[0])
	}

	recs := map[string]Record{}
	for _, r := range y.Records() {
		recs[r.Label] = r
	}
	if r := recs["heaviest coffin"]; r.Life.Path != "Old.cpp" {
		t.Errorf("heaviest = %s", r.Life.Path)
	}
	if r := recs["biggest massacre"]; r.Count != 5 {
		t.Errorf("massacre = %d files", r.Count)
	}
	if r := recs["zombie"]; r.Life.Path != "Kitchen/Pan.h" || r.Count != 2 {
		t.Errorf("zombie = %s × %d", r.Life.Path, r.Count)
	}
	if r := recs["longest life"]; r.Life.Path != "Kitchen/Pan.h" || r.Life.KillHash != "h7" {
		t.Errorf("longest = %s (%s)", r.Life.Path, r.Life.KillHash)
	}
}

func TestFind(t *testing.T) {
	y := story()
	if got := y.Find("pan.h"); len(got) != 1 || got[0].KillHash != "h7" {
		t.Errorf("pan.h should find one grave, the newest death: %+v", got)
	}
	if got := y.Find("tmp"); len(got) != 5 {
		t.Errorf("tmp matches %d", len(got))
	}
	if got := y.Find("a.t"); len(got) != 1 || got[0].Path != "tmp/a.txt" {
		t.Errorf("name prefix: %+v", got)
	}
	if got := y.Find(`Kitchen\Pan.h`); len(got) != 1 {
		t.Error("Windows slashes should work")
	}
	if got := y.Find("nothing"); got != nil {
		t.Errorf("got %+v", got)
	}
}

// Every row of a tombstone must be as wide as the others, or they wobble.
func TestStonesAreStraight(t *testing.T) {
	y := story()
	show := NewShow(y, &bytes.Buffer{}, false)
	check := func(name string, lines []string) {
		w := utf8.RuneCountInString(lines[0])
		for i, l := range lines {
			if n := utf8.RuneCountInString(l); n != w {
				t.Errorf("%s row %d is %d wide, row 0 is %d: %q", name, i, n, w, l)
			}
		}
	}
	for _, g := range y.Recent(6) {
		check("small", smallStone(g, show.name))
	}
	check("big", bigStone(y.Dead[0], show.name))
	check("row", row([][]string{smallStone(y.Recent(1)[0], show.name), smallStone(y.Recent(1)[0], show.name)}))
}

func TestScreens(t *testing.T) {
	var out bytes.Buffer
	show := NewShow(story(), &out, false)
	show.Overview()
	show.List("")
	show.KillersTable()
	show.Choices("tmp", story().Find("tmp"))
	y := story()
	show.Visit(y.Dead[0], []string{"#pragma once", "struct FPan {};"}, -1)
	for _, l := range y.Dead {
		if l.CameBack {
			show.Visit(l, nil, -1) // the first Pan.h, which came back
		}
	}
	s := out.String()
	for _, want := range []string{
		"Here lie 8 files", "RECENTLY DEPARTED", "5 files", "mass grave", "HALL OF RECORDS",
		"zombie", "KILLERS", "EVERY GRAVE", "5 graves match", "Its last lines:",
		"git checkout h7^ -- Kitchen/Pan.h", "rose from the dead", "pan again",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("screens are missing %q", want)
		}
	}
}

func TestLFSAndBinaryAssets(t *testing.T) {
	y := Build([]gitlog.Event{
		ev("h1", "a", 0, "add", add("Content/Map.umap"), add("x.cpp")),
		ev("h2", "b", 1, "remove", del("Content/Map.umap"), del("x.cpp")),
	}, map[string]gitlog.Deletion{"h2:Content/Map.umap": {Lines: 3}, "h2:x.cpp": {Lines: 3}})
	for _, l := range y.Dead {
		if l.Path == "Content/Map.umap" && (!l.Binary || l.Lines != 0) {
			t.Errorf("an LFS pointer counted as text: %+v", l)
		}
		if l.Path == "x.cpp" && (l.Binary || l.Lines != 3) {
			t.Errorf("code counted as binary: %+v", l)
		}
	}

	ptr := "version https://git-lfs.github.com/spec/v1\noid sha256:abc\nsize 11849\n"
	if n, ok := LFSSize(ptr); !ok || n != 11849 {
		t.Errorf("LFSSize = %d, %v", n, ok)
	}
	if _, ok := LFSSize("int main() {}"); ok {
		t.Error("plain code isn't an LFS pointer")
	}

	var out bytes.Buffer
	NewShow(y, &out, false).Visit(y.Dead[0], nil, 11849)
	if !strings.Contains(out.String(), "Git LFS (11.6 KB)") {
		t.Errorf("visit should show the LFS size:\n%s", out.String())
	}
}

func TestEmptyYard(t *testing.T) {
	var out bytes.Buffer
	NewShow(Build(nil, nil), &out, false).Overview()
	if !strings.Contains(out.String(), "Suspiciously peaceful") {
		t.Error("an empty graveyard should say so")
	}
}

func TestFirstLines(t *testing.T) {
	got := FirstLines("\r\n\n\tint a;\r\n\tint b;\r\n\n\n", 10)
	if len(got) != 2 || got[0] != "int a;" {
		t.Errorf("got %q", got)
	}
	if FirstLines("\n\n", 10) != nil {
		t.Error("an empty file has no lines")
	}
}

func TestDateRange(t *testing.T) {
	now := time.Now().Year()
	cases := []struct {
		born, died time.Time
		want       string
	}{
		{time.Time{}, time.Date(now, 11, 4, 0, 0, 0, 0, time.UTC), "? – Nov 4"},
		{time.Date(now, 10, 31, 0, 0, 0, 0, time.UTC), time.Date(now, 11, 4, 0, 0, 0, 0, time.UTC), "Oct 31 – Nov 4"},
		{time.Date(2020, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC), "Mar – Jun 2020"},
		{time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2021, 6, 1, 0, 0, 0, 0, time.UTC), "2019 – 2021"},
	}
	for _, c := range cases {
		if got := dateRange(c.born, c.died); got != c.want {
			t.Errorf("dateRange = %q, want %q", got, c.want)
		}
	}
}
