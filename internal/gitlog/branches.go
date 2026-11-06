package gitlog

import (
	"strconv"
	"strings"
	"time"
)

// Branch is a branch name and when it last got a commit.
type Branch struct {
	Name string
	Last time.Time
}

// Unmerged lists the branches, local and remote, whose work isn't in the
// checked-out branch yet. A branch that exists both locally and on a remote
// ("City" and "origin/City") is listed once, with its newest commit.
func Unmerged(dir string) ([]Branch, error) {
	out, err := git(dir, "for-each-ref", "--no-merged=HEAD",
		"--format=%(refname)%09%(committerdate:unix)", "refs/heads", "refs/remotes")
	if err != nil {
		return nil, err
	}
	return ParseBranches(out), nil
}

// ParseBranches reads `git for-each-ref --format=%(refname)%09%(committerdate:unix)`.
func ParseBranches(out string) []Branch {
	newest := map[string]time.Time{}
	var order []string
	for _, line := range strings.Split(out, "\n") {
		ref, ts, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok {
			continue
		}
		var name string
		switch {
		case strings.HasPrefix(ref, "refs/heads/"):
			name = strings.TrimPrefix(ref, "refs/heads/")
		case strings.HasPrefix(ref, "refs/remotes/"):
			// refs/remotes/<remote>/<branch>
			_, name, _ = strings.Cut(strings.TrimPrefix(ref, "refs/remotes/"), "/")
		}
		if name == "" || name == "HEAD" {
			continue
		}
		sec, err := strconv.ParseInt(ts, 10, 64)
		if err != nil {
			continue
		}
		when := time.Unix(sec, 0)
		if old, seen := newest[name]; !seen {
			order = append(order, name)
			newest[name] = when
		} else if when.After(old) {
			newest[name] = when
		}
	}
	branches := make([]Branch, 0, len(order))
	for _, n := range order {
		branches = append(branches, Branch{Name: n, Last: newest[n]})
	}
	return branches
}

// GitDir returns the absolute path of the repository's .git folder, where
// recess keeps its own small files (they never get committed).
func GitDir(dir string) (string, error) {
	out, err := git(dir, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
