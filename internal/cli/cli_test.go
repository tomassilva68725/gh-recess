package cli

import (
	"slices"
	"testing"
)

func TestParseWhodunitArgs(t *testing.T) {
	cases := []struct {
		args    []string
		rounds  int
		players []string
		solo    bool
	}{
		{nil, defaultRounds, nil, false},
		{[]string{"5"}, 5, nil, false},
		{[]string{"--rounds", "7"}, 7, nil, false},
		{[]string{"-r=3", "--solo"}, 3, nil, true},
		{[]string{"--players", "tomas,andré", "4"}, 4, []string{"tomas", "andré"}, false},
		{[]string{"--players=a, b ,A"}, defaultRounds, []string{"a", "b"}, false},
	}
	for _, c := range cases {
		got, err := parseWhodunitArgs(c.args)
		if err != nil {
			t.Errorf("%q: %v", c.args, err)
			continue
		}
		if got.rounds != c.rounds || !slices.Equal(got.players, c.players) || got.solo != c.solo {
			t.Errorf("%q = %+v", c.args, got)
		}
	}

	for _, bad := range [][]string{{"0"}, {"101"}, {"ten"}, {"--rounds"}, {"--fast"}} {
		if _, err := parseWhodunitArgs(bad); err == nil {
			t.Errorf("%q should fail", bad)
		}
	}
}

func TestEveryGameHasSomethingToSay(t *testing.T) {
	for _, g := range games {
		if g.run == nil && g.later == "" {
			t.Errorf("%s has neither a game nor a 'coming soon' line", g.name)
		}
	}
}
