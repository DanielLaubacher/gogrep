package output

import (
	"os"

	"golang.org/x/sys/unix"
)

// ANSI escape sequences for coloring. Raw codes avoid the overhead of lipgloss.Render().
var (
	ansiReset     = []byte("\x1b[0m")
	ansiMagenta   = []byte("\x1b[35m")   // filename
	ansiGreen     = []byte("\x1b[32m")   // line number
	ansiCyan      = []byte("\x1b[36m")   // separator
	ansiBoldRed   = []byte("\x1b[1;31m") // match highlight
)

// IsTerminal checks if the given file descriptor is a terminal using ioctl.
func IsTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), unix.TCGETS)
	return err == nil
}

// StdoutIsTerminal returns true if stdout is a terminal.
func StdoutIsTerminal() bool {
	return IsTerminal(os.Stdout.Fd())
}
