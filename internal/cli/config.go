package cli

import "fmt"

// ColorMode controls when colored output is used.
type ColorMode int

const (
	ColorAuto   ColorMode = iota // color when stdout is a terminal
	ColorAlways                  // always use color
	ColorNever                   // never use color
)

// Config holds all configuration for a gogrep search.
type Config struct {
	Patterns      []string
	Fixed         bool
	PCRE          bool
	IgnoreCase    bool
	Recursive     bool
	LineNumbers   bool
	CountOnly     bool
	Invert        bool
	FileNamesOnly bool
	ContextBefore int
	ContextAfter  int
	WatchMode     bool
	JSONOutput    bool
	Color         ColorMode
	Workers       int
	NoIgnore       bool
	Hidden         bool
	FollowSymlinks bool
	SmartCase      bool
	Globs          []string
	MaxColumns     int
	MmapThreshold  int64
	Paths          []string
}

// Validate checks that the config is valid and returns an error if not.
func (c *Config) Validate() error {
	if len(c.Patterns) == 0 {
		return fmt.Errorf("no pattern specified")
	}
	if c.Fixed && c.PCRE {
		return fmt.Errorf("cannot use -F (fixed) and -P (pcre) together")
	}
	if c.ContextBefore < 0 {
		return fmt.Errorf("invalid context before: %d", c.ContextBefore)
	}
	if c.ContextAfter < 0 {
		return fmt.Errorf("invalid context after: %d", c.ContextAfter)
	}
	if c.CountOnly && c.FileNamesOnly {
		return fmt.Errorf("cannot use -c (count) and -l (files-with-matches) together")
	}
	return nil
}
