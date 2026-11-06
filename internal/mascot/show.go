package mascot

import (
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tomassilva68725/gh-recess/internal/render"
)

// Logo is the big MASCOT: warm pinks.
var Logo = render.Logo{
	Text:     "mascot",
	Gradient: []int{225, 219, 213, 207, 201, 165},
	Shadow:   96,
}

const (
	bubbleWidth = 36 // text columns inside the speech bubble
	barWidth    = 20
	ruleWidth   = 52 // the MASCOT logo is 51 wide
)

// moodColor tints the mascot by how it feels.
var moodColor = map[Mood]int{
	Content: 223, Happy: 218, Excited: 213, Hungry: 215,
	Sad: 117, Sick: 113, Sleepy: 146, Ghost: 252,
}

// scene puts the mascot (left) and its speech bubble (right) side by side.
// up lifts the mascot by one row, for the bounce.
func scene(p Pet, up, blink bool) []string {
	body := Body(p.Stage, p.Mood, blink)
	h := tallest() + 1 // one spare row to bounce into
	bw := width(body)

	// The body sits on the bottom row, or one higher when bouncing.
	left := make([]string, h)
	start := h - len(body)
	if up {
		start--
	}
	for i := range left {
		left[i] = strings.Repeat(" ", bw)
		if j := i - start; j >= 0 && j < len(body) {
			left[i] = render.Color256(moodColor[p.Mood], body[j])
		}
	}

	text := wrap(p.Speech, bubbleWidth)
	bubble := []string{render.Dim("╭" + strings.Repeat("─", bubbleWidth+2) + "╮")}
	for _, t := range text {
		bubble = append(bubble, render.Dim("│ ")+render.Pad(t, bubbleWidth)+render.Dim(" │"))
	}
	bubble = append(bubble, render.Dim("╰"+strings.Repeat("─", bubbleWidth+2)+"╯"))

	// Center the bubble on the mascot, with a little tail pointing at it.
	top := max((h-len(bubble))/2, 0)
	tailRow := top + len(bubble)/2
	rows := max(h, top+len(bubble))
	out := make([]string, rows)
	for i := range out {
		l := strings.Repeat(" ", bw)
		if i < len(left) {
			l = left[i]
		}
		gap := "    "
		if i == tailRow {
			gap = render.Dim("  ◀─")
		}
		r := ""
		if j := i - top; j >= 0 && j < len(bubble) {
			r = bubble[j]
		} else {
			gap = ""
		}
		out[i] = strings.TrimRight("  "+l+gap+r, " ")
	}
	return out
}

// Show prints the mascot's screen. With animate, it bounces and blinks for
// a couple of seconds before settling.
func Show(w io.Writer, p Pet, now time.Time, animate bool) {
	fmt.Fprintln(w)
	var delay time.Duration
	if animate {
		delay = 45 * time.Millisecond
	}
	Logo.Draw(w, delay)
	fmt.Fprintln(w)

	if animate {
		frames := []struct{ up, blink bool }{
			{true, false}, {false, false}, {true, false}, {false, false},
			{false, true}, {false, false}, {true, false}, {false, false},
		}
		for i, f := range frames {
			lines := scene(p, f.up, f.blink)
			if i > 0 {
				fmt.Fprintf(w, "\x1b[%dA", len(lines)) // back up to redraw in place
			}
			for _, l := range lines {
				fmt.Fprintf(w, "\r\x1b[K%s\n", l)
			}
			time.Sleep(230 * time.Millisecond)
		}
	} else {
		for _, l := range scene(p, false, false) {
			fmt.Fprintln(w, l)
		}
	}

	// Name, stage, age.
	fmt.Fprintln(w)
	about := []string{render.Bold(render.Color256(moodColor[p.Mood], p.Name)), p.Stage.String()}
	if !p.Born.IsZero() {
		about = append(about, render.Age(now.Sub(p.Born))+" old")
	}
	about = append(about, render.Plural(p.Eaten, "commit")+" eaten", "feeling "+p.Mood.String())
	fmt.Fprintf(w, "  %s\n", strings.Join(about, render.Dim(" · ")))
	if p.ToGo > 0 {
		fmt.Fprintf(w, "  %s\n", render.Dim(fmt.Sprintf("grows into a %s in %s", p.Next, render.Plural(p.ToGo, "commit"))))
	} else if p.Stage == Adult {
		fmt.Fprintf(w, "  %s\n", render.Dim("all grown up 👑"))
	}

	// The four bars.
	fmt.Fprintln(w)
	bars := []struct {
		label string
		value int
		why   string
	}{
		{"food", p.Stats.Food, "commits fill it, 3 days without one empty it"},
		{"joy", p.Stats.Joy, "teammates who committed this week"},
		{"health", p.Stats.Health, "stale branches make it sick"},
		{"energy", p.Stats.Energy, "late-night commits wear it out"},
	}
	for _, b := range bars {
		fmt.Fprintf(w, "  %s %s %s  %s\n", render.Pad(b.label, 7), meter(b.value),
			render.Bold(fmt.Sprintf("%3d", b.value)), render.Dim(b.why))
	}

	// Who fed it.
	fmt.Fprintf(w, "\n  %s\n\n", render.Color256(213, render.Title("CARETAKERS THIS WEEK", ruleWidth)))
	if len(p.Caretakers) == 0 {
		fmt.Fprintf(w, "   %s\n", render.Dim("Nobody. It's been a lonely week."))
	} else {
		var names []string
		for _, c := range p.Caretakers {
			names = append(names, c.Name)
		}
		short := render.ShortNames(names)
		nameW := 4
		for _, c := range p.Caretakers {
			nameW = max(nameW, utf8.RuneCountInString(shortOr(short, c.Name)))
		}
		top := float64(p.Caretakers[0].Commits)
		for i, c := range p.Caretakers {
			heart := ""
			if i == 0 {
				heart = " " + render.Color256(205, "♥")
			}
			fmt.Fprintf(w, "   %s  %s  %s  %s%s\n", render.Dim(fmt.Sprintf("%d.", i+1)),
				render.Bold(render.Pad(shortOr(short, c.Name), nameW)),
				render.Pad(render.Color256(213, render.Bar(float64(c.Commits), top, 16)), 16),
				render.Dim(render.Plural(c.Commits, "commit")), heart)
		}
	}
	fmt.Fprintf(w, "\n  %s %s\n", render.Dim("rename it:"), render.Bold("gh recess mascot name <name>"))
}

func shortOr(short map[string]string, name string) string {
	if s, ok := short[name]; ok {
		return s
	}
	return name
}

// meter is a 0-100 bar: green when fine, yellow when worrying, red when bad.
func meter(v int) string {
	filled := v * barWidth / 100
	paint := render.Green
	switch {
	case v < 30:
		paint = render.Red
	case v < 60:
		paint = render.Yellow
	}
	return paint(strings.Repeat("█", filled)) + render.Dim(strings.Repeat("░", barWidth-filled))
}

// Names the mascot can be born with; the repo's name picks one.
var babyNames = []string{
	"Bit", "Pixel", "Commitchi", "Mergey", "Bugsy", "Patchy", "Gizmo", "Noodle", "Byte", "Rebasey",
}

// DefaultName picks a name from the repository's name, so the same repo
// always gets the same mascot.
func DefaultName(repo string) string {
	h := fnv.New32a()
	h.Write([]byte(strings.ToLower(repo)))
	return babyNames[h.Sum32()%uint32(len(babyNames))]
}

const nameFile = "gh-recess-mascot"

// LoadName reads the mascot's name from the .git folder, or "" if it was
// never named.
func LoadName(gitDir string) string {
	b, err := os.ReadFile(filepath.Join(gitDir, nameFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// MaxName is the longest name a mascot can have.
const MaxName = 16

// SaveName writes the mascot's name into the .git folder (never committed).
func SaveName(gitDir, name string) error {
	name = strings.TrimSpace(name)
	switch n := utf8.RuneCountInString(name); {
	case n == 0:
		return fmt.Errorf("the mascot needs a name")
	case n > MaxName:
		return fmt.Errorf("that's a long name: %d characters at most", MaxName)
	}
	return os.WriteFile(filepath.Join(gitDir, nameFile), []byte(name+"\n"), 0o644)
}
