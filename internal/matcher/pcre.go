package matcher

import (
	"bytes"

	"go.elara.ws/pcre"
)

// PCREMatcher matches using PCRE2-compatible regexes via the pure Go pcre package.
// Supports lookahead, lookbehind, backreferences, atomic groups, and all PCRE2 features.
type PCREMatcher struct {
	re         *pcre.Regexp
	ignoreCase bool
	invert     bool
}

// NewPCREMatcher creates a PCREMatcher from a PCRE2 pattern string.
func NewPCREMatcher(pattern string, ignoreCase bool, invert bool) (*PCREMatcher, error) {
	var opts pcre.CompileOption
	if ignoreCase {
		opts |= pcre.Caseless
	}

	re, err := pcre.CompileOpts(pattern, opts)
	if err != nil {
		return nil, err
	}

	return &PCREMatcher{
		re:         re,
		ignoreCase: ignoreCase,
		invert:     invert,
	}, nil
}

func (m *PCREMatcher) MatchExists(data []byte) bool {
	if m.invert {
		return len(data) > 0
	}
	return m.re.Match(data)
}

func (m *PCREMatcher) FindAll(data []byte) []Match {
	if m.invert {
		return m.findAllInvert(data)
	}

	// Search whole buffer at once
	locs := m.re.FindAllIndex(data, -1)
	if len(locs) == 0 {
		return nil
	}

	cursor := newLineCursor(data)
	matches := make([]Match, 0, len(locs))
	allPos := make([][2]int, 0, len(locs))
	lastLineStart := int64(-1)
	posStart := 0

	for _, loc := range locs {
		matchStart := loc[0]
		matchEnd := loc[1]

		line, lineOffset, lineNum := cursor.lineFromPos(matchStart)

		posInLine := matchStart - int(lineOffset)
		posEnd := matchEnd - int(lineOffset)
		lineLen := len(line)
		if posEnd > lineLen {
			posEnd = lineLen
		}

		allPos = append(allPos, [2]int{posInLine, posEnd})

		if lineOffset == lastLineStart {
			last := &matches[len(matches)-1]
			last.Positions = allPos[posStart:]
		} else {
			posStart = len(allPos) - 1
			matches = append(matches, Match{
				LineNum:    lineNum,
				LineBytes:  line,
				ByteOffset: lineOffset,
				Positions:  allPos[posStart:],
			})
			lastLineStart = lineOffset
		}
	}

	return matches
}

func (m *PCREMatcher) findAllInvert(data []byte) []Match {
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

		locs := m.re.FindAllIndex(line, -1)
		if len(locs) == 0 {
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

func (m *PCREMatcher) FindLine(line []byte, lineNum int, byteOffset int64) (Match, bool) {
	return m.findInLine(line, lineNum, byteOffset)
}

func (m *PCREMatcher) findInLine(line []byte, lineNum int, byteOffset int64) (Match, bool) {
	locs := m.re.FindAllIndex(line, -1)
	hasMatch := len(locs) > 0

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
	if !m.invert && len(locs) > 0 {
		match.Positions = make([][2]int, len(locs))
		for i, loc := range locs {
			match.Positions[i] = [2]int{loc[0], loc[1]}
		}
	}

	return match, true
}

// Close releases the compiled PCRE regex resources.
func (m *PCREMatcher) Close() {
	if m.re != nil {
		m.re.Close()
	}
}
