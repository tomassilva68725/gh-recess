package whodunit

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tomassilva68725/gh-recess/internal/gitlog"
)

func rng() *rand.Rand { return rand.New(rand.NewPCG(1, 2)) }

// history builds a small fake repo: each author has a few commits that touch
// a code file and have a decent message.
func history(authors ...string) []gitlog.Commit {
	var commits []gitlog.Commit
	for i := range 12 {
		a := authors[i%len(authors)]
		commits = append(commits, gitlog.Commit{
			Hash:    fmt.Sprintf("%040d", i),
			Author:  a,
			Email:   strings.ToLower(strings.Fields(a)[0]) + "@example.com",
			When:    time.Date(2026, 11, 1+i%5, 3, 12, 0, 0, time.UTC),
			Subject: fmt.Sprintf("add frying logic number %d", i),
			Files:   []gitlog.FileChange{{Path: fmt.Sprintf("Source/Kitchen/F%d.cpp", i), Added: 20, Deleted: 2}},
		})
	}
	return commits
}

// fakeHunks returns code that mentions the author, to test the redaction.
func fakeHunks(commits []gitlog.Commit) HunkFunc {
	return func(hash, path string) ([][]string, error) {
		for _, c := range commits {
			if c.Hash == hash {
				first := strings.Fields(c.Author)[0]
				return [][]string{{
					"void Fry(float Heat)",
					"{",
					"\t// TODO(" + strings.ToLower(first) + "): tune this",
					"\tTemperature += Heat * DeltaTime;",
					"\tif (Temperature > BurnPoint) Ignite();",
					"}",
				}}, nil
			}
		}
		return nil, errors.New("no such commit")
	}
}

func TestNewDeckNeedsTwoPeople(t *testing.T) {
	commits := history("Solo Dev")
	if _, err := NewDeck(commits, fakeHunks(commits), rng()); err == nil {
		t.Error("expected an error with a single author")
	}
}

func TestDeckQuestions(t *testing.T) {
	commits := history("André Narquel", "João", "tomassilva68725", "Rita", "Zé Bot[bot]")
	deck, err := NewDeck(commits, fakeHunks(commits), rng())
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(deck.Suspects(), "Zé Bot[bot]") {
		t.Error("bots shouldn't be suspects")
	}

	seen := map[string]bool{}
	kinds := map[Kind]int{}
	for {
		q, ok := deck.Next()
		if !ok {
			break
		}
		kinds[q.Kind]++
		if seen[q.Commit.Hash] {
			t.Errorf("commit %s asked twice", q.Commit.Short())
		}
		seen[q.Commit.Hash] = true

		if len(q.Options) < 2 || len(q.Options) > maxOptions {
			t.Errorf("%d options", len(q.Options))
		}
		if q.Culprit() != q.Commit.Author {
			t.Errorf("answer %q, but the author is %q", q.Culprit(), q.Commit.Author)
		}
		if len(slices.Compact(slices.Sorted(slices.Values(q.Options)))) != len(q.Options) {
			t.Errorf("repeated options: %v", q.Options)
		}
		if q.Kind == Code {
			text := strings.ToLower(strings.Join(q.Snippet, "\n"))
			for _, name := range []string{"andré", "joão", "rita", "tomassilva68725"} {
				if strings.Contains(text, name) {
					t.Errorf("snippet gives away %q:\n%s", name, text)
				}
			}
		}
	}
	// 10 non-bot commits (the bot's two are dropped), each asked once.
	if len(seen) != 10 {
		t.Errorf("asked about %d commits, want 10", len(seen))
	}
	if kinds[Code] == 0 {
		t.Errorf("no code questions: %v", kinds)
	}
}

func TestHide(t *testing.T) {
	commits := history("André Narquel", "Ana")
	deck, _ := NewDeck(commits, fakeHunks(commits), rng())
	cases := map[string]string{
		"fixed by André":           "fixed by █████",
		"andré narquel did it":     "█████ did it",
		"ping andré@example.com":   "ping █████",
		"banana stays":             "banana stays",
		"Ana and André paired":     "█████ and █████ paired",
		"// TODO(ana): Ana, André": "// TODO(█████): █████, █████",
	}
	for in, want := range cases {
		if got := deck.hide(in); got != want {
			t.Errorf("hide(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBulkCommitsAreNotCodeQuestions(t *testing.T) {
	commits := history("André Narquel", "João")
	for i := range commits {
		for f := range maxBulkFiles {
			commits[i].Files = append(commits[i].Files, gitlog.FileChange{Path: fmt.Sprintf("T/%d.cpp", f), Added: 10})
		}
	}
	deck, err := NewDeck(commits, fakeHunks(commits), rng())
	if err != nil {
		t.Fatal(err)
	}
	for {
		q, ok := deck.Next()
		if !ok {
			break
		}
		if q.Kind == Code {
			t.Fatalf("got a code question from a %d-file commit", len(q.Commit.Files))
		}
	}
}

func TestPickSnippet(t *testing.T) {
	if got := PickSnippet([][]string{{"a", "}"}}, rng()); got != nil {
		t.Errorf("too short, got %q", got)
	}
	if got := PickSnippet([][]string{{"}", "}", ")", "", "}"}}, rng()); got != nil {
		t.Errorf("no real words, got %q", got)
	}
	hunk := []string{"", "\tif (Ready)", "\t{", "\t\tServe(Customer);", "\t\tCollectMoney();", "\t}", ""}
	got := PickSnippet([][]string{hunk}, rng())
	if len(got) < minLines {
		t.Fatalf("got %q", got)
	}
	if strings.HasPrefix(got[0], " ") || strings.HasPrefix(got[0], "}") {
		t.Errorf("first line should be dedented and not a closing brace: %q", got)
	}
}

func TestTidy(t *testing.T) {
	got := Tidy([]string{"\t\tfoo();", "", "  ", "\t\t\tbar();   ", "\t\t" + strings.Repeat("x", 100)})
	want := []string{"foo();", "", "    bar();", strings.Repeat("x", maxWidth-1) + "…"}
	if !slices.Equal(got, want) {
		t.Errorf("Tidy = %q, want %q", got, want)
	}
}

func TestIsCodeFile(t *testing.T) {
	yes := []gitlog.FileChange{
		{Path: "Source/CookFellas/Kitchen/Pan.cpp", Added: 10},
		{Path: "internal/cli/cli.go", Added: 3},
	}
	no := []gitlog.FileChange{
		{Path: "Content/Pan.uasset", Binary: true},
		{Path: "README.md", Added: 50},
		{Path: "a.cpp", Added: 2},
		{Path: "a.cpp", Added: 5000},
		{Path: "web/vendor/lib.js", Added: 40},
		{Path: "web/app.min.js", Added: 40},
		{Path: "Plugins/X/Intermediate/Gen.h", Added: 40},
	}
	for _, f := range yes {
		if !isCodeFile(f) {
			t.Errorf("%s should count as code", f.Path)
		}
	}
	for _, f := range no {
		if isCodeFile(f) {
			t.Errorf("%s (+%d) shouldn't count as code", f.Path, f.Added)
		}
	}
}

func TestPoints(t *testing.T) {
	if got := Points(0, 1, true); got != 150 {
		t.Errorf("instant answer = %d, want 150", got)
	}
	if got := Points(30*time.Second, 1, true); got != 100 {
		t.Errorf("slow answer = %d, want 100", got)
	}
	if got := Points(0, 3, false); got != 150 {
		t.Errorf("untimed streak 3 = %d, want 150", got)
	}
	if got := Points(time.Hour, 50, true); got != 200 {
		t.Errorf("streak bonus should cap: %d", got)
	}
}

func TestRank(t *testing.T) {
	cases := map[[2]int]string{
		{10, 10}: "Sherlock Holmes", {8, 10}: "Senior detective", {5, 10}: "Rookie cop",
		{3, 10}: "Confused witness", {0, 10}: "Prime suspect", {0, 0}: "No-show",
	}
	for in, want := range cases {
		if got := Rank(in[0], in[1]); got != want {
			t.Errorf("Rank(%d, %d) = %q, want %q", in[0], in[1], got, want)
		}
	}
}

func TestMatchName(t *testing.T) {
	opts := []string{"André Narquel", "Ana", "João"}
	if i, ok := matchName("andre", opts); !ok || i != 0 {
		t.Errorf("andre → %d %v", i, ok)
	}
	if _, ok := matchName("an", opts); ok {
		t.Error("'an' is ambiguous")
	}
	if i, ok := matchName("JOAO", opts); !ok || i != 2 {
		t.Errorf("JOAO → %d %v", i, ok)
	}
}

// A whole solo game, typed by a robot that always answers 1.
func TestPlaySolo(t *testing.T) {
	commits := history("André Narquel", "João", "Rita")
	deck, _ := NewDeck(commits, fakeHunks(commits), rng())
	var out bytes.Buffer
	in := bufio.NewScanner(strings.NewReader(strings.Repeat("1\n", 3)))
	err := Play(deck, Config{Rounds: 3, In: in, Out: &out})
	if err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"CASE 1/3", "CASE 3/3", "CASE CLOSED", "it was", "rank"} {
		if !strings.Contains(s, want) {
			t.Errorf("output is missing %q", want)
		}
	}
}

// Party mode: two players, and someone quits halfway through round 2.
func TestPlayPartyQuit(t *testing.T) {
	commits := history("André Narquel", "João")
	deck, _ := NewDeck(commits, fakeHunks(commits), rng())
	var out bytes.Buffer
	in := bufio.NewScanner(strings.NewReader("1\nwhat\n2\n1\nq\n"))
	err := Play(deck, Config{Rounds: 5, Players: []string{"tomas", "andre"}, In: in, Out: &out})
	if err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "type a number from 1 to 2") {
		t.Error("a bad answer should get a hint")
	}
	if !strings.Contains(s, "Case dropped") || !strings.Contains(s, "CASE CLOSED") {
		t.Errorf("quitting should still show the results:\n%s", s)
	}
	if strings.Contains(s, "CASE 3/5") {
		t.Error("the game went on after q")
	}
}
