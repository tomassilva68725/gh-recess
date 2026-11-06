// Package aquarium draws the team as fish swimming in the terminal: they
// grow with commits, sleep when nobody commits, and blow bubbles live.
package aquarium

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/tomassilva68725/gh-recess/internal/gitlog"
)

const (
	// Window is how far back commits count towards a fish's size.
	Window = 14 * 24 * time.Hour
	// SleepAfter: a fish whose owner hasn't committed for this long sleeps.
	SleepAfter = 3 * 24 * time.Hour
	// ActiveWithin: a fish whose owner committed this recently is lively.
	ActiveWithin = 24 * time.Hour
)

// Member is one person on the team and how much they've been committing.
type Member struct {
	Name   string
	Recent int // commits inside the window
	Total  int // commits ever
	Last   time.Time
}

// Members counts commits per person, most recent work first. Bots stay out.
func Members(commits []gitlog.Commit, now time.Time, window time.Duration) []Member {
	byName := map[string]*Member{}
	for _, c := range commits {
		if isBot(c.Author) {
			continue
		}
		m := byName[c.Author]
		if m == nil {
			m = &Member{Name: c.Author}
			byName[c.Author] = m
		}
		m.Total++
		if now.Sub(c.When) <= window {
			m.Recent++
		}
		if c.When.After(m.Last) {
			m.Last = c.When
		}
	}
	list := make([]Member, 0, len(byName))
	for _, m := range byName {
		list = append(list, *m)
	}
	slices.SortFunc(list, func(a, b Member) int {
		return cmp.Or(cmp.Compare(b.Recent, a.Recent), cmp.Compare(b.Total, a.Total), cmp.Compare(a.Name, b.Name))
	})
	return list
}

func isBot(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "[bot]") || n == "dependabot" || n == "github-actions"
}

// Tier is how big a fish is.
type Tier int

const (
	Fry Tier = iota
	Small
	Medium
	Large
)

// TierFor turns a number of commits into a size.
func TierFor(commits int) Tier {
	switch {
	case commits >= 25:
		return Large
	case commits >= 10:
		return Medium
	case commits >= 3:
		return Small
	default:
		return Fry
	}
}

func (t Tier) String() string {
	return [...]string{"fry", "small fish", "big fish", "whale-sized"}[t]
}

// maxLabel is the longest name shown under a fish.
const maxLabel = 16

// FishCount writes "1 fish" or "3 fish" (the plural of fish is fish).
func FishCount(n int) string {
	return strconv.Itoa(n) + " fish"
}
