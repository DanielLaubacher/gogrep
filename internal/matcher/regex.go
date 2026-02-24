package matcher

import (
	"bytes"
	"regexp"
)

// RegexMatcher uses Go's RE2 regexp engine.
type RegexMatcher struct {
	re           *regexp.Regexp
	invert       bool
	maxCols      int
	needLineNums bool
}

// NewRegexMatcher creates a RegexMatcher for the given pattern.
func NewRegexMatcher(pattern string, ignoreCase bool, invert bool) (*RegexMatcher, error) {
	if ignoreCase {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &RegexMatcher{re: re, invert: invert}, nil
}

func (m *RegexMatcher) MatchExists(data []byte) bool {
	if m.invert {
		return len(data) > 0
	}
	return m.re.Match(data)
}

func (m *RegexMatcher) CountAll(data []byte) int {
	if m.invert {
		return countInvert(data, func(line []byte) bool {
			return !m.re.Match(line)
		})
	}

	return countLocsUniqueLines(data, toLocs2(m.re.FindAllIndex(data, -1)))
}

func (m *RegexMatcher) FindAll(data []byte) MatchSet {
	if m.invert {
		return m.findAllInvert(data)
	}

	locs := toLocs2(m.re.FindAllIndex(data, -1))
	if len(locs) == 0 {
		return MatchSet{}
	}

	return matchSetFromLocs(data, locs, m.maxCols, m.needLineNums)
}

func (m *RegexMatcher) findAllInvert(data []byte) MatchSet {
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

		if !m.re.Match(line) {
			ms.Matches = append(ms.Matches, Match{
				LineNum:    lineNum,
				LineStart:  lineStart,
				LineLen:    lineLen,
				ByteOffset: offset,
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

func (m *RegexMatcher) FindLine(line []byte, lineNum int, byteOffset int64) (MatchSet, bool) {
	locs := m.re.FindAllIndex(line, -1)
	hasMatch := len(locs) > 0

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
		match.PosCount = len(locs)
		ms.Positions = make([][2]int, len(locs))
		for i, loc := range locs {
			ms.Positions[i] = [2]int{loc[0], loc[1]}
		}
	}
	ms.Matches = []Match{match}

	return ms, true
}
