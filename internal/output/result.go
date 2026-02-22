package output

import "github.com/dl/gogrep/internal/matcher"

// Result aggregates the matches found in a single file.
type Result struct {
	FilePath string
	SeqNum   int
	Matches  []matcher.Match
	Err      error
}
