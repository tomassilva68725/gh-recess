package whodunit

import (
	"bufio"
	"cmp"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/tomassilva68725/gh-recess/internal/gitlog"
	"github.com/tomassilva68725/gh-recess/internal/render"
)

// Logo is the big WHODUNIT drawn when a game starts: yellow to red, like a
// crime scene tape.
var Logo = render.Logo{
	Text:     "whodunit",
	Gradient: []int{228, 221, 214, 208, 202, 196},
	Shadow:   94,
}

// Config is everything a game needs from the outside.
type Config struct {
	Rounds  int
	Players []string // empty: solo game
	In      *bufio.Scanner
	Out     io.Writer
	Animate bool
	Now     func() time.Time // a function so tests can control the clock
}

// errQuit means the player typed q (or closed the input) mid-game.
var errQuit = errors.New("quit")

const width = 66 // the width of the WHODUNIT logo; everything lines up with it

type game struct {
	cfg     Config
	players []*Player
	solo    bool
	played  int // rounds finished

	// For the end-of-game trivia, per suspect.
	guessed map[string]int // times someone guessed them right
	fooled  map[string]int // times someone guessed wrong when it was them
}

// Play runs a whole game. It returns an error only when no question could be
// asked at all.
func Play(deck *Deck, cfg Config) error {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	g := &game{cfg: cfg, guessed: map[string]int{}, fooled: map[string]int{}}
	if len(cfg.Players) == 0 {
		g.solo = true
		g.players = []*Player{{Name: "you"}}
	} else {
		for _, name := range cfg.Players {
			g.players = append(g.players, &Player{Name: name})
		}
	}

	g.intro()
	for round := 1; round <= cfg.Rounds; round++ {
		q, ok := deck.Next()
		if !ok {
			if round == 1 {
				return errors.New("couldn't find anything to ask about in this history")
			}
			fmt.Fprintf(cfg.Out, "\n  %s\n", render.Dim("That's all the evidence there is. Closing the case early."))
			break
		}
		if err := g.round(round, q); err != nil {
			fmt.Fprintf(cfg.Out, "\n  %s\n", render.Dim("Case dropped."))
			break
		}
		g.played++
	}
	g.summary()
	return nil
}

func (g *game) intro() {
	out := g.cfg.Out
	fmt.Fprintln(out)
	var delay time.Duration
	if g.cfg.Animate {
		delay = 45 * time.Millisecond
	}
	Logo.Draw(out, delay)
	fmt.Fprintf(out, "\n  %s\n", render.Bold("Somebody wrote this. Find out who."))
	if g.solo {
		fmt.Fprintf(out, "  %s rounds · faster answers score more · %s to give up\n",
			render.Number(g.cfg.Rounds), render.Bold("q"))
	} else {
		names := make([]string, len(g.players))
		for i, p := range g.players {
			names[i] = render.Cyan(p.Name)
		}
		fmt.Fprintf(out, "  %s rounds · detectives: %s · %s to give up\n",
			render.Number(g.cfg.Rounds), strings.Join(names, ", "), render.Bold("q"))
	}
}

// round shows one question, collects every player's guess and reveals the answer.
func (g *game) round(n int, q Question) error {
	g.showCase(n, q)

	start := g.cfg.Now()
	guesses := make([]int, len(g.players))
	for i, p := range g.players {
		prompt := "your guess"
		if !g.solo {
			prompt = p.Name
		}
		choice, err := g.ask(prompt, len(q.Options), q.Options)
		if err != nil {
			return err
		}
		guesses[i] = choice
	}
	elapsed := g.cfg.Now().Sub(start)

	fmt.Fprintln(g.cfg.Out)
	render.Suspense(g.cfg.Out, g.cfg.Animate, "  🔍 checking the evidence")
	g.reveal(q, guesses, elapsed)
	return nil
}

var kindLabel = map[Kind]string{
	Code:    "🔎 who wrote this code?",
	Message: "🧾 who wrote this commit message?",
	Clue:    "🔦 who did this? " + "(hardcore: no code, only clues)",
}

func (g *game) showCase(n int, q Question) {
	out := g.cfg.Out
	fmt.Fprintf(out, "\n  %s\n", render.Yellow(render.Title(fmt.Sprintf("CASE %d/%d", n, g.cfg.Rounds), width)))
	fmt.Fprintf(out, "  %s\n\n", render.Bold(kindLabel[q.Kind]))

	switch q.Kind {
	case Code:
		fmt.Fprintf(out, "  %s %s\n", render.Dim("┌─"), render.Cyan(q.File))
		for _, l := range q.Snippet {
			fmt.Fprintf(out, "  %s %s\n", render.Dim("│"), l)
		}
		fmt.Fprintf(out, "  %s\n", render.Dim("└─"))
	case Message:
		fmt.Fprintf(out, "     %s\n", render.Bold(render.Yellow("“"+q.Text+"”")))
	case Clue:
		c := q.Commit
		added, deleted := c.Totals()
		fmt.Fprintf(out, "     %s %s\n", render.Dim("when "), c.When.Format("Monday, 15:04"))
		fmt.Fprintf(out, "     %s %s %s in %s\n", render.Dim("size "),
			render.Green("+"+render.Number(added)), render.Red("−"+render.Number(deleted)),
			render.Plural(len(c.Files), "file"))
		for i, f := range topFiles(c, 3) {
			label := "     "
			if i == 0 {
				label = "files"
			}
			fmt.Fprintf(out, "     %s %s\n", render.Dim(label), render.Cyan(render.Trunc(f, 60)))
		}
		if extra := len(c.Files) - 3; extra > 0 {
			fmt.Fprintf(out, "           %s\n", render.Dim(fmt.Sprintf("+ %d more", extra)))
		}
	}

	fmt.Fprintln(out)
	for i, name := range q.Options {
		fmt.Fprintf(out, "    %s  %s\n", render.Bold(render.Magenta(strconv.Itoa(i+1))), name)
	}
	fmt.Fprintln(out)
}

// topFiles returns the paths of the files with the most changes.
func topFiles(c gitlog.Commit, n int) []string {
	files := slices.Clone(c.Files)
	slices.SortStableFunc(files, func(a, b gitlog.FileChange) int {
		return cmp.Compare(b.Added+b.Deleted, a.Added+a.Deleted)
	})
	var paths []string
	for _, f := range files[:min(n, len(files))] {
		paths = append(paths, f.Path)
	}
	return paths
}

// ask reads a guess: a number, or the start of a suspect's name.
func (g *game) ask(prompt string, n int, options []string) (int, error) {
	out := g.cfg.Out
	for {
		fmt.Fprintf(out, "  %s%s ", render.Bold(prompt), render.Dim(">"))
		if !g.cfg.In.Scan() {
			fmt.Fprintln(out)
			return 0, errQuit
		}
		text := strings.TrimSpace(g.cfg.In.Text())
		switch strings.ToLower(text) {
		case "":
			continue
		case "q", "quit", "exit":
			return 0, errQuit
		}
		if i, err := strconv.Atoi(text); err == nil && i >= 1 && i <= n {
			return i - 1, nil
		}
		if i, ok := matchName(text, options); ok {
			return i, nil
		}
		fmt.Fprintf(out, "  %s\n", render.Dim(fmt.Sprintf("type a number from 1 to %d (or q to give up)", n)))
	}
}

// matchName finds the one option whose name starts with what was typed.
func matchName(typed string, options []string) (int, bool) {
	t := fold(typed)
	found := -1
	for i, o := range options {
		if strings.HasPrefix(fold(o), t) {
			if found >= 0 {
				return 0, false // ambiguous
			}
			found = i
		}
	}
	return found, found >= 0
}

var accents = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n",
)

// fold makes names easy to compare: lower case, no accents ("andre" = "André").
func fold(s string) string {
	return accents.Replace(strings.ToLower(s))
}

func (g *game) reveal(q Question, guesses []int, elapsed time.Duration) {
	out := g.cfg.Out
	culprit := q.Culprit()

	for i, p := range g.players {
		right := guesses[i] == q.Answer
		pts := p.Record(right, elapsed, g.solo)
		if right {
			g.guessed[culprit]++
		} else {
			g.fooled[culprit]++
		}

		who := ""
		if !g.solo {
			who = render.Pad(p.Name, 12) + " "
		}
		switch {
		case right && p.Streak >= 2:
			fmt.Fprintf(out, "  %s%s %s  %s\n", who, render.Green("✓ Correct!"),
				render.Bold(render.Green("+"+strconv.Itoa(pts))),
				render.Yellow(fmt.Sprintf("🔥 %d in a row", p.Streak)))
		case right:
			fmt.Fprintf(out, "  %s%s %s\n", who, render.Green("✓ Correct!"),
				render.Bold(render.Green("+"+strconv.Itoa(pts))))
		case g.solo:
			fmt.Fprintf(out, "  %s %s\n", render.Red("✗ Nope."),
				render.Dim("You said "+q.Options[guesses[i]]+"."))
		default:
			fmt.Fprintf(out, "  %s%s %s\n", who, render.Red("✗ Nope."),
				render.Dim("said "+q.Options[guesses[i]]))
		}
	}

	c := q.Commit
	detail := "“" + render.Trunc(strings.TrimSpace(c.Subject), 50) + "”"
	if q.Kind == Message {
		detail = render.Plural(len(c.Files), "file")
	}
	fmt.Fprintf(out, "  %s %s %s\n", render.Dim("↳ it was"), render.Bold(render.Cyan(culprit)),
		render.Dim("· "+c.Short()+" · "+render.Date(c.When)+" · "+detail))
}

func (g *game) summary() {
	out := g.cfg.Out
	if g.played == 0 {
		return
	}
	fmt.Fprintf(out, "\n  %s\n\n", render.Yellow(render.Title("CASE CLOSED", width)))

	if g.solo {
		p := g.players[0]
		fmt.Fprintf(out, "     %s %s\n", render.Dim(render.Pad("score", 12)), render.Bold(render.Number(p.Score)))
		fmt.Fprintf(out, "     %s %d/%d\n", render.Dim(render.Pad("solved", 12)), p.Correct, p.Guesses)
		fmt.Fprintf(out, "     %s %d\n", render.Dim(render.Pad("best streak", 12)), p.Best)
		fmt.Fprintf(out, "     %s %s\n", render.Dim(render.Pad("rank", 12)), render.Bold(render.Magenta(Rank(p.Correct, p.Guesses))))
	} else {
		g.leaderboard()
	}
	g.trivia()
	fmt.Fprintln(out)
}

func (g *game) leaderboard() {
	out := g.cfg.Out
	ranked := slices.Clone(g.players)
	slices.SortStableFunc(ranked, func(a, b *Player) int { return cmp.Compare(b.Score, a.Score) })

	nameW := 6
	for _, p := range ranked {
		nameW = max(nameW, render.Width(p.Name))
	}
	top := float64(max(ranked[0].Score, 1))
	for i, p := range ranked {
		fmt.Fprintf(out, "   %s  %s  %s  %s  %s  %s\n",
			render.Dim(strconv.Itoa(i+1)+"."),
			render.Bold(render.Pad(p.Name, nameW)),
			render.Pad(render.Number(p.Score), 6),
			render.Pad(render.Magenta(render.Bar(float64(p.Score), top, 16)), 16),
			render.Dim(fmt.Sprintf("%d/%d", p.Correct, p.Guesses)),
			render.Italic(Rank(p.Correct, p.Guesses)))
	}
	if len(ranked) > 1 && ranked[0].Score > ranked[1].Score {
		fmt.Fprintf(out, "\n   🏆 %s\n", render.Bold(render.Yellow(ranked[0].Name+" cracked the case!")))
	} else if ranked[0].Score > 0 {
		fmt.Fprintf(out, "\n   🤝 %s\n", render.Bold("It's a tie at the top!"))
	}
}

// trivia names the easiest and the sneakiest suspects of the game.
func (g *game) trivia() {
	out := g.cfg.Out
	easy, easyN := "", 0
	for name, n := range g.guessed {
		if n > easyN || (n == easyN && name < easy) {
			easy, easyN = name, n
		}
	}
	sneaky, sneakyN := "", 0
	for name, n := range g.fooled {
		if n > sneakyN || (n == sneakyN && name < sneaky) {
			sneaky, sneakyN = name, n
		}
	}
	if easyN == 0 && sneakyN == 0 {
		return
	}
	fmt.Fprintln(out)
	if easyN >= 2 {
		fmt.Fprintf(out, "     %s %s %s\n", render.Dim(render.Pad("open book", 19)), render.Cyan(easy),
			render.Dim(fmt.Sprintf("(spotted %d times)", easyN)))
	}
	if sneakyN >= 1 {
		fmt.Fprintf(out, "     %s %s %s\n", render.Dim(render.Pad("master of disguise", 19)), render.Cyan(sneaky),
			render.Dim(fmt.Sprintf("(got away %s)", times(sneakyN))))
	}
}

func times(n int) string {
	if n == 1 {
		return "once"
	}
	return strconv.Itoa(n) + " times"
}
