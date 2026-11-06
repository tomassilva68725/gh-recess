package gitlog

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

// LoadAll reads the commits of every branch, local and remote-tracking, so
// work that hasn't been merged yet shows up too. With a non-zero since, only
// commits newer than that. File lists are left out (it's much faster).
func LoadAll(dir string, since time.Time) ([]Commit, error) {
	args := []string{"log", "--all", "--no-merges", logFormat}
	if !since.IsZero() {
		args = append(args, "--since="+since.Format(time.RFC3339))
	}
	out, err := git(dir, args...)
	if err != nil {
		if strings.Contains(err.Error(), "does not have any commits") {
			return nil, errors.New("this repository has no commits yet")
		}
		return nil, err
	}
	return Parse(out)
}

// Fetch updates the remote-tracking branches, quietly. It never asks for a
// password: if git would need one, the fetch just fails.
func Fetch(dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "fetch", "--all", "--quiet", "--no-tags")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=never")
	return cmd.Run()
}
