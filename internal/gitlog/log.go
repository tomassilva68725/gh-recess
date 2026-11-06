package gitlog

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// FileChange is one file touched by a commit, as `git log --numstat` reports it.
type FileChange struct {
	Path    string
	Added   int
	Deleted int
	Binary  bool // images, .uasset, ...: git can't count their lines
}

// Commit is one commit from the history (merges are left out).
type Commit struct {
	Hash    string
	Author  string // after .mailmap, so one person's emails share a name
	Email   string
	When    time.Time
	Subject string // first line of the message
	Files   []FileChange
}

// Short is the abbreviated hash people recognise: "a1b2c3d".
func (c Commit) Short() string {
	if len(c.Hash) > 7 {
		return c.Hash[:7]
	}
	return c.Hash
}

// Totals adds up the lines added and deleted across every file.
func (c Commit) Totals() (added, deleted int) {
	for _, f := range c.Files {
		added += f.Added
		deleted += f.Deleted
	}
	return added, deleted
}

// Each commit starts with \x1e (record separator) and its fields are split by
// \x1f (unit separator): characters that never show up in names or messages.
const logFormat = "--format=%x1e%H%x1f%aN%x1f%aE%x1f%at%x1f%s"

// Load reads the whole history of the checked-out branch, newest first.
func Load(dir string) ([]Commit, error) {
	out, err := git(dir, "log", "--no-merges", "--no-renames", "--numstat", logFormat)
	if err != nil {
		if strings.Contains(err.Error(), "does not have any commits") {
			return nil, errors.New("this repository has no commits yet")
		}
		return nil, err
	}
	return Parse(out)
}

// Parse turns the output of `git log --numstat` with logFormat into commits.
func Parse(out string) ([]Commit, error) {
	var commits []Commit
	for _, rec := range strings.Split(out, "\x1e") {
		rec = strings.Trim(rec, "\r\n")
		if rec == "" {
			continue
		}
		lines := strings.Split(rec, "\n")

		f := strings.Split(strings.TrimRight(lines[0], "\r"), "\x1f")
		if len(f) != 5 {
			return nil, fmt.Errorf("unexpected git log line: %q", lines[0])
		}
		ts, err := strconv.ParseInt(f[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("commit %s: bad timestamp %q", f[0], f[3])
		}
		c := Commit{
			Hash:    f[0],
			Author:  f[1],
			Email:   strings.ToLower(f[2]),
			When:    time.Unix(ts, 0),
			Subject: f[4],
		}

		// The numstat lines: "12\t3\tpath", or "-\t-\tpath" for binary files.
		for _, l := range lines[1:] {
			l = strings.TrimRight(l, "\r")
			parts := strings.SplitN(l, "\t", 3)
			if len(parts) != 3 {
				continue
			}
			fc := FileChange{Path: parts[2]}
			if parts[0] == "-" {
				fc.Binary = true
			} else {
				fc.Added, _ = strconv.Atoi(parts[0])
				fc.Deleted, _ = strconv.Atoi(parts[1])
			}
			c.Files = append(c.Files, fc)
		}
		commits = append(commits, c)
	}
	return commits, nil
}
