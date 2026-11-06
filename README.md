# gh-recess

```
██████╗ ███████╗ ██████╗███████╗███████╗███████╗
██╔══██╗██╔════╝██╔════╝██╔════╝██╔════╝██╔════╝
██████╔╝█████╗  ██║     █████╗  ███████╗███████╗
██╔══██╗██╔══╝  ██║     ██╔══╝  ╚════██║╚════██║
██║  ██║███████╗╚██████╗███████╗███████║███████║
╚═╝  ╚═╝╚══════╝ ╚═════╝╚══════╝╚══════╝╚══════╝
```

A `gh` extension with small games played on your repository's history.
Your repo's break time.

It only reads your local clone (`git log`, `git show`), so it works on private
repositories without a paid plan and never sends your code anywhere.

## Install

```sh
gh extension install tomassilva68725/gh-recess
# or, from a clone of this repo:
go build && gh extension install .
```

## Use

Run it inside any git repository:

```sh
gh recess                              # menu
gh recess whodunit                     # play straight away
gh recess whodunit 5 --players tomas,andre,joao
```

| Game | What it is | |
|---|---|---|
| `whodunit` | Guess who wrote a piece of code, a commit message, or (hardcore) a commit from clues only | ✅ |
| `graveyard` | Tombstones for deleted code: who wrote it, who killed it, how long it lived | soon |
| `aquarium` | Your team as fish swimming in the terminal, growing with every commit | soon |
| `mascot` | A tamagotchi that lives on commits and gets sick from stale PRs | soon |

### whodunit

Each case shows one piece of evidence and up to four suspects. Type the number
(or the start of the name). Names and emails in the evidence are blacked out.

- **Solo**: faster answers score more (up to +50), and every answer in a row
  adds +25 more.
- **Party** (`--players a,b,c`, or answer "Who's playing?"): everyone guesses
  the same case, perfect for a screen share on a call.
- At the end: score, rank (from *Prime suspect* to *Sherlock Holmes*), the
  **open book** (easiest person to spot) and the **master of disguise**.

Options: `[rounds]` or `--rounds N` (default 10), `--players a,b,c`, `--solo`.
Global flags: `--no-anim`, `--version`.

To merge one person's several emails into one suspect, add a
[`.mailmap`](https://git-scm.com/docs/gitmailmap).

On Windows, use Windows Terminal for the colors and block characters.

## Layout

```
main.go             entry point: calls cli.Run
internal/gitlog/    runs git log / git show and parses the output
internal/render/    colors, the big block font, bars, spinners
internal/whodunit/  questions (deck), snippets, scoring, the game loop
internal/cli/       banner, game menu, prompt, command-line options
```

Run the tests with `go test ./...`. Releases are built by
`.github/workflows/release.yml` when you push a tag like `v0.1.0`.

## License

gh-recess is released under the
[PolyForm Noncommercial License 1.0.0](LICENSE.md).

- ✅ You may use, copy, modify and share it for **noncommercial purposes**:
  personal use, study, hobby projects, playing with your team, education.
- ❌ **Commercial use by third parties is not allowed.** You may not sell it,
  include it in a paid product or service, or use it for commercial purposes
  without written permission from the author.
- Anyone you share it with must also get a copy of the license, including the
  `Required Notice` line.

For commercial licensing, contact the author through
[GitHub](https://github.com/tomassilva68725).

Copyright © 2026 tomassilva68725.
