package render

import (
	"fmt"
	"io"
	"strings"
	"time"
)

var spinFrames = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

// Step runs fn while a spinner turns next to label, then prints the result:
//
//	⠿ Reading git history…   58 commits ✓
//
// Without animate (output is a file, or --no-anim) it only prints the result.
func Step(w io.Writer, animate bool, label string, fn func() (string, error)) error {
	stop := make(chan struct{})
	stopped := make(chan struct{})

	if animate {
		// A goroutine is a lightweight thread: `go f()` starts f and returns
		// immediately. This one redraws the spinner until we close stop.
		go func() {
			defer close(stopped)
			tick := time.NewTicker(80 * time.Millisecond)
			defer tick.Stop()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				case <-tick.C:
					// \r goes back to the start of the line; \x1b[K clears it.
					fmt.Fprintf(w, "\r\x1b[K %s %s", Cyan(string(spinFrames[i%len(spinFrames)])), label)
				}
			}
		}()
	} else {
		close(stopped)
	}

	detail, err := fn()
	close(stop)
	<-stopped // wait for the spinner to finish before printing over it

	if animate {
		fmt.Fprint(w, "\r\x1b[K")
	}
	if err != nil {
		fmt.Fprintf(w, " %s %s %s\n", Red("✗"), Pad(label, 24), Red(firstLine(err.Error())))
		return err
	}
	fmt.Fprintf(w, " %s %s %s %s\n", Dim("⠿"), Pad(label, 24), detail, Green("✓"))
	return nil
}

// Suspense prints text followed by dots that appear one by one, then erases
// the line. It's the drum roll before revealing an answer.
func Suspense(w io.Writer, animate bool, text string) {
	if !animate {
		return
	}
	fmt.Fprint(w, text)
	for range 3 {
		time.Sleep(260 * time.Millisecond)
		fmt.Fprint(w, ".")
	}
	time.Sleep(260 * time.Millisecond)
	fmt.Fprint(w, "\r\x1b[K")
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}
