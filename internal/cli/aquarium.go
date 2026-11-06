package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/tomassilva68725/gh-recess/internal/aquarium"
	"github.com/tomassilva68725/gh-recess/internal/gitlog"
	"github.com/tomassilva68725/gh-recess/internal/render"
)

// aquarium runs:
//
//	aquarium            the tank, full screen, live (q to leave)
//	aquarium --fetch    same, fetching the team's pushes every minute
func (a *App) aquarium(args []string) error {
	fetch := false
	for _, arg := range args {
		switch arg {
		case "--fetch", "-fetch", "fetch":
			fetch = true
		default:
			return fmt.Errorf("unknown option %q (aquarium takes --fetch)", arg)
		}
	}

	var commits []gitlog.Commit
	err := render.Step(a.out, a.animate, "Filling the tank…", func() (string, error) {
		var err error
		commits, err = gitlog.LoadAll(a.dir, time.Time{})
		if err != nil {
			return "", err
		}
		return aquarium.FishCount(len(aquarium.Members(commits, a.now(), aquarium.Window))), nil
	})
	if err != nil {
		return err
	}

	if !a.animate {
		// Not a terminal (or --no-anim): a still picture and the legend.
		fmt.Fprintln(a.out)
		aquarium.Snapshot(a.out, commits, a.now(), 78, 18)
		aquarium.Legend(a.out, commits, a.now())
		return nil
	}

	fmt.Fprintln(a.out)
	aquarium.Logo.Draw(a.out, 45*time.Millisecond)
	fmt.Fprintf(a.out, "\n  %s\n", render.Dim("Diving in… (q to come back up)"))
	time.Sleep(900 * time.Millisecond)

	cfg := aquarium.LiveConfig{
		In:      os.Stdin,
		Out:     os.Stdout,
		Repo:    a.repo,
		Commits: commits,
		Poll:    func(since time.Time) ([]gitlog.Commit, error) { return gitlog.LoadAll(a.dir, since) },
	}
	if fetch {
		cfg.Fetch = func() error { return gitlog.Fetch(a.dir) }
	}
	if err := aquarium.Live(cfg); err != nil {
		return err
	}

	// Back on the normal screen: who's who, with fresh numbers.
	if latest, err := gitlog.LoadAll(a.dir, time.Time{}); err == nil {
		commits = latest
	}
	aquarium.Legend(a.out, commits, a.now())
	return nil
}
