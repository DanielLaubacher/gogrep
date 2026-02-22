package matcher

import "bytes"

// ContextMatcher wraps a Matcher and adds context lines (before/after).
type ContextMatcher struct {
	inner  Matcher
	before int
	after  int
}

// NewContextMatcher wraps an existing matcher to add context lines.
// If both before and after are 0, returns the inner matcher directly.
func NewContextMatcher(inner Matcher, before, after int) Matcher {
	if before == 0 && after == 0 {
		return inner
	}
	return &ContextMatcher{inner: inner, before: before, after: after}
}

func (m *ContextMatcher) MatchExists(data []byte) bool {
	return m.inner.MatchExists(data)
}

func (m *ContextMatcher) FindAll(data []byte) []Match {
	// First, split data into lines and find all matching line numbers
	var lines [][]byte
	var offsets []int64
	var offset int64

	remaining := data
	for len(remaining) > 0 {
		idx := bytes.IndexByte(remaining, '\n')
		var line []byte
		if idx >= 0 {
			line = remaining[:idx]
			remaining = remaining[idx+1:]
		} else {
			line = remaining
			remaining = nil
		}
		lines = append(lines, line)
		offsets = append(offsets, offset)
		offset += int64(len(line)) + 1
	}

	// Find which lines match
	matchSet := make(map[int]Match) // line index -> Match
	for i, line := range lines {
		match, ok := m.inner.FindLine(line, i+1, offsets[i])
		if ok {
			matchSet[i] = match
		}
	}

	if len(matchSet) == 0 {
		return nil
	}

	// Determine which lines to include (matches + context)
	include := make(map[int]bool)
	for idx := range matchSet {
		for i := idx - m.before; i <= idx+m.after; i++ {
			if i >= 0 && i < len(lines) {
				include[i] = true
			}
		}
	}

	// Build result in order, inserting group separators
	var result []Match
	lastIncluded := -2 // sentinel

	for i := 0; i < len(lines); i++ {
		if !include[i] {
			continue
		}

		// Insert separator between non-contiguous groups
		if lastIncluded >= 0 && i > lastIncluded+1 && len(result) > 0 {
			result = append(result, Match{
				LineNum:   0, // sentinel for separator
				LineBytes: []byte("--"),
				IsContext: true,
			})
		}

		if match, isMatch := matchSet[i]; isMatch {
			result = append(result, match)
		} else {
			// Context line
			result = append(result, Match{
				LineNum:    i + 1,
				LineBytes:  lines[i],
				ByteOffset: offsets[i],
				IsContext:  true,
			})
		}

		lastIncluded = i
	}

	return result
}

func (m *ContextMatcher) FindLine(line []byte, lineNum int, byteOffset int64) (Match, bool) {
	return m.inner.FindLine(line, lineNum, byteOffset)
}
