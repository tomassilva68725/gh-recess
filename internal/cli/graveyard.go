package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tomassilva68725/gh-recess/internal/gitlog"
	"github.com/tomassilva68725/gh-recess/internal/graveyard"
	"github.com/tomassilva68725/gh-recess/internal/render"
)

// graveyard runs:
//
//	graveyard                 the gate: newest tombstones, records, killers
//	graveyard visit <name>    one grave up close (also: graveyard <name>)
//	graveyard list [text]     every grave, or those whose path has text
//	graveyard killers         who deleted the most
func (a *App) graveyard(args []string) error {
	yard, err := a.loadYard()
	if err != nil {
		return err
	}
	show := graveyard.NewShow(yard, a.out, a.animate)

	if len(args) == 0 {
		show.Overview()
		return nil
	}
	switch strings.ToLower(args[0]) {
	case "list", "ls":
		show.List(strings.Join(args[1:], " "))
		return nil
	case "killers":
		show.KillersTable()
		return nil
	case "visit", "v":
		args = args[1:]
		if len(args) == 0 {
			return errors.New("visit who? try: graveyard visit <file name>")
		}
	}

	query := strings.Join(args, " ")
	found := yard.Find(query)
	switch len(found) {
	case 0:
		return fmt.Errorf("no grave matches %q (see 'graveyard list')", query)
	case 1:
	default:
		show.Choices(query, found)
		return nil
	}

	l := found[0]
	var code []string
	var lfs int64 = -1
	if content, err := gitlog.FileBefore(a.dir, l.KillHash, l.Path); err == nil {
		if size, ok := graveyard.LFSSize(content); ok {
			lfs = size
		} else if !l.Binary {
			code = graveyard.FirstLines(content, graveyard.LastLines)
		}
	}
	show.Visit(l, code, lfs)
	return nil
}

// loadYard digs up the graveyard the first time it's needed.
func (a *App) loadYard() (*graveyard.Yard, error) {
	if a.yard != nil {
		return a.yard, nil
	}
	err := render.Step(a.out, a.animate, "Digging up graves…", func() (string, error) {
		events, err := gitlog.Lifecycle(a.dir)
		if err != nil {
			return "", err
		}
		sizes, err := gitlog.DeletedLines(a.dir)
		if err != nil {
			return "", err
		}
		a.yard = graveyard.Build(events, sizes)
		return render.Plural(len(a.yard.Dead), "grave"), nil
	})
	return a.yard, err
}
