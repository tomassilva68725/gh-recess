package aquarium

import (
	"bytes"
	"math/rand/v2"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/tomassilva68725/gh-recess/internal/gitlog"
)

var now = time.Date(2026, 11, 6, 18, 0, 0, 0, time.UTC)

func clock() time.Time { return now }

func commitsBy(name string, n int, ago time.Duration) []gitlog.Commit {
	var cs []gitlog.Commit
	for i := range n {
		cs = append(cs, gitlog.Commit{Hash: name + string(rune('a'+i%26)) + strings.Repeat("x", i/26),
			Author: name, When: now.Add(-ago - time.Duration(i)*time.Minute), Subject: "work"})
	}
	return cs
}

func team() []gitlog.Commit {
	var cs []gitlog.Commit
	cs = append(cs, commitsBy("tomassilva68725", 33, 2*time.Hour)...)  // lively whale
	cs = append(cs, commitsBy("André Narquel", 12, 28*time.Hour)...)   // awake big fish
	cs = append(cs, commitsBy("João Rodrigues", 1, 5*24*time.Hour)...) // asleep fry
	cs = append(cs, commitsBy("Old Timer", 40, 60*24*time.Hour)...)    // outside the window
	cs = append(cs, commitsBy("dependabot[bot]", 5, time.Hour)...)
	return cs
}

func TestMembers(t *testing.T) {
	ms := Members(team(), now, Window)
	if len(ms) != 4 {
		t.Fatalf("%d members (bots should be out): %+v", len(ms), ms)
	}
	if ms[0].Name != "tomassilva68725" || ms[0].Recent != 33 {
		t.Errorf("first = %+v", ms[0])
	}
	for _, m := range ms {
		if m.Name == "Old Timer" && (m.Recent != 0 || m.Total != 40) {
			t.Errorf("old timer = %+v", m)
		}
	}
}

func TestTierFor(t *testing.T) {
	cases := map[int]Tier{0: Fry, 2: Fry, 3: Small, 9: Small, 10: Medium, 24: Medium, 25: Large, 400: Large}
	for n, want := range cases {
		if got := TierFor(n); got != want {
			t.Errorf("TierFor(%d) = %v, want %v", n, got, want)
		}
	}
}

func TestArtMirrors(t *testing.T) {
	for tier := Fry; tier <= Large; tier++ {
		right, left := Art(tier, 1), Art(tier, -1)
		w, h := artSize(tier)
		if len(right) != h || len(left) != h {
			t.Errorf("%v: heights differ", tier)
		}
		for i := range right {
			if utf8.RuneCountInString(right[i]) != w || utf8.RuneCountInString(left[i]) != w {
				t.Errorf("%v row %d isn't %d wide: %q / %q", tier, i, w, right[i], left[i])
			}
		}
	}
	if got := Art(Small, -1)[0]; got != "<°))><" {
		t.Errorf("small fish facing left = %q", got)
	}
	if got := Art(Fry, -1)[0]; got != "<><" {
		t.Errorf("fry facing left = %q", got)
	}
}

func newTank(w, h int) *Tank {
	return NewTank(Members(team(), now, Window), w, h, rand.New(rand.NewPCG(3, 4)), clock)
}

// Fish must stay in the water, whatever happens to the window.
func TestFishStayInTheTank(t *testing.T) {
	tank := newTank(80, 24)
	sizes := [][2]int{{80, 24}, {40, 12}, {120, 40}, {30, 10}, {200, 15}}
	for step := range 2000 {
		if step%400 == 0 {
			s := sizes[step/400]
			tank.Resize(s[0], s[1])
		}
		tank.Step(0.1)
		for _, f := range tank.Fish {
			fw, fh := tank.size(f)
			if f.x < 0 || int(f.x)+fw > tank.W {
				t.Fatalf("step %d: %s at x=%.1f (w=%d) left a %d-wide tank", step, f.Label, f.x, fw, tank.W)
			}
			if f.y < topRow || int(f.y)+fh > tank.H-sandRows {
				t.Fatalf("step %d: %s at y=%.1f (h=%d) left a %d-tall tank", step, f.Label, f.y, fh, tank.H)
			}
		}
	}
}

func TestSleepersSinkAndSnore(t *testing.T) {
	tank := newTank(80, 24)
	var joao *Fish
	for _, f := range tank.Fish {
		if f.Name == "João Rodrigues" {
			joao = f
		}
	}
	if !joao.Asleep(now) {
		t.Fatal("João hasn't committed in 5 days and should be asleep")
	}
	for range 300 {
		tank.Step(0.1)
	}
	_, fh := tank.size(joao)
	if _, maxY := tank.yRange(fh); joao.y != maxY {
		t.Errorf("a sleeping fish should rest on the bottom: y=%.1f, bottom=%.1f", joao.y, maxY)
	}
	zzz := false
	for _, b := range tank.bubbles {
		zzz = zzz || b.zzz
	}
	if !zzz {
		t.Error("someone should be snoring")
	}
}

func TestCommitWakesAndGrows(t *testing.T) {
	tank := newTank(80, 24)
	c := gitlog.Commit{Hash: "new", Author: "João Rodrigues", When: now, Subject: "fry the onions"}
	f := tank.Commit(c)
	if f == nil || f.Asleep(now) || !f.Active(now) || f.burst == 0 || f.flash == 0 {
		t.Fatalf("the fish should wake up and celebrate: %+v", f)
	}
	before := len(tank.bubbles)
	for range 15 {
		tank.Step(0.1)
	}
	if len(tank.bubbles) <= before {
		t.Error("a commit should make bubbles")
	}
	for range 3 {
		tank.Commit(gitlog.Commit{Author: "João Rodrigues", When: now})
	}
	if tank.Tier(f) != Small {
		t.Errorf("5 commits should make a small fish, got %v", tank.Tier(f))
	}

	newbie := tank.Commit(gitlog.Commit{Author: "Rita Sousa", When: now})
	if newbie == nil || newbie.Label != "Rita" || len(tank.Fish) != 5 {
		t.Errorf("a newcomer should get a fish: %+v", newbie)
	}
	if tank.Commit(gitlog.Commit{Author: "renovate[bot]", When: now}) != nil {
		t.Error("bots don't get fish")
	}
}

func TestEveryoneOutsideTheWindow(t *testing.T) {
	old := commitsBy("A", 30, 90*24*time.Hour)
	old = append(old, commitsBy("B", 2, 90*24*time.Hour)...)
	tank := NewTank(Members(old, now, Window), 80, 24, rand.New(rand.NewPCG(1, 2)), clock)
	if !tank.useTotal || tank.Tier(tank.Fish[0]) != Large {
		t.Error("with no recent commits, size should come from all-time commits")
	}
}

func TestDrawAndCanvas(t *testing.T) {
	tank := newTank(70, 20)
	for range 20 {
		tank.Step(0.1)
	}
	c := tank.Draw()
	if c.W != 70 || c.H != 20 {
		t.Fatalf("canvas is %dx%d", c.W, c.H)
	}
	text := ""
	for y := range c.H {
		line := c.Line(y, false)
		if n := utf8.RuneCountInString(line); n > 70 {
			t.Errorf("row %d is %d wide", y, n)
		}
		text += line + "\n"
	}
	for _, want := range []string{"tomassilva68725", "André", "João", "°", "▒"} {
		if !strings.Contains(text, want) {
			t.Errorf("the picture is missing %q:\n%s", want, text)
		}
	}
	if c.At(0, c.H-1) != '▒' {
		t.Error("the bottom row should be sand")
	}
	colored := c.Line(5, true)
	if !strings.Contains(colored, "\x1b[48;5;") || !strings.HasSuffix(colored, "\x1b[0m") {
		t.Errorf("colored rows need a background and a reset: %q", colored)
	}
}

func TestCanvasClips(t *testing.T) {
	c := NewCanvas(5, 2)
	c.Put(-2, 0, "abcdefgh", 1)
	c.Put(0, 5, "nope", 1)
	c.Put(1, 1, "x y", 1)
	if got := c.Line(0, false); got != "cdefg" {
		t.Errorf("row 0 = %q", got)
	}
	if got := c.Line(1, false); got != " x y" {
		t.Errorf("row 1 = %q (spaces must be see-through)", got)
	}
}

func TestSnapshotAndLegend(t *testing.T) {
	var out bytes.Buffer
	Snapshot(&out, team(), now, 78, 18)
	Legend(&out, team(), now)
	s := out.String()
	for _, want := range []string{"Your fish:", "33 commits in 14 days", "asleep zzz", "lively", "~"} {
		if !strings.Contains(s, want) {
			t.Errorf("output is missing %q", want)
		}
	}
	if strings.Contains(s, "dependabot") {
		t.Error("bots don't get fish")
	}
}

func TestStatusLines(t *testing.T) {
	tank := newTank(80, 24)
	l1, l2 := statusLines(tank, "me/repo", "", false, "")
	if !strings.Contains(l1, "me/repo · 4 fish") || !strings.Contains(l1, "by tomassilva68725") {
		t.Errorf("l1 = %q", l1)
	}
	if !strings.Contains(l2, "--fetch") {
		t.Errorf("l2 should suggest --fetch: %q", l2)
	}
	_, l2 = statusLines(tank, "me/repo", "fetched 18:00", true, "João committed: “x”")
	if !strings.Contains(l2, "João committed") {
		t.Errorf("headline missing: %q", l2)
	}
}
