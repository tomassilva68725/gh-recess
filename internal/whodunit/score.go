package whodunit

import "time"

// Player is one detective and their running score.
type Player struct {
	Name    string
	Score   int
	Correct int
	Guesses int
	Streak  int // correct answers in a row right now
	Best    int // longest streak so far
}

// Record scores one guess and returns the points it earned.
// timed is false in party mode, where only one person types.
func (p *Player) Record(correct bool, elapsed time.Duration, timed bool) int {
	p.Guesses++
	if !correct {
		p.Streak = 0
		return 0
	}
	p.Correct++
	p.Streak++
	p.Best = max(p.Best, p.Streak)
	pts := Points(elapsed, p.Streak, timed)
	p.Score += pts
	return pts
}

const (
	basePoints  = 100
	speedPoints = 50               // the most a fast answer adds
	speedWindow = 20 * time.Second // after this, no speed bonus
	streakBonus = 25               // per correct answer in a row, after the first
	maxStreak   = 4                // the streak bonus stops growing here
)

// Points is what a correct answer is worth: 100, plus up to 50 for answering
// fast, plus 25 for every answer in a row after the first (up to +100).
func Points(elapsed time.Duration, streak int, timed bool) int {
	pts := basePoints
	if timed && elapsed < speedWindow {
		pts += int(float64(speedPoints) * float64(speedWindow-elapsed) / float64(speedWindow))
	}
	if streak > 1 {
		pts += streakBonus * min(streak-1, maxStreak)
	}
	return pts
}

// Rank is the title a detective earns at the end.
func Rank(correct, total int) string {
	if total == 0 {
		return "No-show"
	}
	switch pct := correct * 100 / total; {
	case pct == 100:
		return "Sherlock Holmes"
	case pct >= 75:
		return "Senior detective"
	case pct >= 50:
		return "Rookie cop"
	case pct >= 25:
		return "Confused witness"
	default:
		return "Prime suspect"
	}
}
