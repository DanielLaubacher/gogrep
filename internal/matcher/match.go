package matcher

// Match represents a single matched (or context) line.
type Match struct {
	LineNum    int      // 1-based line number
	LineBytes  []byte   // full line content (no trailing newline)
	ByteOffset int64    // byte offset of line start within file
	Positions  [][2]int // start/end byte offsets of each match within LineBytes
	IsContext  bool     // true if this is a context line, not an actual match
}

// Matcher finds pattern matches in data.
type Matcher interface {
	// FindAll scans data (full file content) and returns all matches.
	FindAll(data []byte) []Match

	// MatchExists returns true if there is at least one match in data.
	// This is faster than FindAll when you only need to know if matches exist
	// (e.g., for -l / --files-with-matches mode).
	MatchExists(data []byte) bool

	// FindLine checks a single line for matches.
	// lineNum is 1-based, byteOffset is the offset of the line start in the file.
	FindLine(line []byte, lineNum int, byteOffset int64) (Match, bool)
}

// matchesFromOffsets converts a list of whole-buffer match offsets into Match structs,
// deduplicating multiple matches on the same line and merging their positions.
// patternLen is the length of the fixed pattern (for computing position end offsets).
// Uses a forward-scanning cursor — no backward scans, no pre-built index.
//
// Allocates a single flat positions buffer for all matches, then sub-slices it
// into each Match.Positions. This reduces per-match heap allocations from N to 1.
func matchesFromOffsets(data []byte, offsets []int, patternLen int) []Match {
	if len(offsets) == 0 {
		return nil
	}

	cursor := newLineCursor(data)
	matches := make([]Match, 0, len(offsets))
	// Single allocation for all position pairs.
	allPos := make([][2]int, 0, len(offsets))
	lastLineStart := int64(-1)
	posStart := 0 // start index of current match's positions in allPos

	for _, off := range offsets {
		line, lineOffset, lineNum := cursor.lineFromPos(off)

		posInLine := off - int(lineOffset)
		allPos = append(allPos, [2]int{posInLine, posInLine + patternLen})

		if lineOffset == lastLineStart {
			// Same line — extend previous match's positions via shared buffer
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
