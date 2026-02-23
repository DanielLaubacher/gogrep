package output

import (
	"strconv"

	"github.com/dl/gogrep/internal/matcher"
)

// separatorLine is the shared "--" separator text for context groups.
var separatorLine = []byte("--")

// TextFormatter formats results as human-readable text with optional color.
type TextFormatter struct {
	lineNumbers bool
	countOnly   bool
	filesOnly   bool
	useColor    bool
	maxColumns  int
}

// NewTextFormatter creates a TextFormatter.
func NewTextFormatter(lineNumbers bool, countOnly bool, filesOnly bool, useColor bool, maxColumns int) *TextFormatter {
	return &TextFormatter{
		lineNumbers: lineNumbers,
		countOnly:   countOnly,
		filesOnly:   filesOnly,
		useColor:    useColor,
		maxColumns:  maxColumns,
	}
}

func (f *TextFormatter) Format(buf []byte, result Result, multiFile bool) []byte {
	if f.filesOnly {
		if result.HasMatch() {
			buf = append(buf, result.FilePath...)
			buf = append(buf, '\n')
			return buf
		}
		return buf
	}

	if f.countOnly {
		if multiFile {
			buf = append(buf, result.FilePath...)
			buf = append(buf, ':')
		}
		buf = strconv.AppendInt(buf, int64(result.Count()), 10)
		buf = append(buf, '\n')
		return buf
	}

	ms := &result.MatchSet
	for i := range ms.Matches {
		buf = f.formatMatch(buf, result.FilePath, ms, i, multiFile)
	}
	return buf
}

func (f *TextFormatter) formatMatch(buf []byte, filePath string, ms *matcher.MatchSet, idx int, multiFile bool) []byte {
	m := &ms.Matches[idx]

	// Resolve line bytes: separator sentinel or normal line
	var lineBytes []byte
	if m.LineStart < 0 {
		lineBytes = separatorLine
	} else {
		lineBytes = ms.Data[m.LineStart : m.LineStart+m.LineLen]
	}
	positions := ms.MatchPositions(idx)

	sep := ":"
	if m.IsContext {
		sep = "-"
	}

	// Filename prefix
	if multiFile {
		if f.useColor {
			buf = append(buf, ansiMagenta...)
			buf = append(buf, filePath...)
			buf = append(buf, ansiReset...)
			buf = append(buf, ansiCyan...)
			buf = append(buf, sep...)
			buf = append(buf, ansiReset...)
		} else {
			buf = append(buf, filePath...)
			buf = append(buf, sep...)
		}
	}

	// Line number
	if f.lineNumbers {
		if f.useColor {
			buf = append(buf, ansiGreen...)
			buf = strconv.AppendInt(buf, int64(m.LineNum), 10)
			buf = append(buf, ansiReset...)
			buf = append(buf, ansiCyan...)
			buf = append(buf, sep...)
			buf = append(buf, ansiReset...)
		} else {
			buf = strconv.AppendInt(buf, int64(m.LineNum), 10)
			buf = append(buf, sep...)
		}
	}

	// Truncate line content if needed, centering around the first match
	if f.maxColumns > 0 && len(lineBytes) > f.maxColumns {
		winStart, winEnd := truncateWindow(lineBytes, positions, f.maxColumns)
		lineBytes = lineBytes[winStart:winEnd]
		// Shift positions into the window and clip
		var clipped [][2]int
		for _, pos := range positions {
			s := pos[0] - winStart
			e := pos[1] - winStart
			if e <= 0 {
				continue
			}
			if s >= len(lineBytes) {
				break
			}
			if s < 0 {
				s = 0
			}
			if e > len(lineBytes) {
				e = len(lineBytes)
			}
			clipped = append(clipped, [2]int{s, e})
		}
		positions = clipped
	}

	// Line content with match highlighting
	if f.useColor && len(positions) > 0 {
		buf = f.highlightMatches(buf, lineBytes, positions)
	} else {
		buf = append(buf, lineBytes...)
	}
	buf = append(buf, '\n')
	return buf
}

// truncateWindow computes a [start, end) byte window of maxCols bytes
// centered on the first match position.
func truncateWindow(line []byte, positions [][2]int, maxCols int) (int, int) {
	center := 0
	if len(positions) > 0 {
		center = (positions[0][0] + positions[0][1]) / 2
	}

	start := center - maxCols/2
	if start < 0 {
		start = 0
	}
	end := start + maxCols
	if end > len(line) {
		end = len(line)
		start = end - maxCols
		if start < 0 {
			start = 0
		}
	}
	return start, end
}

func (f *TextFormatter) highlightMatches(buf []byte, line []byte, positions [][2]int) []byte {
	prev := 0
	for _, pos := range positions {
		start, end := pos[0], pos[1]
		if start > len(line) {
			break
		}
		if end > len(line) {
			end = len(line)
		}
		if start > prev {
			buf = append(buf, line[prev:start]...)
		}
		buf = append(buf, ansiBoldRed...)
		buf = append(buf, line[start:end]...)
		buf = append(buf, ansiReset...)
		prev = end
	}
	if prev < len(line) {
		buf = append(buf, line[prev:]...)
	}
	return buf
}
