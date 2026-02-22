package matcher

import (
	"bytes"

	"github.com/dl/gogrep/internal/simd"
)

// BoyerMooreMatcher uses SIMD-accelerated fixed string matching for single patterns.
// Uses the SIMD-friendly Horspool algorithm for whole-buffer search.
type BoyerMooreMatcher struct {
	pattern    []byte
	patternLow []byte // lowered pattern for case-insensitive
	ignoreCase bool
	invert     bool
}

// NewBoyerMooreMatcher creates a BoyerMooreMatcher for a single fixed pattern.
func NewBoyerMooreMatcher(pattern string, ignoreCase bool, invert bool) *BoyerMooreMatcher {
	p := []byte(pattern)
	var pLow []byte
	if ignoreCase {
		pLow = bytes.ToLower(p)
	} else {
		pLow = p
	}

	return &BoyerMooreMatcher{
		pattern:    p,
		patternLow: pLow,
		ignoreCase: ignoreCase,
		invert:     invert,
	}
}

func (m *BoyerMooreMatcher) MatchExists(data []byte) bool {
	if m.invert {
		// For invert mode, almost any non-empty file will have non-matching lines.
		return len(data) > 0
	}
	if m.ignoreCase {
		return simd.IndexCaseInsensitive(data, m.patternLow) >= 0
	}
	return simd.Index(data, m.patternLow) >= 0
}

func (m *BoyerMooreMatcher) FindAll(data []byte) []Match {
	if m.invert {
		return m.findAllInvert(data)
	}

	// Use SIMD-accelerated whole-buffer search, then extract lines around matches
	var offsets []int
	if m.ignoreCase {
		offsets = simd.IndexAllCaseInsensitive(data, m.patternLow)
	} else {
		offsets = simd.IndexAll(data, m.patternLow)
	}
	return matchesFromOffsets(data, offsets, len(m.patternLow))
}

// findAllInvert returns lines that do NOT contain the pattern.
func (m *BoyerMooreMatcher) findAllInvert(data []byte) []Match {
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

		var found int
		if m.ignoreCase {
			found = simd.IndexCaseInsensitive(line, m.patternLow)
		} else {
			found = simd.Index(line, m.patternLow)
		}
		if found < 0 {
			matches = append(matches, Match{
				LineNum:    lineNum,
				LineBytes:  line,
				ByteOffset: offset,
			})
		}

		offset += int64(len(line)) + 1
		lineNum++
	}

	return matches
}

func (m *BoyerMooreMatcher) FindLine(line []byte, lineNum int, byteOffset int64) (Match, bool) {
	return m.findInLine(line, lineNum, byteOffset)
}

func (m *BoyerMooreMatcher) findInLine(line []byte, lineNum int, byteOffset int64) (Match, bool) {
	var offsets []int
	if m.ignoreCase {
		offsets = simd.IndexAllCaseInsensitive(line, m.patternLow)
	} else {
		offsets = simd.IndexAll(line, m.patternLow)
	}
	hasMatch := len(offsets) > 0

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
		pLen := len(m.patternLow)
		match.Positions = make([][2]int, len(offsets))
		for i, off := range offsets {
			match.Positions[i] = [2]int{off, off + pLen}
		}
	}

	return match, true
}

// toLower converts an ASCII byte to lowercase.
func toLower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}
