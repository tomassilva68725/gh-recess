package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/tomassilva68725/gh-recess/internal/gitlog"
	"github.com/tomassilva68725/gh-recess/internal/mascot"
	"github.com/tomassilva68725/gh-recess/internal/render"
)

// mascot runs:
//
//	mascot               how the repo's tamagotchi is doing
//	mascot name <name>   give it a name (kept in .git, never committed)
func (a *App) mascot(args []string) error {
	gitDir, err := gitlog.GitDir(a.dir)
	if err != nil {
		return err
	}

	if len(args) > 0 {
		if strings.ToLower(args[0]) != "name" {
			return fmt.Errorf("unknown option %q (try: mascot, or mascot name <name>)", args[0])
		}
		name := strings.Join(args[1:], " ")
		if err := mascot.SaveName(gitDir, name); err != nil {
			return err
		}
		fmt.Fprintf(a.out, "\n  %s %s\n", render.Green("✓"),
			render.Bold("Your mascot is now called "+strings.TrimSpace(name)+"."))
		return nil
	}

	var pet mascot.Pet
	err = render.Step(a.out, a.animate, "Waking up the mascot…", func() (string, error) {
		commits, err := gitlog.LoadAll(a.dir, time.Time{})
		if err != nil {
			return "", err
		}
		unmerged, _ := gitlog.Unmerged(a.dir) // without it the mascot just can't get sick
		name := mascot.LoadName(gitDir)
		if name == "" {
			name = mascot.DefaultName(a.repo)
		}
		pet = mascot.Assess(mascot.Input{Name: name, Commits: commits, Unmerged: unmerged, Now: a.now()})
		return pet.Name + " is " + pet.Mood.String(), nil
	})
	if err != nil {
		return err
	}
	mascot.Show(a.out, pet, a.now(), a.animate)
	return nil
}
