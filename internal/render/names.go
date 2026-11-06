package render

import "strings"

// ShortNames maps each full name to its first word ("André Narquel" →
// "André"), for the names whose first word nobody else shares.
func ShortNames(names []string) map[string]string {
	first := func(n string) string {
		if f := strings.Fields(n); len(f) > 0 {
			return f[0]
		}
		return n
	}
	seen := map[string]bool{}
	count := map[string]int{}
	for _, n := range names {
		if n != "" && !seen[n] {
			seen[n] = true
			count[strings.ToLower(first(n))]++
		}
	}
	short := map[string]string{}
	for n := range seen {
		if count[strings.ToLower(first(n))] == 1 {
			short[n] = first(n)
		}
	}
	return short
}
