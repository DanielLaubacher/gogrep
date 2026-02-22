package output

import (
	"strconv"

	"github.com/dl/gogrep/internal/matcher"
)

// TextFormatter formats results as human-readable text with optional color.
type TextFormatter struct {
	lineNumbers bool
	countOnly   bool
	filesOnly   bool
	useColor    bool
}

// NewTextFormatter creates a TextFormatter.
func NewTextFormatter(lineNumbers bool, countOnly bool, filesOnly bool, useColor bool) *TextFormatter {
	return &TextFormatter{
		lineNumbers: lineNumbers,
		countOnly:   countOnly,
		filesOnly:   filesOnly,
		useColor:    useColor,
	}
}

func (f *TextFormatter) Format(buf []byte, result Result, multiFile bool) []byte {
	if f.filesOnly {
		if len(result.Matches) > 0 {
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
		buf = strconv.AppendInt(buf, int64(len(result.Matches)), 10)
		buf = append(buf, '\n')
		return buf
	}

	for _, m := range result.Matches {
		buf = f.formatLine(buf, result.FilePath, m, multiFile)
	}
	return buf
}

func (f *TextFormatter) formatLine(buf []byte, filePath string, m matcher.Match, multiFile bool) []byte {
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

	// Line content with match highlighting
	if f.useColor && len(m.Positions) > 0 {
		buf = f.highlightMatches(buf, m.LineBytes, m.Positions)
	} else {
		buf = append(buf, m.LineBytes...)
	}

	buf = append(buf, '\n')
	return buf
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
