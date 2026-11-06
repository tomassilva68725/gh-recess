package cli

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"

	"github.com/tomassilva68725/gh-recess/internal/gitlog"
	"github.com/tomassilva68725/gh-recess/internal/render"
	"github.com/tomassilva68725/gh-recess/internal/whodunit"
)

// game is one entry of the menu.
type game struct {
	name  string
	desc  string
	run   func(a *App, args []string) error // nil while it's still being built
	later string                            // what to say while run is nil
}

var games = []game{
	{name: "whodunit", desc: "guess who wrote the code", run: (*App).whodunit},
	{name: "graveyard", desc: "visit the code that died", run: (*App).graveyard},
	{name: "aquarium", desc: "your team swimming in real time", later: "🐟 The aquarium is still being filled."},
	{name: "mascot", desc: "your repo's tamagotchi", later: "🥚 The mascot hasn't hatched yet."},
}

// menu lists the games.
func (a *App) menu() {
	fmt.Fprintf(a.out, "\n %s\n\n", render.Bold("Pick a game:"))
	for i, g := range games {
		num := render.Bold(render.Magenta(strconv.Itoa(i + 1)))
		line := fmt.Sprintf("   %s  %s %s", num, render.Bold(render.Pad(g.name, 11)), render.Pad(g.desc, 33))
		if g.run == nil {
			line = fmt.Sprintf("   %s  %s %s %s", render.Dim(strconv.Itoa(i+1)),
				render.Dim(render.Pad(g.name, 11)), render.Dim(render.Pad(g.desc, 33)), render.Yellow("soon"))
		}
		fmt.Fprintln(a.out, line)
	}
	fmt.Fprintf(a.out, "\n Type a number or a name · %s for more · %s to leave\n\n",
		render.Bold("help"), render.Bold("quit"))
}

func (a *App) help() {
	rows := [][2]string{
		{"1 / whodunit [rounds]", "play whodunit (10 rounds unless you say otherwise)"},
		{"  --players a,b,c", "party mode: everyone guesses the same case"},
		{"  --solo", "skip the 'who's playing?' question"},
		{"2 / graveyard", "tombstones for every deleted file"},
		{"  visit <name>", "one grave up close, its last lines and how to bring it back"},
		{"  list [text]", "every grave (or the ones whose path has text)"},
		{"  killers", "who deleted the most"},
		{"menu", "clear the screen and show the games again"},
		{"help", "this list"},
		{"quit", "leave (or Ctrl+D / Ctrl+Z)"},
	}
	fmt.Fprintln(a.out)
	for _, r := range rows {
		fmt.Fprintf(a.out, "   %s %s\n", render.Bold(render.Pad(r[0], 24)), render.Dim(r[1]))
	}
	fmt.Fprintf(a.out, "\n %s\n", render.Dim("Any game also runs straight from the shell: gh recess whodunit --rounds 5"))
}

// exec runs one command line, already split into words.
func (a *App) exec(args []string) error {
	cmd := strings.ToLower(args[0])
	switch cmd {
	case "help", "?", "-h", "--help":
		a.help()
		return nil
	case "menu", "clear", "cls":
		if a.animate {
			fmt.Fprint(a.out, "\x1b[2J\x1b[3J\x1b[H") // clear screen and scrollback
		}
		a.banner()
		a.header()
		a.menu()
		return nil
	}

	for i, g := range games {
		if cmd != g.name && cmd != strconv.Itoa(i+1) {
			continue
		}
		if g.run == nil {
			fmt.Fprintf(a.out, "\n  %s %s\n", g.later, render.Dim("Coming soon!"))
			return nil
		}
		return g.run(a, args[1:])
	}
	return fmt.Errorf("unknown command %q (type 'help')", args[0])
}

// whodunitOptions is what the whodunit command line can change.
type whodunitOptions struct {
	rounds  int
	players []string
	solo    bool
}

const (
	defaultRounds = 10
	maxRounds     = 100
	maxPlayers    = 8
)

func (a *App) whodunit(args []string) error {
	opt, err := parseWhodunitArgs(args)
	if err != nil {
		return err
	}
	if len(opt.players) == 0 && !opt.solo {
		opt.players = a.askPlayers()
	}

	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	hunks := func(hash, path string) ([][]string, error) { return gitlog.AddedHunks(a.dir, hash, path) }
	deck, err := whodunit.NewDeck(a.commits, hunks, rng)
	if err != nil {
		return err
	}
	return whodunit.Play(deck, whodunit.Config{
		Rounds:  opt.rounds,
		Players: opt.players,
		In:      a.in,
		Out:     a.out,
		Animate: a.animate,
		Now:     a.now,
	})
}

// askPlayers asks who's playing. An empty answer means a solo game.
func (a *App) askPlayers() []string {
	fmt.Fprintf(a.out, "\n  %s %s\n  %s ",
		render.Bold("Who's playing?"),
		render.Dim("(names separated by commas, or Enter to play solo)"),
		render.Dim(">"))
	if !a.in.Scan() {
		return nil
	}
	players, _ := splitPlayers(a.in.Text())
	return players
}

// parseWhodunitArgs reads "10", "--rounds 10", "--rounds=10", "--players a,b"
// and "--solo", in any order.
func parseWhodunitArgs(args []string) (whodunitOptions, error) {
	opt := whodunitOptions{rounds: defaultRounds}
	setRounds := func(v string) error {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxRounds {
			return fmt.Errorf("rounds must be a number from 1 to %d (got %q)", maxRounds, v)
		}
		opt.rounds = n
		return nil
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			if err := setRounds(arg); err != nil {
				return opt, err
			}
			continue
		}

		name, value, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		next := func() (string, error) {
			if hasValue {
				return value, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("--%s needs a value", name)
			}
			i++
			return args[i], nil
		}

		switch name {
		case "rounds", "r":
			v, err := next()
			if err != nil {
				return opt, err
			}
			if err := setRounds(v); err != nil {
				return opt, err
			}
		case "players", "p":
			v, err := next()
			if err != nil {
				return opt, err
			}
			players, err := splitPlayers(v)
			if err != nil {
				return opt, err
			}
			opt.players = players
		case "solo":
			opt.solo = true
		default:
			return opt, fmt.Errorf("unknown option %q (whodunit takes --rounds, --players and --solo)", arg)
		}
	}
	return opt, nil
}

// splitPlayers turns "tomas, andré  joão" into three names, without repeats.
func splitPlayers(s string) ([]string, error) {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
	var players []string
	for _, f := range fields {
		if !slices.ContainsFunc(players, func(p string) bool { return strings.EqualFold(p, f) }) {
			players = append(players, f)
		}
	}
	if len(players) > maxPlayers {
		return players[:maxPlayers], fmt.Errorf("at most %d players", maxPlayers)
	}
	return players, nil
}
