// Package cli is the user-facing part of recess: the banner, the game menu,
// the interactive prompt and the command line.
package cli

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/tomassilva68725/gh-recess/internal/gitlog"
	"github.com/tomassilva68725/gh-recess/internal/graveyard"
	"github.com/tomassilva68725/gh-recess/internal/render"
	"github.com/tomassilva68725/gh-recess/internal/whodunit"
)

// Version is shown in the banner and by --version.
const Version = "v0.1.0"

const tagline = "your repo's break time"

// logo is the big RECESS: cyan at the top fading into purple.
var logo = render.Logo{
	Text:     "recess",
	Gradient: []int{51, 45, 39, 63, 99, 135},
	Shadow:   61,
}

// App holds what every command needs: the loaded history and the terminal.
type App struct {
	dir     string
	repo    string
	branch  string
	commits []gitlog.Commit
	yard    *graveyard.Yard // loaded the first time graveyard runs

	in      *bufio.Scanner // one scanner for everything, or typed lines get lost
	out     io.Writer
	animate bool
	now     func() time.Time
}

// Run is the whole program. It returns the exit code (0 = success).
//
//	gh recess                       → banner, game menu and prompt
//	gh recess whodunit              → play one game and exit
//	gh recess whodunit --rounds 5   → same, shorter
func Run(args []string) int {
	flags := flag.NewFlagSet("gh recess", flag.ContinueOnError)
	showVersion := flags.Bool("version", false, "print the version and exit")
	noAnim := flags.Bool("no-anim", false, "turn off animations")
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "Usage: gh recess [flags] [game [options]]\n\n"+
			"Without a game, shows the menu (type 'help' there).\n\n"+
			"Games:\n"+
			"  whodunit [rounds] [--rounds N] [--players a,b,c] [--solo]\n"+
			"  graveyard [visit <name> | list [text] | killers]\n"+
			"  aquarium [--fetch]\n"+
			"  mascot [name <name>]\n\n"+
			"Flags:\n")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *showVersion {
		fmt.Println("gh-recess", Version)
		return 0
	}

	tty := isTerminal(os.Stdout) && enableVT(os.Stdout)
	render.ColorEnabled = tty && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"

	a := &App{
		in:      bufio.NewScanner(os.Stdin),
		out:     os.Stdout,
		animate: tty && !*noAnim,
		now:     time.Now,
	}
	interactive := flags.NArg() == 0

	if interactive {
		a.banner()
	}
	if err := a.load("."); err != nil {
		return 1
	}

	if !interactive {
		if err := a.exec(flags.Args()); err != nil {
			a.fail(err)
			return 1
		}
		return 0
	}

	a.header()
	a.menu()
	a.repl()
	return 0
}

func (a *App) banner() {
	fmt.Fprintln(a.out)
	var delay time.Duration
	if a.animate {
		delay = 45 * time.Millisecond
	}
	logo.Draw(a.out, delay)
	fmt.Fprintf(a.out, "  %s %s\n\n", render.Bold("gh-recess "+Version), render.Dim("· "+tagline))
}

// load reads the history, with a spinner while git works.
func (a *App) load(dir string) error {
	top, err := gitlog.TopLevel(dir)
	if err != nil {
		a.fail(err)
		return err
	}
	a.dir = top

	err = render.Step(a.out, a.animate, "Reading git history…", func() (string, error) {
		commits, err := gitlog.Load(top)
		if err != nil {
			return "", err
		}
		a.commits = commits
		return render.Plural(len(commits), "commit"), nil
	})
	if err != nil {
		return err
	}
	_ = render.Step(a.out, a.animate, "Lining up suspects…", func() (string, error) {
		return render.Plural(len(whodunit.Suspects(a.commits)), "suspect"), nil
	})

	a.repo = gitlog.RepoName(top)
	a.branch, _ = gitlog.CurrentBranch(top) // "_" ignores the error: no branch name is fine
	return nil
}

// header is the line under the loading steps:
//
//	tomassilva68725/CookFellas · branch main
func (a *App) header() {
	parts := []string{render.Bold(a.repo)}
	if a.branch != "" && a.branch != "HEAD" {
		parts = append(parts, "branch "+a.branch)
	}
	fmt.Fprintf(a.out, "\n %s\n", strings.Join(parts, render.Dim(" · ")))
}

// repl is the Read-Eval-Print Loop: show the prompt, read a line, run it, repeat.
func (a *App) repl() {
	for {
		fmt.Fprint(a.out, render.Bold(render.Magenta("recess"))+"> ")
		if !a.in.Scan() { // false on Ctrl+D / Ctrl+Z (end of input)
			fmt.Fprintln(a.out)
			return
		}
		args := strings.Fields(a.in.Text())
		if len(args) == 0 {
			continue
		}
		switch strings.ToLower(args[0]) {
		case "quit", "exit", "q":
			fmt.Fprintf(a.out, "\n  %s\n\n", render.Dim("🔔 break's over. Back to work!"))
			return
		}
		if err := a.exec(args); err != nil {
			a.fail(err)
		}
		fmt.Fprintln(a.out)
	}
}

// fail prints a red ✗ line.
func (a *App) fail(err error) {
	fmt.Fprintf(a.out, " %s %v\n", render.Red("✗"), err)
}

// isTerminal reports whether f is an interactive terminal (not a pipe or file).
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
