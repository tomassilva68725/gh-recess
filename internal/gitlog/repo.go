package gitlog

import (
	"errors"
	"path/filepath"
	"strings"
)

// TopLevel returns the root folder of the repository that contains dir.
func TopLevel(dir string) (string, error) {
	out, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		if strings.Contains(err.Error(), "not a git repository") {
			return "", errors.New("this folder is not a git repository (cd into one first)")
		}
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// CurrentBranch returns the name of the checked-out branch (e.g. "main").
func CurrentBranch(dir string) (string, error) {
	out, err := git(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// RepoName returns "owner/name" from the origin remote, or the folder name
// when there's no remote.
func RepoName(dir string) string {
	if out, err := git(dir, "remote", "get-url", "origin"); err == nil {
		if name := ParseRemote(strings.TrimSpace(out)); name != "" {
			return name
		}
	}
	return filepath.Base(dir)
}

// ParseRemote turns a remote URL into "owner/name". It understands
// https://github.com/owner/name.git, git@github.com:owner/name.git and
// ssh://git@host/owner/name.
func ParseRemote(url string) string {
	u := strings.TrimSuffix(strings.TrimSuffix(url, "/"), ".git")
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
		j := strings.Index(u, "/")
		if j < 0 {
			return ""
		}
		u = u[j+1:]
	} else if i := strings.Index(u, ":"); i >= 0 {
		u = u[i+1:]
	} else {
		return ""
	}

	parts := strings.Split(strings.Trim(u, "/"), "/")
	if len(parts) < 2 || parts[len(parts)-2] == "" || parts[len(parts)-1] == "" {
		return ""
	}
	return parts[len(parts)-2] + "/" + parts[len(parts)-1]
}
