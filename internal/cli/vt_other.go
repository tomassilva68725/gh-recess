//go:build !windows

package cli

import "os"

// enableVT: terminals on Linux and macOS understand ANSI codes out of the box.
func enableVT(*os.File) bool { return true }
