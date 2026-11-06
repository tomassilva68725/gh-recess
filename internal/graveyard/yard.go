// Package graveyard turns deleted files into tombstones: who wrote them, who
// killed them, how long they lived and what their last words were.
package graveyard

import (
	"cmp"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/tomassilva68725/gh-recess/internal/gitlog"
)

// Life is one file from the moment it was added to the moment it was deleted.
// A file deleted, brought back and deleted again has two lives.
type Life struct {
	Path string

	Born     time.Time // zero when the birth is older than the history
	Creator  string
	BornHash string

	Died      time.Time
	Killer    string
	KillHash  string
	LastWords string // the subject of the commit that deleted it

	Lines    int  // how long it was when it died
	Binary   bool // images, .uasset...: no lines to count
	MassSize int  // files deleted by the same commit (1 = it died alone)
	CameBack bool // the same path was added again later
}

// Name is the file name without its folders.
func (l Life) Name() string { return path.Base(l.Path) }

// Age is how long the file lived. ok is false when its birth is unknown.
func (l Life) Age() (age time.Duration, ok bool) {
	if l.Born.IsZero() {
		return 0, false
	}
	return max(l.Died.Sub(l.Born), 0), true
}

// Yard holds every life that ended, newest death first.
type Yard struct {
	Dead []Life
}

// Build replays the history (oldest first) and records every death.
// sizes comes from gitlog.DeletedLines.
func Build(events []gitlog.Event, sizes map[string]gitlog.Deletion) *Yard {
	type birth struct {
		when time.Time
		who  string
		hash string
	}
	alive := map[string]birth{}
	lastDeath := map[string]int{} // path → index in dead of its latest death
	var dead []Life

	for _, e := range events {
		deletions := 0
		for _, c := range e.Changes {
			if c.Status == 'D' {
				deletions++
			}
		}

		for _, c := range e.Changes {
			switch c.Status {
			case 'A':
				if i, ok := lastDeath[c.Path]; ok {
					dead[i].CameBack = true
					delete(lastDeath, c.Path)
				}
				alive[c.Path] = birth{e.When, e.Author, e.Hash}
			case 'R':
				// A move: the file keeps its original birth.
				b := alive[c.From]
				delete(alive, c.From)
				alive[c.Path] = b
			case 'D':
				b := alive[c.Path]
				delete(alive, c.Path)
				size := sizes[e.Hash+":"+c.Path]
				if looksBinary(c.Path) {
					// Often stored in Git LFS, where git only sees a 3-line pointer.
					size = gitlog.Deletion{Binary: true}
				}
				dead = append(dead, Life{
					Path:      c.Path,
					Born:      b.when,
					Creator:   b.who,
					BornHash:  b.hash,
					Died:      e.When,
					Killer:    e.Author,
					KillHash:  e.Hash,
					LastWords: strings.TrimSpace(e.Subject),
					Lines:     size.Lines,
					Binary:    size.Binary,
					MassSize:  deletions,
				})
				lastDeath[c.Path] = len(dead) - 1
			}
		}
	}

	slices.Reverse(dead) // newest first; lives killed together stay together
	return &Yard{Dead: dead}
}

var binaryExt = map[string]bool{
	".uasset": true, ".umap": true, ".upk": true, ".pak": true,
	".png": true, ".jpg": true, ".jpeg": true, ".tga": true, ".bmp": true, ".gif": true, ".webp": true,
	".psd": true, ".exr": true, ".hdr": true, ".dds": true, ".ico": true, ".tif": true, ".tiff": true,
	".fbx": true, ".obj": true, ".blend": true, ".glb": true, ".max": true, ".ma": true, ".mb": true,
	".wav": true, ".mp3": true, ".ogg": true, ".flac": true, ".mp4": true, ".mov": true, ".webm": true,
	".ttf": true, ".otf": true, ".zip": true, ".7z": true, ".rar": true, ".pdf": true,
	".exe": true, ".dll": true, ".pdb": true, ".lib": true, ".so": true, ".dylib": true, ".bin": true,
}

// looksBinary says whether a file is an asset rather than text, by its extension.
func looksBinary(p string) bool {
	return binaryExt[strings.ToLower(path.Ext(p))]
}

// LFSSize reads a Git LFS pointer ("version https://git-lfs...\noid ...\nsize 11849")
// and returns the real file's size in bytes. ok is false if it isn't one.
func LFSSize(content string) (size int64, ok bool) {
	if !strings.HasPrefix(content, "version https://git-lfs.github.com/spec/") {
		return 0, false
	}
	for _, line := range strings.Split(content, "\n") {
		if v, found := strings.CutPrefix(strings.TrimSpace(line), "size "); found {
			n, err := strconv.ParseInt(v, 10, 64)
			return n, err == nil
		}
	}
	return 0, false
}

// massGrave is how many files one commit must delete to share a tombstone.
const massGrave = 4

// Grave is one tombstone: a single life, or a mass grave of lives that one
// commit ended together.
type Grave struct {
	Lives []Life
}

// Mass reports whether this is a shared grave.
func (g Grave) Mass() bool { return len(g.Lives) > 1 }

// Recent returns the n newest tombstones.
func (y *Yard) Recent(n int) []Grave {
	var graves []Grave
	for i := 0; i < len(y.Dead) && len(graves) < n; {
		j := i + 1
		for j < len(y.Dead) && y.Dead[j].KillHash == y.Dead[i].KillHash {
			j++
		}
		if j-i >= massGrave {
			graves = append(graves, Grave{Lives: y.Dead[i:j]})
		} else {
			for _, l := range y.Dead[i:j] {
				if len(graves) < n {
					graves = append(graves, Grave{Lives: []Life{l}})
				}
			}
		}
		i = j
	}
	return graves
}

// Stats are the numbers on the graveyard gate.
type Stats struct {
	Files    int // lives ended
	Lines    int // lines buried with them
	CameBack int // files that rose from the dead
	Killers  int
}

func (y *Yard) Stats() Stats {
	var s Stats
	killers := map[string]bool{}
	for _, l := range y.Dead {
		s.Files++
		s.Lines += l.Lines
		if l.CameBack {
			s.CameBack++
		}
		killers[l.Killer] = true
	}
	s.Killers = len(killers)
	return s
}

// Killer is someone who deleted files.
type Killer struct {
	Name    string
	Files   int
	Lines   int
	Commits int // separate deleting commits ("killing sprees")
}

// Killers ranks people by how many files they deleted.
func (y *Yard) Killers() []Killer {
	byName := map[string]*Killer{}
	sprees := map[string]bool{}
	for _, l := range y.Dead {
		k := byName[l.Killer]
		if k == nil {
			k = &Killer{Name: l.Killer}
			byName[l.Killer] = k
		}
		k.Files++
		k.Lines += l.Lines
		if !sprees[l.KillHash] {
			sprees[l.KillHash] = true
			k.Commits++
		}
	}
	list := make([]Killer, 0, len(byName))
	for _, k := range byName {
		list = append(list, *k)
	}
	slices.SortFunc(list, func(a, b Killer) int {
		return cmp.Or(cmp.Compare(b.Files, a.Files), cmp.Compare(b.Lines, a.Lines), cmp.Compare(a.Name, b.Name))
	})
	return list
}

// Record is one line of the hall of records.
type Record struct {
	Label string
	Life  Life
	Count int // massacre: files; zombie: deaths
}

// Records finds the notable lives. Records with no winner are left out.
func (y *Yard) Records() []Record {
	var longest, shortest, biggest, massacre *Life
	deaths := map[string]int{}
	for i := range y.Dead {
		l := &y.Dead[i]
		deaths[l.Path]++
		if age, ok := l.Age(); ok {
			if a, _ := longestAge(longest); longest == nil || age > a {
				longest = l
			}
			if a, _ := longestAge(shortest); shortest == nil || age < a {
				shortest = l
			}
		}
		if l.Lines > 0 && (biggest == nil || l.Lines > biggest.Lines) {
			biggest = l
		}
		if l.MassSize >= massGrave && (massacre == nil || l.MassSize > massacre.MassSize) {
			massacre = l
		}
	}

	var recs []Record
	if longest != nil {
		recs = append(recs, Record{Label: "longest life", Life: *longest})
	}
	if shortest != nil && shortest != longest {
		recs = append(recs, Record{Label: "shortest life", Life: *shortest})
	}
	if biggest != nil {
		recs = append(recs, Record{Label: "heaviest coffin", Life: *biggest})
	}
	if massacre != nil {
		recs = append(recs, Record{Label: "biggest massacre", Life: *massacre, Count: massacre.MassSize})
	}

	zombie, most := "", 1
	for p, n := range deaths {
		if n > most || (n == most && n > 1 && p < zombie) {
			zombie, most = p, n
		}
	}
	if zombie != "" {
		for _, l := range y.Dead { // newest death of that path
			if l.Path == zombie {
				recs = append(recs, Record{Label: "zombie", Life: l, Count: most})
				break
			}
		}
	}
	return recs
}

func longestAge(l *Life) (time.Duration, bool) {
	if l == nil {
		return 0, false
	}
	return l.Age()
}

// Find looks for graves by path. It tries, in order: the exact path, the
// exact file name, the start of the file name, and anywhere in the path.
// A file that died several times shows up once, with its newest death.
func (y *Yard) Find(query string) []Life {
	q := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(query, "\\", "/")))
	if q == "" {
		return nil
	}
	tiers := []func(l Life) bool{
		func(l Life) bool { return strings.ToLower(l.Path) == q },
		func(l Life) bool { return strings.ToLower(l.Name()) == q },
		func(l Life) bool { return strings.HasPrefix(strings.ToLower(l.Name()), q) },
		func(l Life) bool { return strings.Contains(strings.ToLower(l.Path), q) },
	}
	for _, match := range tiers {
		var found []Life
		seen := map[string]bool{}
		for _, l := range y.Dead {
			if match(l) && !seen[l.Path] {
				seen[l.Path] = true
				found = append(found, l)
			}
		}
		if len(found) > 0 {
			return found
		}
	}
	return nil
}

// Filter returns the lives whose path contains text (all of them when empty).
func (y *Yard) Filter(text string) []Life {
	q := strings.ToLower(strings.ReplaceAll(text, "\\", "/"))
	var out []Life
	for _, l := range y.Dead {
		if strings.Contains(strings.ToLower(l.Path), q) {
			out = append(out, l)
		}
	}
	return out
}
