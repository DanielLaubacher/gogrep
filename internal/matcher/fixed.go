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

func (m *FixedMatcher) MatchExists(data []byte) bool {
	if m.invert {
		return len(data) > 0
	}
	if m.ignoreCase {
		return bytes.Contains(bytes.ToLower(data), m.patternLow)
	}
	return bytes.Contains(data, m.pattern)
}

func (m *FixedMatcher) CountAll(data []byte) int {
	count := 0
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
		_, ok := m.findInLine(line, 0, 0)
		if ok {
			count++
		}
	}
	return count
}

func (m *FixedMatcher) FindAll(data []byte) MatchSet {
	ms := MatchSet{Data: data}
	var offset int64
	lineNum := 1
	remaining := data

	for len(remaining) > 0 {
		idx := bytes.IndexByte(remaining, '\n')
		var lineLen int
		if idx >= 0 {
			lineLen = idx
		} else {
			lineLen = len(remaining)
		}
		lineStart := int(offset)
		line := remaining[:lineLen]

		lineMS, ok := m.findInLine(line, lineNum, offset)
		if ok {
			// Re-base positions into the shared positions array
			posIdx := len(ms.Positions)
			innerPositions := lineMS.MatchPositions(0)
			ms.Positions = append(ms.Positions, innerPositions...)

			ms.Matches = append(ms.Matches, Match{
				LineNum:    lineNum,
				LineStart:  lineStart,
				LineLen:    lineLen,
				ByteOffset: offset,
				PosIdx:     posIdx,
				PosCount:   len(innerPositions),
			})
		}

		if idx >= 0 {
			remaining = remaining[idx+1:]
		} else {
			remaining = nil
		}
		offset += int64(lineLen) + 1
		lineNum++
	}

	return ms
}

func (m *FixedMatcher) FindLine(line []byte, lineNum int, byteOffset int64) (MatchSet, bool) {
	return m.findInLine(line, lineNum, byteOffset)
}

func (m *FixedMatcher) findInLine(line []byte, lineNum int, byteOffset int64) (MatchSet, bool) {
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
		return MatchSet{}, false
	}

	ms := MatchSet{Data: line}
	match := Match{
		LineNum:    lineNum,
		LineStart:  0,
		LineLen:    len(line),
		ByteOffset: byteOffset,
	}
	if !m.invert {
		match.PosIdx = 0
		match.PosCount = len(positions)
		ms.Positions = positions
	}
	ms.Matches = []Match{match}

	return ms, true
}
