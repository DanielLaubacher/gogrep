package matcher

import (
	"bytes"
)

// FixedMatcher does literal string matching using bytes.Index.
type FixedMatcher struct {
	pattern    []byte
	patternLow []byte // lowercased pattern for case-insensitive
	ignoreCase bool
	invert     bool
}

// NewFixedMatcher creates a FixedMatcher for a single fixed pattern.
func NewFixedMatcher(pattern string, ignoreCase bool, invert bool) *FixedMatcher {
	p := []byte(pattern)
	var pLow []byte
	if ignoreCase {
		pLow = bytes.ToLower(p)
	}
	return &FixedMatcher{
		pattern:    p,
		patternLow: pLow,
		ignoreCase: ignoreCase,
		invert:     invert,
	}
}

func (m *FixedMatcher) FindAll(data []byte) []Match {
	var matches []Match
	var offset int64
	lineNum := 1

	for len(data) > 0 {
		idx := bytes.IndexByte(data, '\n')
		var line []byte
		if idx >= 0 {
			line = data[:idx]
			data = data[idx+1:]
		} else {
			line = data
			data = nil
		}

		match, ok := m.findInLine(line, lineNum, offset)
		if ok {
			matches = append(matches, match)
		}

		offset += int64(len(line)) + 1
		lineNum++
	}

	return matches
}

func (m *FixedMatcher) FindLine(line []byte, lineNum int, byteOffset int64) (Match, bool) {
	return m.findInLine(line, lineNum, byteOffset)
}

func (m *FixedMatcher) findInLine(line []byte, lineNum int, byteOffset int64) (Match, bool) {
	searchLine := line
	pattern := m.pattern
	if m.ignoreCase {
		searchLine = bytes.ToLower(line)
		pattern = m.patternLow
	}

	var positions [][2]int
	start := 0
	for start <= len(searchLine) {
		idx := bytes.Index(searchLine[start:], pattern)
		if idx < 0 {
			break
		}
		pos := start + idx
		positions = append(positions, [2]int{pos, pos + len(pattern)})
		start = pos + len(pattern)
		if len(pattern) == 0 {
			start++ // avoid infinite loop on empty pattern
		}
	}

	hasMatch := len(positions) > 0
	if m.invert {
		hasMatch = !hasMatch
	}

	if !hasMatch {
		return Match{}, false
	}

	match := Match{
		LineNum:    lineNum,
		LineBytes:  line,
		ByteOffset: byteOffset,
	}
	if !m.invert {
		match.Positions = positions
	}

	return match, true
}
