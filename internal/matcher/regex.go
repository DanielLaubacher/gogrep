package matcher

import (
	"bytes"
	"regexp"

	"github.com/dl/gogrep/internal/simd"
)

// RegexMatcher uses Go's RE2 regexp engine with optional SIMD literal prefiltering.
// When a required literal substring is extracted from the regex AST, the matcher
// first scans the buffer with SIMD for literal candidates, then only runs the
// regex engine on candidate lines.
type RegexMatcher struct {
	re           *regexp.Regexp
	invert       bool
	maxCols      int
	needLineNums bool
	prefilter    []byte // extracted literal for SIMD prefilter (nil = no prefilter)
	prefilterCI  bool   // use case-insensitive SIMD scan
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

	m := &RegexMatcher{re: re, invert: invert}

	// Extract a literal prefilter from the regex AST.
	// Invert mode checks every line, so prefilter doesn't help.
	if !invert {
		if info, ok := extractLiteral(pattern, ignoreCase); ok {
			m.prefilter = []byte(info.literal)
			m.prefilterCI = info.ignoreCase
		}
	}

	return m, nil
}

func (m *RegexMatcher) hasPrefilter() bool {
	return len(m.prefilter) > 0
}

func (m *RegexMatcher) MatchExists(data []byte) bool {
	if m.invert {
		return len(data) > 0
	}

	if !m.hasPrefilter() {
		return m.re.Match(data)
	}

	// SIMD scan for literal candidates one at a time, verify with regex.
	off := 0
	for off < len(data) {
		var idx int
		if m.prefilterCI {
			idx = simd.IndexCaseInsensitive(data[off:], m.prefilter)
		} else {
			idx = simd.Index(data[off:], m.prefilter)
		}
		if idx < 0 {
			return false
		}

		absOff := off + idx

		// Find containing line boundaries.
		lineStart := 0
		if absOff > 0 {
			if i := bytes.LastIndexByte(data[:absOff], '\n'); i >= 0 {
				lineStart = i + 1
			}
		}
		lineEnd := len(data)
		if i := bytes.IndexByte(data[absOff:], '\n'); i >= 0 {
			lineEnd = absOff + i
		}

		if m.re.Match(data[lineStart:lineEnd]) {
			return true
		}

		// Advance past this line.
		if lineEnd >= len(data) {
			return false
		}
		off = lineEnd + 1
	}
	return false
}

func (m *RegexMatcher) CountAll(data []byte) int {
	if m.invert {
		return countInvert(data, func(line []byte) bool {
			return !m.re.Match(line)
		})
	}

	if !m.hasPrefilter() {
		return countLocsUniqueLines(data, toLocs2(m.re.FindAllIndex(data, -1)))
	}

	// SIMD prefilter: find literal candidates, deduplicate by line, regex-verify.
	var offsets []int
	if m.prefilterCI {
		offsets = simd.IndexAllCaseInsensitive(data, m.prefilter)
	} else {
		offsets = simd.IndexAll(data, m.prefilter)
	}
	if len(offsets) == 0 {
		return 0
	}

	count := 0
	lastLineEnd := -1

	for _, off := range offsets {
		lineStart := 0
		if off > 0 {
			if i := bytes.LastIndexByte(data[:off], '\n'); i >= 0 {
				lineStart = i + 1
			}
		}

		if lineStart <= lastLineEnd {
			continue // same line as previous candidate
		}

		lineEnd := len(data)
		if i := bytes.IndexByte(data[off:], '\n'); i >= 0 {
			lineEnd = off + i
		}
		lastLineEnd = lineEnd

		if m.re.Match(data[lineStart:lineEnd]) {
			count++
		}
	}

	return count
}

func (m *RegexMatcher) FindAll(data []byte) MatchSet {
	if m.invert {
		return m.findAllInvert(data)
	}

	if !m.hasPrefilter() {
		locs := toLocs2(m.re.FindAllIndex(data, -1))
		if len(locs) == 0 {
			return MatchSet{}
		}
		return matchSetFromLocs(data, locs, m.maxCols, m.needLineNums)
	}

	return m.findAllPrefiltered(data)
}

// findAllPrefiltered scans the buffer with SIMD for literal candidates,
// extracts candidate lines, runs the regex on each, and collects results.
func (m *RegexMatcher) findAllPrefiltered(data []byte) MatchSet {
	// Step 1: SIMD scan for all literal occurrences.
	var offsets []int
	if m.prefilterCI {
		offsets = simd.IndexAllCaseInsensitive(data, m.prefilter)
	} else {
		offsets = simd.IndexAll(data, m.prefilter)
	}
	if len(offsets) == 0 {
		return MatchSet{}
	}

	// Step 2: Convert offsets to candidate lines, deduplicated.
	// Step 3: Run regex on each candidate line, collect buffer-absolute locs.
	var allLocs [][2]int
	lastLineEnd := -1

	for _, off := range offsets {
		// Find line start.
		lineStart := 0
		if off > 0 {
			if i := bytes.LastIndexByte(data[:off], '\n'); i >= 0 {
				lineStart = i + 1
			}
		}

		// Deduplicate: skip if same line as previous candidate.
		if lineStart <= lastLineEnd {
			continue
		}

		// Find line end.
		lineEnd := len(data)
		if i := bytes.IndexByte(data[off:], '\n'); i >= 0 {
			lineEnd = off + i
		}
		lastLineEnd = lineEnd

		// Run regex on this candidate line.
		line := data[lineStart:lineEnd]
		lineLocs := m.re.FindAllIndex(line, -1)
		for _, loc := range lineLocs {
			allLocs = append(allLocs, [2]int{lineStart + loc[0], lineStart + loc[1]})
		}
	}

	if len(allLocs) == 0 {
		return MatchSet{}
	}

	return matchSetFromLocs(data, allLocs, m.maxCols, m.needLineNums)
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
