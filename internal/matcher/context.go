package matcher

import "bytes"

// separatorData is a shared backing buffer for "--" separator lines.
var separatorData = []byte("--")

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

func (m *ContextMatcher) CountAll(data []byte) int {
	return m.inner.CountAll(data)
}

func (m *ContextMatcher) FindAll(data []byte) MatchSet {
	// First, split data into lines and find all matching line numbers
	type lineInfo struct {
		start int
		len   int
	}
	var lines []lineInfo
	offset := 0
	remaining := data
	for len(remaining) > 0 {
		idx := bytes.IndexByte(remaining, '\n')
		var lineLen int
		if idx >= 0 {
			lineLen = idx
			remaining = remaining[idx+1:]
		} else {
			lineLen = len(remaining)
			remaining = nil
		}
		lines = append(lines, lineInfo{start: offset, len: lineLen})
		offset += lineLen + 1
	}

	// Find which lines match — store the MatchSet from FindLine for each
	type matchInfo struct {
		ms MatchSet
	}
	matchSet := make(map[int]matchInfo)
	for i, li := range lines {
		line := data[li.start : li.start+li.len]
		ms, ok := m.inner.FindLine(line, i+1, int64(li.start))
		if ok {
			matchSet[i] = matchInfo{ms: ms}
		}
	}

	if len(matchSet) == 0 {
		return MatchSet{}
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
	// All matches and context lines reference data, separators reference separatorData
	result := MatchSet{Data: data}
	lastIncluded := -2 // sentinel

	for i := 0; i < len(lines); i++ {
		if !include[i] {
			continue
		}

		// Insert separator between non-contiguous groups
		if lastIncluded >= 0 && i > lastIncluded+1 && len(result.Matches) > 0 {
			// Separator: LineNum=0, references separatorData indirectly.
			// We store negative LineStart as sentinel; the formatter checks IsContext+LineNum==0.
			// Actually, we need the separator text available. Since Data=data and "--" isn't in data,
			// we handle separators specially: LineStart=-1, LineLen=0 signals separator.
			result.Matches = append(result.Matches, Match{
				LineNum:   0,
				LineStart: -1, // sentinel for separator
				LineLen:   0,
				IsContext: true,
			})
		}

		if mi, isMatch := matchSet[i]; isMatch {
			// Copy match from inner FindLine result
			// The inner result has Data=line (sub-slice of data), positions relative to line start.
			// We need to re-base: positions stay the same (relative to line start),
			// but LineStart needs to reference our data buffer.
			li := lines[i]
			innerMatch := mi.ms.Matches[0]
			posIdx := len(result.Positions)
			innerPositions := mi.ms.MatchPositions(0)
			result.Positions = append(result.Positions, innerPositions...)

			result.Matches = append(result.Matches, Match{
				LineNum:    innerMatch.LineNum,
				LineStart:  li.start,
				LineLen:    li.len,
				ByteOffset: int64(li.start),
				PosIdx:     posIdx,
				PosCount:   len(innerPositions),
			})
		} else {
			// Context line
			li := lines[i]
			result.Matches = append(result.Matches, Match{
				LineNum:    i + 1,
				LineStart:  li.start,
				LineLen:    li.len,
				ByteOffset: int64(li.start),
				IsContext:  true,
			})
		}

		lastIncluded = i
	}

	return result
}

func (m *ContextMatcher) FindLine(line []byte, lineNum int, byteOffset int64) (MatchSet, bool) {
	return m.inner.FindLine(line, lineNum, byteOffset)
}
