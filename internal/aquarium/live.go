package aquarium

import (
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"

	"github.com/tomassilva68725/gh-recess/internal/gitlog"
	"github.com/tomassilva68725/gh-recess/internal/render"
)

// Logo is the big AQUARIUM: sea blue getting deeper.
var Logo = render.Logo{
	Text:     "aquarium",
	Gradient: []int{123, 87, 81, 75, 39, 33},
	Shadow:   24,
}

const (
	statusRows   = 2
	frameEvery   = 100 * time.Millisecond
	pollEvery    = 5 * time.Second
	fetchEvery   = time.Minute
	newsDuration = 10 * time.Second
	lookBack     = 7 * 24 * time.Hour // how far back the live check looks for new commits
)

// LiveConfig is what Live needs from the outside.
type LiveConfig struct {
	In, Out *os.File
	Repo    string
	Commits []gitlog.Commit // every branch, already loaded

	// Poll returns commits newer than since (from every branch).
	Poll func(since time.Time) ([]gitlog.Commit, error)
	// Fetch pulls the team's pushes; nil means don't fetch.
	Fetch func() error
}

// Live runs the aquarium full screen until q (or Ctrl+C) is pressed.
func Live(cfg LiveConfig) error {
	inFd, outFd := int(cfg.In.Fd()), int(cfg.Out.Fd())
	w, h, err := term.GetSize(outFd)
	if err != nil {
		return errors.New("the aquarium needs a real terminal window")
	}
	state, err := term.MakeRaw(inFd)
	if err != nil {
		return fmt.Errorf("can't read the keyboard: %w", err)
	}
	defer term.Restore(inFd, state)

	out := cfg.Out
	fmt.Fprint(out, "\x1b[?1049h\x1b[?25l") // alternate screen, hide the cursor
	defer fmt.Fprint(out, "\x1b[2J\x1b[?25h\x1b[?1049l")

	start := time.Now()
	tank := NewTank(Members(cfg.Commits, start, Window), w, h-statusRows,
		rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())), time.Now)
	seen := map[string]bool{}
	for _, c := range cfg.Commits {
		seen[c.Hash] = true
	}

	// The keyboard: q, Q, Esc or Ctrl+C quit. This goroutine stops after
	// reading the key that quits, so it never steals typing meant for the menu.
	quit := make(chan struct{})
	go func() {
		defer close(quit)
		buf := make([]byte, 1)
		for {
			n, err := cfg.In.Read(buf)
			if err != nil {
				return
			}
			if n == 1 && (buf[0] == 'q' || buf[0] == 'Q' || buf[0] == 3 || buf[0] == 27) {
				return
			}
		}
	}()

	// Git, in the background: fetch now and then, look for new commits often.
	news := make(chan []gitlog.Commit, 4)
	stop := make(chan struct{})
	defer close(stop)
	var mu sync.Mutex
	fetchNote := ""
	go func() {
		lastFetch := time.Time{}
		tick := time.NewTicker(pollEvery)
		defer tick.Stop()
		for {
			if cfg.Fetch != nil && time.Since(lastFetch) >= fetchEvery {
				lastFetch = time.Now()
				note := "fetched " + time.Now().Format("15:04")
				if err := cfg.Fetch(); err != nil {
					note = "fetch failed (offline?)"
				}
				mu.Lock()
				fetchNote = note
				mu.Unlock()
			}
			if cs, err := cfg.Poll(time.Now().Add(-lookBack)); err == nil {
				select {
				case news <- cs:
				case <-stop:
					return
				}
			}
			select {
			case <-tick.C:
			case <-stop:
				return
			}
		}
	}()

	var headline string
	var headlineUntil time.Time
	frame := time.NewTicker(frameEvery)
	defer frame.Stop()
	last := time.Now()
	for {
		select {
		case <-quit:
			return nil
		case cs := <-news:
			// Oldest first, so the newest commit ends up as the headline.
			for i := len(cs) - 1; i >= 0; i-- {
				c := cs[i]
				if seen[c.Hash] {
					continue
				}
				seen[c.Hash] = true
				if f := tank.Commit(c); f != nil {
					headline = fmt.Sprintf("%s committed: “%s”", f.Label, strings.TrimSpace(c.Subject))
					headlineUntil = time.Now().Add(newsDuration)
				}
			}
		case now := <-frame.C:
			dt := min(now.Sub(last).Seconds(), 0.5)
			last = now
			if nw, nh, err := term.GetSize(outFd); err == nil {
				w, h = nw, nh
			}
			if w < MinW || h < MinH+statusRows {
				fmt.Fprint(out, "\x1b[H\x1b[2J"+render.Bold("Make the window bigger to see the fish."))
				continue
			}
			tank.Resize(w, h-statusRows)
			tank.Step(dt)

			mu.Lock()
			note := fetchNote
			mu.Unlock()
			hint := ""
			if time.Now().Before(headlineUntil) {
				hint = headline
			}
			fmt.Fprint(out, frameText(tank, cfg.Repo, note, cfg.Fetch != nil, hint))
		}
	}
}

// frameText is one whole screen, drawn in a single write so it doesn't flicker.
func frameText(t *Tank, repo, fetchNote string, fetching bool, headline string) string {
	c := t.Draw()
	var b strings.Builder
	for y := range c.H {
		fmt.Fprintf(&b, "\x1b[%d;1H%s", y+1, c.Line(y, render.ColorEnabled))
	}
	l1, l2 := statusLines(t, repo, fetchNote, fetching, headline)
	fmt.Fprintf(&b, "\x1b[%d;1H%s\x1b[%d;1H%s", c.H+1, bar(l1, t.W, 236, true), c.H+2, bar(l2, t.W, 235, false))
	return b.String()
}

func statusLines(t *Tank, repo, fetchNote string, fetching bool, headline string) (string, string) {
	now := t.now()
	var newest *Fish
	for _, f := range t.Fish {
		if newest == nil || f.Last.After(newest.Last) {
			newest = f
		}
	}
	l1 := " AQUARIUM · " + repo + " · " + FishCount(len(t.Fish))
	if newest != nil && !newest.Last.IsZero() {
		l1 += " · last commit " + render.Age(now.Sub(newest.Last)) + " ago by " + newest.Label
	}
	if fetchNote != "" {
		l1 += " · " + fetchNote
	}

	l2 := headline
	if l2 != "" {
		l2 = " o°  " + l2
	} else {
		l2 = " q quit · fish grow with commits from the last 14 days · zzz = no commits for 3 days"
		if !fetching {
			l2 += " · start with --fetch to see pushes live"
		}
	}
	return l1, l2
}

// bar is a full-width status row.
func bar(text string, w, bg int, bold bool) string {
	text = render.Pad(render.Trunc(text, w), w)
	if !render.ColorEnabled {
		return text
	}
	style := fmt.Sprintf("\x1b[0m\x1b[48;5;%dm\x1b[38;5;252m", bg)
	if bold {
		style += "\x1b[1m"
	}
	return style + text + "\x1b[0m"
}

// Snapshot prints one still picture of the tank, for when the output isn't
// a terminal (a pipe, a file) and nothing can move.
func Snapshot(out io.Writer, commits []gitlog.Commit, now time.Time, w, h int) {
	tank := NewTank(Members(commits, now, Window), w, h, rand.New(rand.NewPCG(7, 7)), func() time.Time { return now })
	for range 30 {
		tank.Step(0.1) // let the bubbles rise a little
	}
	c := tank.Draw()
	for y := range c.H {
		fmt.Fprintln(out, c.Line(y, render.ColorEnabled))
	}
}

// icons are one-line fish for the legend, longer for bigger fish.
var icons = map[Tier]string{Fry: "><>", Small: "><(°>", Medium: "><((°>", Large: "><(((°>"}

// Legend lists the fish: size, recent commits and whether they're awake.
func Legend(out io.Writer, commits []gitlog.Commit, now time.Time) {
	members := Members(commits, now, Window)
	tank := NewTank(members, MinW*2, MinH*2, rand.New(rand.NewPCG(1, 1)), func() time.Time { return now })
	nameW := 4
	for _, f := range tank.Fish {
		nameW = max(nameW, render.Width(f.Label))
	}
	fmt.Fprintf(out, "\n  %s\n\n", render.Bold("Your fish:"))
	for _, f := range tank.Fish {
		state := render.Green("awake")
		switch {
		case f.Asleep(now):
			state = render.Dim("asleep zzz")
		case f.Active(now):
			state = render.Cyan("lively, blowing bubbles")
		}
		fmt.Fprintf(out, "   %s  %s  %s  %s\n",
			render.Color256(f.Color, render.Pad(icons[tank.Tier(f)], 9)),
			render.Bold(render.Pad(f.Label, nameW)),
			render.Dim(render.Pad(render.Plural(f.Recent, "commit")+" in 14 days", 21)),
			state)
	}
}
