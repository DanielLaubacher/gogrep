package output

import (
	"os"

	"golang.org/x/sys/unix"
)

// Writer writes formatted output to stdout, using writev for batching.
type Writer struct {
	fd int
}

// NewWriter creates a Writer that writes to stdout.
func NewWriter() *Writer {
	return &Writer{fd: int(os.Stdout.Fd())}
}

// Write writes the given bytes to stdout using writev for scatter-gather I/O.
func (w *Writer) Write(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	for len(data) > 0 {
		iovs := [][]byte{data}
		n, err := unix.Writev(w.fd, iovs)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

// OrderedWriter receives results from a channel and writes them in sequence order.
// This ensures output is deterministic even with parallel workers.
type OrderedWriter struct {
	writer    *Writer
	formatter Formatter
	multiFile bool
}

// NewOrderedWriter creates an OrderedWriter.
func NewOrderedWriter(w *Writer, f Formatter, multiFile bool) *OrderedWriter {
	return &OrderedWriter{
		writer:    w,
		formatter: f,
		multiFile: multiFile,
	}
}

// WriteOrdered consumes results from the channel, buffering out-of-order results
// and writing them in sequence-number order. Reuses a single format buffer
// across all writes to avoid per-file allocation.
func (ow *OrderedWriter) WriteOrdered(results <-chan Result, onMatch func()) {
	nextSeq := 1
	pending := make(map[int]Result)
	var buf []byte // reused across all writeResult calls

	for r := range results {
		if r.Err == nil && r.HasMatch() {
			if onMatch != nil {
				onMatch()
			}
		}

		if r.SeqNum == nextSeq {
			buf = ow.writeResult(buf, r)
			nextSeq++
			// Flush any consecutive pending results
			for {
				if p, ok := pending[nextSeq]; ok {
					buf = ow.writeResult(buf, p)
					delete(pending, nextSeq)
					nextSeq++
				} else {
					break
				}
			}
		} else {
			pending[r.SeqNum] = r
		}
	}
}

func (ow *OrderedWriter) writeResult(buf []byte, r Result) []byte {
	if r.Err != nil {
		if r.Closer != nil {
			r.Closer()
		}
		return buf
	}
	buf = ow.formatter.Format(buf[:0], r, ow.multiFile)
	if r.Closer != nil {
		r.Closer()
	}
	ow.writer.Write(buf)
	return buf
}
