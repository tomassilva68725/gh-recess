// Package whodunit is the "guess who wrote it" game: it turns commits into
// questions (code, commit messages, hardcore clues) and runs the rounds.
package whodunit

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/tomassilva68725/gh-recess/internal/gitlog"
)

// Kind is the type of evidence a question shows.
type Kind int

const (
	Code    Kind = iota // a few lines of code someone added
	Message             // a commit message
	Clue                // hardcore: only when, how big and which files
)

// Question is one round of the game.
type Question struct {
	Kind    Kind
	Commit  gitlog.Commit
	File    string   // Code: the file the snippet came from
	Snippet []string // Code: the lines to show, names already hidden
	Text    string   // Message: the subject, names already hidden
	Options []string // the suspects to choose from
	Answer  int      // index of the real author in Options
}

// Culprit is the real author.
func (q Question) Culprit() string { return q.Options[q.Answer] }

// HunkFunc fetches the lines a commit added to a file (gitlog.AddedHunks in
// the real game, a fake in tests).
type HunkFunc func(hash, path string) ([][]string, error)

// MinSuspects is how many different authors the game needs to make sense.
const MinSuspects = 2

const (
	maxOptions = 4  // choices shown per question
	maxLookups = 30 // git calls per question before giving up on code

	// Commits touching more files than this are imports, not code someone
	// wrote, so they never become code questions (they still make good clues).
	maxBulkFiles = 300
)

// How often each kind comes up, out of 10.
var weights = map[Kind]int{Code: 5, Message: 3, Clue: 2}

type candidate struct {
	commit int // index into Deck.commits
	file   string
}

// Deck hands out questions without repeating a commit.
type Deck struct {
	commits  []gitlog.Commit
	suspects []string
	hunks    HunkFunc
	rng      *rand.Rand
	redact   *regexp.Regexp

	piles map[Kind][]candidate // shuffled; questions are drawn from the top
	used  map[string]bool      // commit hashes already asked about
}

// NewDeck prepares the questions for a history. It fails when there are
// fewer than MinSuspects people to choose from.
func NewDeck(commits []gitlog.Commit, hunks HunkFunc, rng *rand.Rand) (*Deck, error) {
	d := &Deck{hunks: hunks, rng: rng, used: map[string]bool{}, piles: map[Kind][]candidate{}}

	for _, c := range commits {
		if !isBot(c.Author) {
			d.commits = append(d.commits, c)
		}
	}
	d.suspects = Suspects(d.commits)
	if len(d.suspects) < MinSuspects {
		return nil, fmt.Errorf("whodunit needs at least %d people in the history (found %d)",
			MinSuspects, len(d.suspects))
	}
	d.redact = redactor(d.commits)

	for i, c := range d.commits {
		if goodSubject(c.Subject) {
			d.piles[Message] = append(d.piles[Message], candidate{commit: i})
		}
		if len(c.Files) > 0 {
			d.piles[Clue] = append(d.piles[Clue], candidate{commit: i})
		}
		if len(c.Files) > maxBulkFiles {
			continue // an import (engine template, asset pack): code nobody here wrote
		}
		for _, f := range c.Files {
			if isCodeFile(f) {
				d.piles[Code] = append(d.piles[Code], candidate{commit: i, file: f.Path})
			}
		}
	}
	for _, pile := range d.piles {
		rng.Shuffle(len(pile), func(i, j int) { pile[i], pile[j] = pile[j], pile[i] })
	}
	return d, nil
}

// Suspects returns the people this deck asks about, most commits first.
func (d *Deck) Suspects() []string { return slices.Clone(d.suspects) }

// Suspects lists the people in a history, most commits first. Bots are left out.
func Suspects(commits []gitlog.Commit) []string {
	count := map[string]int{}
	for _, c := range commits {
		if !isBot(c.Author) {
			count[c.Author]++
		}
	}
	names := make([]string, 0, len(count))
	for n := range count {
		names = append(names, n)
	}
	slices.SortFunc(names, func(a, b string) int {
		return cmp.Or(cmp.Compare(count[b], count[a]), cmp.Compare(a, b))
	})
	return names
}

// Next draws the next question. ok is false when the deck has run out.
func (d *Deck) Next() (q Question, ok bool) {
	for _, k := range d.kindOrder() {
		if q, ok := d.draw(k); ok {
			return q, true
		}
	}
	return Question{}, false
}

// kindOrder picks a kind at random (by weight) and puts the others after it,
// as fallbacks when the first one has nothing left.
func (d *Deck) kindOrder() []Kind {
	all := []Kind{Code, Message, Clue}
	total := 0
	for _, k := range all {
		total += weights[k]
	}
	roll := d.rng.IntN(total)
	first := Code
	for _, k := range all {
		if roll < weights[k] {
			first = k
			break
		}
		roll -= weights[k]
	}
	order := []Kind{first}
	for _, k := range all {
		if k != first {
			order = append(order, k)
		}
	}
	return order
}

func (d *Deck) draw(k Kind) (Question, bool) {
	lookups := 0
	for len(d.piles[k]) > 0 {
		if k == Code && lookups >= maxLookups {
			return Question{}, false // too slow: let another kind take this round
		}
		c := d.piles[k][0]
		d.piles[k] = d.piles[k][1:]
		commit := d.commits[c.commit]
		if d.used[commit.Hash] {
			continue
		}

		q := Question{Kind: k, Commit: commit}
		switch k {
		case Code:
			lookups++
			hunks, err := d.hunks(commit.Hash, c.file)
			if err != nil {
				continue
			}
			snippet := PickSnippet(hunks, d.rng)
			if snippet == nil {
				continue
			}
			for i := range snippet {
				snippet[i] = d.hide(snippet[i])
			}
			q.File, q.Snippet = c.file, snippet
		case Message:
			q.Text = d.hide(strings.TrimSpace(commit.Subject))
		}

		d.used[commit.Hash] = true
		q.Options, q.Answer = d.options(commit.Author)
		return q, true
	}
	return Question{}, false
}

// options picks up to maxOptions suspects, always including the culprit,
// in random order.
func (d *Deck) options(culprit string) ([]string, int) {
	var others []string
	for _, s := range d.suspects {
		if s != culprit {
			others = append(others, s)
		}
	}
	d.rng.Shuffle(len(others), func(i, j int) { others[i], others[j] = others[j], others[i] })
	if len(others) > maxOptions-1 {
		others = others[:maxOptions-1]
	}
	opts := append(others, culprit)
	d.rng.Shuffle(len(opts), func(i, j int) { opts[i], opts[j] = opts[j], opts[i] })
	return opts, slices.Index(opts, culprit)
}

const mask = "█████"

// hide blacks out anyone's name or email, so "// TODO(andre)" doesn't give
// the answer away.
func (d *Deck) hide(s string) string {
	if d.redact == nil {
		return s
	}
	// Twice, because two names next to each other share the separator between
	// them and the second one is skipped on the first pass.
	for range 2 {
		s = d.redact.ReplaceAllString(s, "${1}"+mask+"${3}")
	}
	return s
}

// redactor builds a pattern matching every author's name, each word of it,
// their email and the part of the email before the @, as whole words.
func redactor(commits []gitlog.Commit) *regexp.Regexp {
	seen := map[string]bool{}
	var words []string
	add := func(w string) {
		w = strings.TrimSpace(w)
		key := strings.ToLower(w)
		if utf8.RuneCountInString(w) < 3 || seen[key] {
			return
		}
		seen[key] = true
		words = append(words, w)
	}
	for _, c := range commits {
		add(c.Author)
		for _, part := range strings.Fields(c.Author) {
			add(part)
		}
		add(c.Email)
		local, _, _ := strings.Cut(c.Email, "@")
		add(local)
	}
	if len(words) == 0 {
		return nil
	}
	// Longest first, so "André Narquel" wins over "André".
	slices.SortFunc(words, func(a, b string) int { return cmp.Compare(len(b), len(a)) })
	for i, w := range words {
		words[i] = regexp.QuoteMeta(w)
	}
	// Go's \b only knows ASCII letters, so the word edges are spelled out.
	return regexp.MustCompile(`(?i)(^|[^\pL\pN_])(` + strings.Join(words, "|") + `)([^\pL\pN_]|$)`)
}

func isBot(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "[bot]") || n == "dependabot" || n == "github-actions"
}

// goodSubject filters out messages that say nothing: "wip", merges, reverts.
func goodSubject(s string) bool {
	s = strings.TrimSpace(s)
	low := strings.ToLower(s)
	if utf8.RuneCountInString(s) < 10 || len(strings.Fields(s)) < 2 {
		return false
	}
	for _, p := range []string{"merge ", "revert \"", "initial commit"} {
		if strings.HasPrefix(low, p) {
			return false
		}
	}
	return true
}

var codeExt = map[string]bool{
	".c": true, ".cc": true, ".cpp": true, ".cxx": true, ".h": true, ".hh": true, ".hpp": true,
	".cs": true, ".java": true, ".kt": true, ".kts": true, ".scala": true, ".swift": true,
	".go": true, ".rs": true, ".zig": true, ".py": true, ".rb": true, ".php": true, ".lua": true,
	".js": true, ".jsx": true, ".ts": true, ".tsx": true, ".mjs": true, ".vue": true, ".svelte": true,
	".css": true, ".scss": true, ".html": true, ".sql": true, ".sh": true, ".bash": true, ".ps1": true,
	".dart": true, ".ex": true, ".exs": true, ".hs": true, ".ml": true, ".gd": true,
	".hlsl": true, ".glsl": true, ".usf": true, ".ush": true,
}

// Folders full of code nobody on the team actually wrote.
var skipDirs = []string{
	"/vendor/", "/node_modules/", "/third_party/", "/thirdparty/", "/generated/",
	"/intermediate/", "/binaries/", "/dist/", "/build/",
}

// isCodeFile says whether a changed file is worth showing as a code snippet.
func isCodeFile(f gitlog.FileChange) bool {
	if f.Binary || f.Added < 3 || f.Added > 1500 { // huge additions are usually imports
		return false
	}
	p := "/" + strings.ToLower(filepath.ToSlash(f.Path))
	if !codeExt[filepath.Ext(p)] || strings.Contains(p, ".min.") {
		return false
	}
	for _, dir := range skipDirs {
		if strings.Contains(p, dir) {
			return false
		}
	}
	return true
}
