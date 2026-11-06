// Package mascot is the repository's tamagotchi: it eats commits, grows up,
// gets sick from stale branches, tired from late nights, and turns into a
// ghost when nobody commits for a week.
package mascot

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/tomassilva68725/gh-recess/internal/gitlog"
	"github.com/tomassilva68725/gh-recess/internal/render"
)

// Stage is how grown-up the mascot is.
type Stage int

const (
	Egg Stage = iota
	Baby
	Kid
	Teen
	Adult
)

var stageNames = [...]string{"egg", "baby", "kid", "teen", "adult"}

func (s Stage) String() string { return stageNames[s] }

// evolveAt is how many commits each stage needs.
var evolveAt = [...]int{Egg: 0, Baby: 5, Kid: 25, Teen: 75, Adult: 200}

// StageFor turns commits eaten into a stage.
func StageFor(commits int) Stage {
	s := Egg
	for st := Egg; st <= Adult; st++ {
		if commits >= evolveAt[st] {
			s = st
		}
	}
	return s
}

// Mood is how the mascot feels right now; it picks the face and the words.
type Mood int

const (
	Content Mood = iota
	Happy
	Excited
	Hungry
	Sad
	Sick
	Sleepy
	Ghost
)

var moodNames = [...]string{"content", "happy", "excited", "hungry", "sad", "sick", "sleepy", "a ghost"}

func (m Mood) String() string { return moodNames[m] }

// Stats are the four bars, from 0 to 100.
type Stats struct {
	Food   int // full right after a commit, empty after 3 days without one
	Joy    int // how much of the team committed this week
	Health int // stale branches make it sick
	Energy int // late-night commits wear it out
}

// Caretaker is someone who fed the mascot this week.
type Caretaker struct {
	Name    string
	Commits int
}

// Pet is everything the screen needs to know.
type Pet struct {
	Name       string
	Stage      Stage
	Mood       Mood
	Stats      Stats
	Born       time.Time // first commit
	Eaten      int       // commits, every branch
	Today      int       // commits in the last 24 hours
	Speech     string
	Next       Stage // the next stage, when there is one
	ToGo       int   // commits until it evolves (0 at Adult)
	Caretakers []Caretaker
	Stale      []gitlog.Branch // unmerged branches nobody touched for a week
}

const (
	day        = 24 * time.Hour
	fullFor    = 6 * time.Hour  // food stays at 100 this long after a commit
	starveAt   = 72 * time.Hour // food hits 0 here
	ghostAfter = 7 * day
	staleAfter = 7 * day
	lateFrom   = 0 // late-night commits: from midnight...
	lateUntil  = 5 // ...to 5 in the morning
)

// Input is what the mascot is made from.
type Input struct {
	Name     string
	Commits  []gitlog.Commit // every branch
	Unmerged []gitlog.Branch
	Now      time.Time
}

// Assess looks at the history and works out how the mascot is doing.
func Assess(in Input) Pet {
	p := Pet{Name: in.Name}
	now := in.Now

	var last gitlog.Commit
	weekly := map[string]int{}
	monthly := map[string]bool{}
	late := []gitlog.Commit{}
	for _, c := range in.Commits {
		if isBot(c.Author) {
			continue
		}
		p.Eaten++
		if p.Born.IsZero() || c.When.Before(p.Born) {
			p.Born = c.When
		}
		if c.When.After(last.When) {
			last = c
		}
		age := now.Sub(c.When)
		if age <= day {
			p.Today++
		}
		if age <= 7*day {
			weekly[c.Author]++
		}
		if age <= 30*day {
			monthly[c.Author] = true
		}
		if h := c.When.Hour(); age <= 3*day && h >= lateFrom && h < lateUntil {
			late = append(late, c)
		}
	}

	p.Stage = StageFor(p.Eaten)
	if p.Stage < Adult {
		p.Next = p.Stage + 1
		p.ToGo = evolveAt[p.Next] - p.Eaten
	}
	for name, n := range weekly {
		p.Caretakers = append(p.Caretakers, Caretaker{name, n})
	}
	slices.SortFunc(p.Caretakers, func(a, b Caretaker) int {
		return cmp.Or(cmp.Compare(b.Commits, a.Commits), cmp.Compare(a.Name, b.Name))
	})
	for _, b := range in.Unmerged {
		if now.Sub(b.Last) > staleAfter {
			p.Stale = append(p.Stale, b)
		}
	}

	// The bars.
	since := now.Sub(last.When)
	switch {
	case last.When.IsZero():
		p.Stats.Food = 0
	case since <= fullFor:
		p.Stats.Food = 100
	default:
		p.Stats.Food = clamp(100 - int(float64(since-fullFor)*100/float64(starveAt-fullFor)))
	}
	if len(monthly) > 0 {
		p.Stats.Joy = clamp(len(weekly) * 100 / len(monthly))
	}
	p.Stats.Health = clamp(100 - 20*len(p.Stale))
	p.Stats.Energy = max(clamp(100-25*len(late)), 10)

	// The mood: the worst problem wins, then the best news.
	switch {
	case p.Eaten == 0 || since > ghostAfter:
		p.Mood = Ghost
	case p.Stats.Food < 25:
		p.Mood = Hungry
	case p.Stats.Health < 50:
		p.Mood = Sick
	case p.Stats.Energy < 40:
		p.Mood = Sleepy
	case p.Stats.Joy < 60:
		p.Mood = Sad
	case p.Today >= 5:
		p.Mood = Excited
	case (p.Stats.Food+p.Stats.Joy+p.Stats.Health+p.Stats.Energy)/4 >= 75:
		p.Mood = Happy
	default:
		p.Mood = Content
	}
	p.Speech = speech(p, last, since, late, now)
	return p
}

// speech is what the mascot says in its bubble.
func speech(p Pet, last gitlog.Commit, since time.Duration, late []gitlog.Commit, now time.Time) string {
	short := func(n string) string {
		if f := strings.Fields(n); len(f) > 0 {
			return f[0]
		}
		return n
	}
	switch p.Mood {
	case Ghost:
		if p.Eaten == 0 {
			return "Boo. I haven't even hatched: nobody has ever committed here. Feed me one commit and I'll be born."
		}
		return fmt.Sprintf("Boooo. Nobody fed me for %s, so I'm a ghost now. Commit anything to bring me back to life.",
			render.Age(since))
	case Hungry:
		return fmt.Sprintf("I'm staaarving! My last meal was %s ago, from %s. A commit, please?",
			render.Age(since), short(last.Author))
	case Sick:
		names := make([]string, 0, len(p.Stale))
		for _, b := range p.Stale {
			names = append(names, b.Name)
		}
		return fmt.Sprintf("I feel sick… %s rotting in the fridge: %s. Merge them or throw them away.",
			plural(len(p.Stale), "branch is", "branches are"), render.Trunc(strings.Join(names, ", "), 60))
	case Sleepy:
		c := late[0]
		return fmt.Sprintf("*yawn* %s fed me at %s. I couldn't sleep. Go to bed, all of you.",
			short(c.Author), c.When.Format("15:04"))
	case Sad:
		return fmt.Sprintf("Only %s played with me this week. I miss the others.",
			plural(len(p.Caretakers), "person", "people"))
	case Excited:
		top := p.Caretakers[0]
		return fmt.Sprintf("WOW! %d commits today! %s is my favourite human right now.", p.Today, short(top.Name))
	case Happy:
		company := "most of the team came to play"
		if p.Stats.Joy == 100 {
			company = "the whole team came to play"
		}
		return fmt.Sprintf("I'm so happy! Full belly, %s. %s fed me %s ago.",
			company, short(last.Author), render.Age(since))
	default:
		return fmt.Sprintf("Life's good. %s fed me %s ago.", short(last.Author), render.Age(since))
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func clamp(v int) int { return min(max(v, 0), 100) }

func isBot(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "[bot]") || n == "dependabot" || n == "github-actions"
}
