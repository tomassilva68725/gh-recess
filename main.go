// gh-recess is a gh extension full of small games played on your git history.
package main

import (
	"os"

	"github.com/tomassilva68725/gh-recess/internal/cli"
)

func main() {
	// os.Args[0] is the program's own path; the rest are the user's arguments.
	os.Exit(cli.Run(os.Args[1:]))
}
