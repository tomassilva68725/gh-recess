//go:build windows

package cli

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableVT asks the Windows console to understand ANSI escape codes (colors,
// moving the cursor). Windows Terminal already does; the old console window
// needs to be told. Returns false when the console can't do it.
func enableVT(f *os.File) bool {
	h := windows.Handle(f.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return false
	}
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
		return true
	}
	return windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) == nil
}
