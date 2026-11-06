package mascot

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/tomassilva68725/gh-recess/internal/gitlog"
)

// Midday, so "a few hours ago" never crosses into the late-night window.
var now = time.Date(2026, 11, 6, 14, 0, 0, 0, time.UTC)

func at(who string, ago time.Duration) gitlog.Commit {
	return gitlog.Commit{Hash: fmt.Sprint(who, ago), Author: who, When: now.Add(-ago)}
}

// many makes n commits by who, spread one hour apart starting ago in the past.
func many(who string, n int, ago time.Duration) []gitlog.Commit {
	var cs []gitlog.Commit
	for i := range n {
		cs = append(cs, at(who, ago+time.Duration(i)*time.Hour))
	}
	return cs
}

func assess(commits []gitlog.Commit, unmerged ...gitlog.Branch) Pet {
	return Assess(Input{Name: "Bit", Commits: commits, Unmerged: unmerged, Now: now})
}

func TestStageFor(t *testing.T) {
	cases := map[int]Stage{0: Egg, 4: Egg, 5: Baby, 24: Baby, 25: Kid, 74: Kid, 75: Teen, 199: Teen, 200: Adult, 5000: Adult}
	for n, want := range cases {
		if got := StageFor(n); got != want {
			t.Errorf("StageFor(%d) = %v, want %v", n, got, want)
		}
	}
}

func TestMoods(t *testing.T) {
	// A healthy week for a team of two, last commit 2 hours ago.
	healthy := append(many("Ana", 3, 2*time.Hour), many("Rui", 3, 30*time.Hour)...)

	cases := []struct {
		name     string
		commits  []gitlog.Commit
		unmerged []gitlog.Branch
		want     Mood
		says     string
	}{
		{"never committed", nil, nil, Ghost, "haven't even hatched"},
		{"abandoned", many("Ana", 10, 9*24*time.Hour), nil, Ghost, "ghost"},
		{"starving", many("Ana", 10, 60*time.Hour), nil, Hungry, "staaarving"},
		{"stale branches", healthy, []gitlog.Branch{
			{Name: "City", Last: now.Add(-10 * 24 * time.Hour)},
			{Name: "Rules", Last: now.Add(-20 * 24 * time.Hour)},
			{Name: "Kitchen", Last: now.Add(-time.Hour)}, // fresh, doesn't count
			{Name: "old", Last: now.Add(-30 * 24 * time.Hour)},
		}, Sick, "City, Rules, old"},
		{"late nights", append(healthy,
			gitlog.Commit{Hash: "l1", Author: "Rui", When: time.Date(2026, 11, 6, 3, 12, 0, 0, time.UTC)},
			gitlog.Commit{Hash: "l2", Author: "Rui", When: time.Date(2026, 11, 5, 1, 0, 0, 0, time.UTC)},
			gitlog.Commit{Hash: "l3", Author: "Ana", When: time.Date(2026, 11, 4, 2, 0, 0, 0, time.UTC)},
		), nil, Sleepy, "yawn"},
		{"lonely", append(many("Ana", 3, 2*time.Hour), many("Rui", 1, 20*24*time.Hour)...), nil, Sad, "Only 1 person"},
		{"busy day", many("Ana", 6, time.Hour), nil, Excited, "6 commits today"},
		{"all good", healthy, nil, Happy, "so happy"},
	}
	for _, c := range cases {
		p := assess(c.commits, c.unmerged...)
		if p.Mood != c.want {
			t.Errorf("%s: mood = %v, want %v (stats %+v)", c.name, p.Mood, c.want, p.Stats)
		}
		if !strings.Contains(p.Speech, c.says) {
			t.Errorf("%s: speech %q should mention %q", c.name, p.Speech, c.says)
		}
	}
}

func TestStats(t *testing.T) {
	p := assess(append(many("Ana", 30, 2*time.Hour), at("dependabot[bot]", time.Minute)))
	if p.Eaten != 30 {
		t.Errorf("bots don't feed it: eaten %d", p.Eaten)
	}
	if p.Stage != Kid || p.Next != Teen || p.ToGo != 45 {
		t.Errorf("stage %v → %v in %d", p.Stage, p.Next, p.ToGo)
	}
	if p.Stats.Food != 100 || p.Stats.Health != 100 {
		t.Errorf("stats = %+v", p.Stats)
	}
	if !p.Born.Equal(now.Add(-31 * time.Hour)) {
		t.Errorf("born %v", p.Born)
	}

	// Food drains between 6 hours and 3 days after the last commit.
	if f := assess(many("Ana", 1, 39*time.Hour)).Stats.Food; f < 45 || f > 55 {
		t.Errorf("halfway food = %d", f)
	}
	if f := assess(many("Ana", 1, 80*time.Hour)).Stats.Food; f != 0 {
		t.Errorf("starved food = %d", f)
	}

	adult := assess(many("Ana", 250, time.Hour))
	if adult.Stage != Adult || adult.ToGo != 0 {
		t.Errorf("adult: %v, %d to go", adult.Stage, adult.ToGo)
	}
}

func TestCaretakers(t *testing.T) {
	cs := append(many("Ana", 2, time.Hour), many("Rui Costa", 5, time.Hour)...)
	cs = append(cs, many("Old", 9, 10*24*time.Hour)...)
	p := assess(cs)
	if len(p.Caretakers) != 2 || p.Caretakers[0].Name != "Rui Costa" || p.Caretakers[0].Commits != 5 {
		t.Errorf("caretakers = %+v", p.Caretakers)
	}
}

func TestBodiesAreStraight(t *testing.T) {
	for st := Egg; st <= Adult; st++ {
		for m := Content; m <= Ghost; m++ {
			for _, blink := range []bool{false, true} {
				body := Body(st, m, blink)
				w := utf8.RuneCountInString(body[0])
				for i, l := range body {
					if utf8.RuneCountInString(l) != w {
						t.Errorf("%v/%v row %d is crooked: %q", st, m, i, l)
					}
					if strings.Contains(l, face) {
						t.Errorf("%v/%v still has the face placeholder", st, m)
					}
				}
			}
		}
	}
	if !strings.Contains(strings.Join(Body(Kid, Happy, true), ""), "-_-") {
		t.Error("blinking closes the eyes")
	}
	if !strings.Contains(strings.Join(Body(Teen, Ghost, false), ""), "x_x") {
		t.Error("ghosts look like ghosts")
	}
}

func TestSceneKeepsItsHeight(t *testing.T) {
	p := assess(many("Ana", 30, time.Hour))
	a, b := scene(p, true, false), scene(p, false, true)
	if len(a) != len(b) {
		t.Errorf("frames have %d and %d rows: redrawing in place would leave junk", len(a), len(b))
	}
}

func TestWrap(t *testing.T) {
	got := wrap("the quick brown fox jumps over the lazy dog", 10)
	want := []string{"the quick", "brown fox", "jumps over", "the lazy", "dog"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("wrap = %q", got)
	}
}

func TestShow(t *testing.T) {
	var out bytes.Buffer
	p := assess(append(many("Ana", 6, time.Hour), many("Rui", 2, 30*time.Hour)...))
	Show(&out, p, now, false)
	s := out.String()
	for _, want := range []string{"Bit · baby", "8 commits eaten", "grows into a kid in 17 commits",
		"food", "energy", "CARETAKERS THIS WEEK", "Ana", "*o*", "mascot name"} {
		if !strings.Contains(s, want) {
			t.Errorf("screen is missing %q:\n%s", want, s)
		}
	}
}

func TestNames(t *testing.T) {
	if DefaultName("me/repo") != DefaultName("ME/REPO") {
		t.Error("the default name should only depend on the repo")
	}
	dir := t.TempDir()
	if LoadName(dir) != "" {
		t.Error("no name saved yet")
	}
	if err := SaveName(dir, "  Bolinhas "); err != nil {
		t.Fatal(err)
	}
	if got := LoadName(dir); got != "Bolinhas" {
		t.Errorf("LoadName = %q", got)
	}
	if SaveName(dir, "") == nil || SaveName(dir, strings.Repeat("x", MaxName+1)) == nil {
		t.Error("empty and too-long names should be refused")
	}
}
