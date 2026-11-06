package aquarium

import (
	"math/rand/v2"
	"strings"
	"time"

	"github.com/tomassilva68725/gh-recess/internal/gitlog"
	"github.com/tomassilva68725/gh-recess/internal/render"
)

const (
	sandRows = 2  // rows of sand at the bottom
	topRow   = 2  // highest row a fish swims in (row 0 is the surface)
	MinW     = 30 // smaller than this and there's no room to swim
	MinH     = 10
)

// Colors for the fish, in the order members are listed.
var palette = []int{208, 51, 213, 226, 118, 39, 203, 141, 214, 87}

const (
	sleepColor  = 245
	eyeColor    = 231
	bubbleColor = 195
	zzzColor    = 250
)

// Water gets darker the deeper you go.
var waterBG = []int{25, 25, 24, 24, 18, 18, 17, 17}

// How fast each size swims, in columns per second.
var speed = map[Tier]float64{Fry: 7, Small: 5.5, Medium: 4.5, Large: 3.5}

// Fish is one member of the team, swimming.
type Fish struct {
	Member
	Label string // short name shown under the fish
	Color int

	x, y     float64
	dir      int // 1 = facing right, -1 = left
	targetY  float64
	retarget float64 // seconds until it picks a new depth
	bubbleIn float64 // seconds until the next bubble
	burst    int     // bubbles left from a commit celebration
	flash    float64 // seconds left of glowing after a commit
}

// Asleep: no commits for SleepAfter.
func (f *Fish) Asleep(now time.Time) bool { return now.Sub(f.Last) > SleepAfter }

// Active: committed within ActiveWithin.
func (f *Fish) Active(now time.Time) bool { return now.Sub(f.Last) <= ActiveWithin }

type bubble struct {
	x     int
	y, vy float64
	age   float64
	zzz   bool // a sleeping fish's snore instead of air
}

type weed struct{ x, h int }

// Tank is the whole aquarium. Call Step to move time forward and Draw to
// get a picture of it.
type Tank struct {
	W, H int
	Fish []*Fish

	bubbles  []bubble
	weeds    []weed
	sand     []rune
	rng      *rand.Rand
	t        float64 // seconds since the tank was filled
	useTotal bool    // nobody committed inside the window: size by all-time commits
	now      func() time.Time
}

// NewTank fills a w×h tank with one fish per member.
func NewTank(members []Member, w, h int, rng *rand.Rand, now func() time.Time) *Tank {
	t := &Tank{rng: rng, now: now, useTotal: true}
	for _, m := range members {
		if m.Recent > 0 {
			t.useTotal = false
		}
	}
	var names []string
	for _, m := range members {
		names = append(names, m.Name)
	}
	short := render.ShortNames(names)

	t.Resize(w, h)
	for i, m := range members {
		label := m.Name
		if s, ok := short[m.Name]; ok {
			label = s
		}
		f := &Fish{Member: m, Label: render.Trunc(label, maxLabel), Color: palette[i%len(palette)], dir: 1}
		if rng.IntN(2) == 0 {
			f.dir = -1
		}
		fw, fh := t.size(f)
		minY, maxY := t.yRange(fh)
		f.x = float64(rng.IntN(max(t.W-fw, 1)))
		f.y = minY + rng.Float64()*(maxY-minY)
		f.targetY = f.y
		f.bubbleIn = rng.Float64() * 2
		t.Fish = append(t.Fish, f)
	}
	return t
}

// Tier is the fish's current size.
func (t *Tank) Tier(f *Fish) Tier {
	if t.useTotal {
		return TierFor(f.Total)
	}
	return TierFor(f.Recent)
}

func (t *Tank) size(f *Fish) (w, h int) { return artSize(t.Tier(f)) }

// yRange is where a fish of height h may swim: below the surface, above the
// sand, with a row left for its name.
func (t *Tank) yRange(h int) (minY, maxY float64) {
	minY = topRow
	maxY = float64(t.H - sandRows - h - 1)
	return minY, max(maxY, minY)
}

// Resize fits the tank to a new terminal size, replanting the decor.
func (t *Tank) Resize(w, h int) {
	if w == t.W && h == t.H {
		return
	}
	t.W, t.H = w, h

	t.weeds = nil
	for x := 2 + t.rng.IntN(5); x < w-1; x += 7 + t.rng.IntN(9) {
		t.weeds = append(t.weeds, weed{x: x, h: 2 + t.rng.IntN(max(min(6, h/3), 1))})
	}
	t.sand = make([]rune, w)
	grains := []rune("░░░░░.:░░")
	for x := range t.sand {
		t.sand[x] = grains[t.rng.IntN(len(grains))]
	}
	for _, f := range t.Fish {
		t.keepInside(f)
	}
}

func (t *Tank) keepInside(f *Fish) {
	fw, fh := t.size(f)
	minY, maxY := t.yRange(fh)
	f.x = min(max(f.x, 0), float64(max(t.W-fw, 0)))
	f.y = min(max(f.y, minY), maxY)
	f.targetY = min(max(f.targetY, minY), maxY)
}

// Step moves everything forward by dt seconds.
func (t *Tank) Step(dt float64) {
	t.t += dt
	now := t.now()

	for _, f := range t.Fish {
		fw, fh := t.size(f)
		minY, maxY := t.yRange(fh)
		v := 0.6 // a sleeping fish barely drifts

		if f.Asleep(now) && f.burst == 0 {
			f.targetY = maxY
			f.bubbleIn -= dt
			if f.bubbleIn <= 0 {
				t.bubbles = append(t.bubbles, bubble{x: int(f.x) + fw/2, y: f.y - 1, vy: -1.2, zzz: true})
				f.bubbleIn = 2 + t.rng.Float64()*2
			}
		} else {
			v = speed[t.Tier(f)]
			if f.Active(now) {
				v *= 1.6
			}
			f.retarget -= dt
			if f.retarget <= 0 {
				f.targetY = minY + t.rng.Float64()*(maxY-minY)
				f.retarget = 3 + t.rng.Float64()*5
			}
			if t.rng.Float64() < dt*0.05 {
				f.dir = -f.dir // changed its mind
			}
			if f.Active(now) || f.burst > 0 {
				f.bubbleIn -= dt
				if f.bubbleIn <= 0 {
					t.blow(f, fw, fh)
					if f.burst > 0 {
						f.burst--
						f.bubbleIn = 0.12
					} else {
						f.bubbleIn = 1.5 + t.rng.Float64()*3
					}
				}
			}
		}

		// Swim, turning around at the glass.
		f.x += float64(f.dir) * v * dt
		if f.x <= 0 {
			f.x, f.dir = 0, 1
		}
		if limit := float64(max(t.W-fw, 0)); f.x >= limit {
			f.x, f.dir = limit, -1
		}
		// Glide towards the chosen depth.
		switch step := 1.5 * dt; {
		case f.y < f.targetY-step:
			f.y += step
		case f.y > f.targetY+step:
			f.y -= step
		default:
			f.y = f.targetY
		}
		t.keepInside(f)
		f.flash = max(f.flash-dt, 0)
	}

	alive := t.bubbles[:0]
	for _, b := range t.bubbles {
		b.y += b.vy * dt
		b.age += dt
		if b.y >= 1 && b.x >= 0 && b.x < t.W {
			alive = append(alive, b)
		}
	}
	t.bubbles = alive
}

// blow lets out one bubble from the fish's mouth.
func (t *Tank) blow(f *Fish, fw, fh int) {
	x := int(f.x) + fw
	if f.dir < 0 {
		x = int(f.x) - 1
	}
	t.bubbles = append(t.bubbles, bubble{x: x, y: f.y + float64(fh/2), vy: -(2.5 + t.rng.Float64())})
}

// Commit celebrates a new commit: its author's fish wakes up, grows if it
// earned it, glows and blows a stream of bubbles. Someone new gets a new fish.
func (t *Tank) Commit(c gitlog.Commit) *Fish {
	if isBot(c.Author) {
		return nil
	}
	var fish *Fish
	for _, f := range t.Fish {
		if f.Name == c.Author {
			fish = f
		}
	}
	if fish == nil {
		label := strings.Fields(c.Author + " ?")[0]
		fish = &Fish{
			Member: Member{Name: c.Author},
			Label:  render.Trunc(label, maxLabel),
			Color:  palette[len(t.Fish)%len(palette)],
			dir:    1,
			y:      topRow,
		}
		t.Fish = append(t.Fish, fish)
	}
	fish.Recent++
	fish.Total++
	if c.When.After(fish.Last) {
		fish.Last = c.When
	}
	if fish.Recent > 0 {
		t.useTotal = false
	}
	fish.burst = 10
	fish.bubbleIn = 0
	fish.flash = 2.5
	fish.retarget = 0
	t.keepInside(fish)
	return fish
}

// Draw paints the current moment.
func (t *Tank) Draw() *Canvas {
	c := NewCanvas(t.W, t.H)
	water := t.H - sandRows
	now := t.now()

	for y := range water {
		c.SetBG(y, waterBG[min(y*len(waterBG)/max(water, 1), len(waterBG)-1)])
	}
	c.SetBG(t.H-2, 180)
	c.SetBG(t.H-1, 137)

	// The surface, rolling.
	waves := []rune("~~-~^~-~~^-")
	shift := int(t.t * 3)
	var surface strings.Builder
	for x := range t.W {
		surface.WriteRune(waves[(x+shift)%len(waves)])
	}
	c.Put(0, 0, surface.String(), 159)

	// Seaweed, swaying.
	for _, w := range t.weeds {
		for i := range w.h {
			ch := "("
			if (int(t.t*2)+i+w.x)%2 == 0 {
				ch = ")"
			}
			color := 34
			if i%2 == 1 {
				color = 28
			}
			c.Put(w.x, water-1-i, ch, color)
		}
	}

	// Sand.
	c.Put(0, t.H-2, string(t.sand), 137)
	c.Put(0, t.H-1, strings.Repeat("▒", t.W), 94)

	for _, b := range t.bubbles {
		ch, color := ".", bubbleColor
		switch {
		case b.zzz && b.age > 1.5:
			ch, color = "Z", zzzColor
		case b.zzz:
			ch, color = "z", zzzColor
		case b.age > 1.4:
			ch = "O"
		case b.age > 0.6:
			ch = "o"
		}
		c.Put(b.x, int(b.y), ch, color)
	}

	for _, f := range t.Fish {
		color := f.Color
		if f.Asleep(now) && f.burst == 0 {
			color = sleepColor
		}
		if f.flash > 0 && int(t.t*8)%2 == 0 {
			color = eyeColor // blinking glow
		}
		x, y := int(f.x), int(f.y)
		art := Art(t.Tier(f), f.dir)
		for i, line := range art {
			c.Put(x, y+i, strings.ReplaceAll(line, "°", " "), color)
			if j := strings.Index(line, "°"); j >= 0 {
				c.Put(x+len([]rune(line[:j])), y+i, "°", eyeColor)
			}
		}
		fw, _ := t.size(f)
		lx := x + (fw-len([]rune(f.Label)))/2
		lx = min(max(lx, 0), max(t.W-len([]rune(f.Label)), 0))
		c.Put(lx, y+len(art), f.Label, color)
	}
	return c
}
