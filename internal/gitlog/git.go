// Package gitlog runs git and turns its output into Go values.
package gitlog

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// git runs one git command inside dir and returns what it printed.
func git(dir string, args ...string) (string, error) {
	// quotepath=off keeps accented file names readable instead of "\303\251".
	full := append([]string{"-c", "core.quotepath=off", "-c", "color.ui=never"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", errors.New("git is not installed (or not in your PATH)")
		}
		msg := firstLine(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", args[0], msg)
	}
	return string(out), nil
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(line)
}
