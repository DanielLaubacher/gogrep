package output

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/sys/unix"
)

// Styles holds the lipgloss styles for output formatting.
type Styles struct {
	Filename  lipgloss.Style
	LineNum   lipgloss.Style
	Separator lipgloss.Style
	Match     lipgloss.Style
	Context   lipgloss.Style
}

// NewStyles creates the default color styles.
func NewStyles() Styles {
	return Styles{
		Filename:  lipgloss.NewStyle().Foreground(lipgloss.Color("5")),  // magenta
		LineNum:   lipgloss.NewStyle().Foreground(lipgloss.Color("2")),  // green
		Separator: lipgloss.NewStyle().Foreground(lipgloss.Color("6")), // cyan
		Match:     lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true), // bold red
		Context:   lipgloss.NewStyle(),
	}
}

// NoStyles returns styles with no coloring.
func NoStyles() Styles {
	return Styles{
		Filename:  lipgloss.NewStyle(),
		LineNum:   lipgloss.NewStyle(),
		Separator: lipgloss.NewStyle(),
		Match:     lipgloss.NewStyle(),
		Context:   lipgloss.NewStyle(),
	}
}

// IsTerminal checks if the given file descriptor is a terminal using ioctl.
func IsTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), unix.TCGETS)
	return err == nil
}

// StdoutIsTerminal returns true if stdout is a terminal.
func StdoutIsTerminal() bool {
	return IsTerminal(os.Stdout.Fd())
}
