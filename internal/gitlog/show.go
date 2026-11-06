package gitlog

import "strings"

// AddedHunks returns the blocks of lines that one commit added to one file.
// Each block is a run of consecutive new lines (a "hunk" in diff terms).
func AddedHunks(dir, hash, path string) ([][]string, error) {
	// -U0: no context lines, only what changed. --format= hides the header.
	out, err := git(dir, "show", "--format=", "--no-ext-diff", "--no-renames", "-U0", hash, "--", path)
	if err != nil {
		return nil, err
	}
	return ParseAddedHunks(out), nil
}

// ParseAddedHunks pulls the added lines ("+...") out of a unified diff.
func ParseAddedHunks(diff string) [][]string {
	var hunks [][]string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			hunks = append(hunks, cur)
		}
		cur = nil
	}

	// Before the first "@@" of each file come header lines ("+++ b/path")
	// that look like added lines, so we only collect once inside a hunk.
	inHunk := false
	for _, line := range strings.Split(diff, "\n") {
		line = strings.TrimSuffix(line, "\r")
		switch {
		case strings.HasPrefix(line, "diff --git"):
			flush()
			inHunk = false
		case strings.HasPrefix(line, "@@"):
			flush()
			inHunk = true
		case !inHunk:
			continue
		case strings.HasPrefix(line, "+"):
			cur = append(cur, line[1:])
		case strings.HasPrefix(line, "-"):
			// A removed line: with -U0 they come right before the added ones.
		default:
			// "\ No newline at end of file" and similar notes.
		}
	}
	flush()
	return hunks
}
