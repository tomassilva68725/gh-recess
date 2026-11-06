package gitlog

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Change is a file being added (A), deleted (D) or renamed (R) by a commit.
type Change struct {
	Status byte   // 'A', 'D' or 'R'
	Path   string // the file's path after the commit
	From   string // R only: the path before
}

// Event is a commit that added, deleted or renamed files.
type Event struct {
	Hash    string
	Author  string
	When    time.Time
	Subject string
	Changes []Change
}

const eventFormat = "--format=%x1e%H%x1f%aN%x1f%at%x1f%s"

// Lifecycle returns every commit that added, deleted or renamed a file,
// oldest first. Renames are detected (-M), so a moved file isn't a death.
func Lifecycle(dir string) ([]Event, error) {
	out, err := git(dir, "log", "--reverse", "--no-merges", "-M", "--diff-filter=ADR", "--name-status", eventFormat)
	if err != nil {
		return nil, err
	}
	return ParseLifecycle(out)
}

// ParseLifecycle reads the output of `git log --name-status` with eventFormat.
func ParseLifecycle(out string) ([]Event, error) {
	var events []Event
	for _, rec := range strings.Split(out, "\x1e") {
		rec = strings.Trim(rec, "\r\n")
		if rec == "" {
			continue
		}
		lines := strings.Split(rec, "\n")
		f := strings.Split(strings.TrimRight(lines[0], "\r"), "\x1f")
		if len(f) != 4 {
			return nil, fmt.Errorf("unexpected git log line: %q", lines[0])
		}
		ts, err := strconv.ParseInt(f[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("commit %s: bad timestamp %q", f[0], f[2])
		}
		e := Event{Hash: f[0], Author: f[1], When: time.Unix(ts, 0), Subject: f[3]}

		// "A\tpath", "D\tpath" or "R087\told\tnew" (87 = how similar they are).
		for _, l := range lines[1:] {
			parts := strings.Split(strings.TrimRight(l, "\r"), "\t")
			if len(parts) < 2 || parts[0] == "" {
				continue
			}
			switch s := parts[0][0]; {
			case (s == 'A' || s == 'D') && len(parts) == 2:
				e.Changes = append(e.Changes, Change{Status: s, Path: parts[1]})
			case s == 'R' && len(parts) == 3:
				e.Changes = append(e.Changes, Change{Status: 'R', From: parts[1], Path: parts[2]})
			}
		}
		events = append(events, e)
	}
	return events, nil
}

// Deletion is how big a file was when a commit deleted it.
type Deletion struct {
	Lines  int
	Binary bool
}

// DeletedLines maps "hash:path" to the size of each file at the moment it
// was deleted.
func DeletedLines(dir string) (map[string]Deletion, error) {
	out, err := git(dir, "log", "--no-merges", "-M", "--diff-filter=D", "--numstat", "--format=%x1e%H")
	if err != nil {
		return nil, err
	}
	return ParseDeletedLines(out), nil
}

// ParseDeletedLines reads `git log --numstat --format=%x1e%H` output.
func ParseDeletedLines(out string) map[string]Deletion {
	sizes := map[string]Deletion{}
	for _, rec := range strings.Split(out, "\x1e") {
		lines := strings.Split(strings.Trim(rec, "\r\n"), "\n")
		hash := strings.TrimSpace(lines[0])
		if hash == "" {
			continue
		}
		for _, l := range lines[1:] {
			parts := strings.SplitN(strings.TrimRight(l, "\r"), "\t", 3)
			if len(parts) != 3 {
				continue
			}
			d := Deletion{Binary: parts[1] == "-"}
			d.Lines, _ = strconv.Atoi(parts[1])
			sizes[hash+":"+parts[2]] = d
		}
	}
	return sizes
}

// FileBefore returns the content a file had just before a commit changed it
// (for a deleted file: its last version).
func FileBefore(dir, hash, path string) (string, error) {
	return git(dir, "show", hash+"^:"+path)
}
