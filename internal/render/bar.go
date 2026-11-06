package render

import (
	"math"
	"strings"
)

// eighths are the partial blocks used for the last cell of a bar.
var eighths = []rune("▏▎▍▌▋▊▉")

// Bar draws value/maxValue as a bar up to width cells long, with 1/8-cell
// precision: ███████▍
func Bar(value, maxValue float64, width int) string {
	if maxValue <= 0 || value <= 0 || width <= 0 {
		return ""
	}
	n := int(math.Round(math.Min(value/maxValue, 1) * float64(width*8)))
	if n < 1 {
		n = 1 // anything above zero gets at least a sliver
	}
	s := strings.Repeat("█", n/8)
	if rem := n % 8; rem > 0 {
		s += string(eighths[rem-1])
	}
	return s
}
