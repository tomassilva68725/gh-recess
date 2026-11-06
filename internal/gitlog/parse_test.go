package gitlog

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	out := "\x1eaaaaaaaaaa\x1fAndré Narquel\x1fAndre@Example.com\x1f1730000000\x1ffire on the frying pan\n\n" +
		"12\t3\tSource/Kitchen/Pan.cpp\n" +
		"-\t-\tContent/Kitchen/Pan.uasset\n" +
		"\x1ebbbbbbbbbb\x1fJoão\x1fjoao@example.com\x1f1730000100\x1fempty commit\n"

	commits, err := Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2", len(commits))
	}
	c := commits[0]
	if c.Author != "André Narquel" || c.Email != "andre@example.com" || c.Subject != "fire on the frying pan" {
		t.Errorf("header parsed wrong: %+v", c)
	}
	want := []FileChange{
		{Path: "Source/Kitchen/Pan.cpp", Added: 12, Deleted: 3},
		{Path: "Content/Kitchen/Pan.uasset", Binary: true},
	}
	if !reflect.DeepEqual(c.Files, want) {
		t.Errorf("files = %+v, want %+v", c.Files, want)
	}
	if a, d := c.Totals(); a != 12 || d != 3 {
		t.Errorf("totals = %d, %d", a, d)
	}
	if c.Short() != "aaaaaaa" {
		t.Errorf("short = %q", c.Short())
	}
	if len(commits[1].Files) != 0 {
		t.Errorf("second commit should have no files")
	}
}

func TestParseBadLine(t *testing.T) {
	if _, err := Parse("\x1eonly\x1ftwo\n"); err == nil {
		t.Error("expected an error for a broken header")
	}
}

func TestParseAddedHunks(t *testing.T) {
	diff := `diff --git a/a.cpp b/a.cpp
index 111..222 100644
--- a/a.cpp
+++ b/a.cpp
@@ -1,0 +2,2 @@
+int x = 1;
+++i;
@@ -10 +12 @@
-old();
+fresh();
\ No newline at end of file
`
	got := ParseAddedHunks(diff)
	want := [][]string{{"int x = 1;", "++i;"}, {"fresh();"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseLifecycle(t *testing.T) {
	out := "\x1eh1\x1fJoão\x1f1730000000\x1fadd pan\n\nA\tsrc/Pan.h\nM\tREADME.md\n" +
		"\x1eh2\x1fAndré\x1f1730000100\x1fmove and kill\n\nR087\tsrc/Pan.h\tKitchen/Pan.h\nD\tOld.cpp\n"
	events, err := ParseLifecycle(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("%d events", len(events))
	}
	want := []Change{{Status: 'R', From: "src/Pan.h", Path: "Kitchen/Pan.h"}, {Status: 'D', Path: "Old.cpp"}}
	if !reflect.DeepEqual(events[1].Changes, want) {
		t.Errorf("changes = %+v", events[1].Changes)
	}
	if len(events[0].Changes) != 1 || events[0].Author != "João" {
		t.Errorf("first event = %+v", events[0])
	}
}

func TestParseDeletedLines(t *testing.T) {
	got := ParseDeletedLines("\x1eh1\n\n0\t212\tOld.cpp\n-\t-\tPan.uasset\n\x1eh2\n")
	want := map[string]Deletion{"h1:Old.cpp": {Lines: 212}, "h1:Pan.uasset": {Binary: true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v", got)
	}
}

func TestParseRemote(t *testing.T) {
	cases := map[string]string{
		"https://github.com/tomassilva68725/CookFellas.git": "tomassilva68725/CookFellas",
		"https://github.com/a/b":                            "a/b",
		"git@github.com:a-narquel/gh-snitch.git":            "a-narquel/gh-snitch",
		"ssh://git@github.com/owner/name.git":               "owner/name",
		"https://github.com/":                               "",
		"nonsense":                                          "",
	}
	for in, want := range cases {
		if got := ParseRemote(in); got != want {
			t.Errorf("ParseRemote(%q) = %q, want %q", in, got, want)
		}
	}
}
