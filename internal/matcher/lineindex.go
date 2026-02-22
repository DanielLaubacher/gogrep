package matcher

import "bytes"

// lineCursor tracks position while scanning forward through data for line boundaries.
// Offsets must be processed in sorted (ascending) order.
// For nearby advances, walks line-by-line. For large gaps, jumps directly
// to the target position using backward/forward scans + newline counting.
type lineCursor struct {
	data      []byte
	lineNum   int // 1-based line number at lineStart
	lineStart int // byte offset of current line start
	lineEnd   int // byte offset of current line end (position of \n, or len(data))
}

// newlineByte avoids allocating []byte{'\n'} on every call to bytes.Count.
var newlineByte = []byte{'\n'}

// newLineCursor initializes a cursor at the beginning of data.
func newLineCursor(data []byte) lineCursor {
	end := len(data)
	i := bytes.IndexByte(data, '\n')
	if i >= 0 {
		end = i
	}
	return lineCursor{
		data:      data,
		lineNum:   1,
		lineStart: 0,
		lineEnd:   end,
	}
}

// lineFromPos advances the cursor to the line containing pos and returns line info.
// pos must be >= the pos from the previous call (sorted order).
func (c *lineCursor) lineFromPos(pos int) ([]byte, int64, int) {
	// Already on the right line?
	if pos < c.lineEnd {
		return c.data[c.lineStart:c.lineEnd], int64(c.lineStart), c.lineNum
	}

	// If the gap is small, walk line by line (avoids overhead of Count + LastIndexByte).
	// Threshold: if pos is within ~256 bytes, walking is cheaper than jumping.
	if pos-c.lineEnd <= 256 {
		for pos >= c.lineEnd && c.lineEnd < len(c.data) {
			c.lineStart = c.lineEnd + 1
			c.lineNum++
			i := bytes.IndexByte(c.data[c.lineStart:], '\n')
			if i >= 0 {
				c.lineEnd = c.lineStart + i
			} else {
				c.lineEnd = len(c.data)
			}
		}
		return c.data[c.lineStart:c.lineEnd], int64(c.lineStart), c.lineNum
	}

	// Large gap: jump directly to pos.
	// Count newlines in the skipped region to update line number.
	// Find line start by scanning backward from pos.
	// Find line end by scanning forward from pos.
	gapStart := c.lineEnd // start counting from current line end
	nlCount := bytes.Count(c.data[gapStart:pos], newlineByte)
	c.lineNum += nlCount

	// Find line start: last \n before pos
	start := 0
	if pos > 0 {
		i := bytes.LastIndexByte(c.data[gapStart:pos], '\n')
		if i >= 0 {
			start = gapStart + i + 1
		} else {
			// No newline between gapStart and pos — still on same line
			// (but we already counted nlCount which should be 0 here)
			start = c.lineStart
		}
	}

	// Find line end: next \n at or after pos
	end := len(c.data)
	i := bytes.IndexByte(c.data[pos:], '\n')
	if i >= 0 {
		end = pos + i
	}

	c.lineStart = start
	c.lineEnd = end

	return c.data[c.lineStart:c.lineEnd], int64(c.lineStart), c.lineNum
}
