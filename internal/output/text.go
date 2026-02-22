package output

import (
	"fmt"
	"strconv"

	"github.com/dl/gogrep/internal/matcher"
)

// TextFormatter formats results as human-readable text with optional color.
type TextFormatter struct {
	styles      Styles
	lineNumbers bool
	countOnly   bool
	filesOnly   bool
	useColor    bool
}

// NewTextFormatter creates a TextFormatter.
func NewTextFormatter(styles Styles, lineNumbers bool, countOnly bool, filesOnly bool, useColor bool) *TextFormatter {
	return &TextFormatter{
		styles:      styles,
		lineNumbers: lineNumbers,
		countOnly:   countOnly,
		filesOnly:   filesOnly,
		useColor:    useColor,
	}
}

func (f *TextFormatter) Format(result Result, multiFile bool) []byte {
	if f.filesOnly {
		if len(result.Matches) > 0 {
			return append([]byte(result.FilePath), '\n')
		}
		return nil
	}

	if f.countOnly {
		if multiFile {
			return []byte(fmt.Sprintf("%s:%d\n", result.FilePath, len(result.Matches)))
		}
		return []byte(strconv.Itoa(len(result.Matches)) + "\n")
	}

	var buf []byte
	for _, m := range result.Matches {
		buf = f.formatLine(buf, result.FilePath, m, multiFile)
	}
	return buf
}

func (f *TextFormatter) formatLine(buf []byte, filePath string, m matcher.Match, multiFile bool) []byte {
	// Filename prefix
	if multiFile {
		if f.useColor {
			buf = append(buf, f.styles.Filename.Render(filePath)...)
		} else {
			buf = append(buf, filePath...)
		}
		sep := ":"
		if m.IsContext {
			sep = "-"
		}
		if f.useColor {
			buf = append(buf, f.styles.Separator.Render(sep)...)
		} else {
			buf = append(buf, sep...)
		}
	}

	// Line number
	if f.lineNumbers {
		numStr := strconv.Itoa(m.LineNum)
		if f.useColor {
			buf = append(buf, f.styles.LineNum.Render(numStr)...)
		} else {
			buf = append(buf, numStr...)
		}
		sep := ":"
		if m.IsContext {
			sep = "-"
		}
		if f.useColor {
			buf = append(buf, f.styles.Separator.Render(sep)...)
		} else {
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
		buf = append(buf, f.styles.Match.Render(string(line[start:end]))...)
		prev = end
	}
	if prev < len(line) {
		buf = append(buf, line[prev:]...)
	}
	return buf
}
