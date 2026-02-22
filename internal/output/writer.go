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
// and writing them in sequence-number order.
func (ow *OrderedWriter) WriteOrdered(results <-chan Result, onMatch func()) {
	nextSeq := 1
	pending := make(map[int]Result)

	for r := range results {
		if r.Err == nil && len(r.Matches) > 0 {
			if onMatch != nil {
				onMatch()
			}
		}

		if r.SeqNum == nextSeq {
			ow.writeResult(r)
			nextSeq++
			// Flush any consecutive pending results
			for {
				if p, ok := pending[nextSeq]; ok {
					ow.writeResult(p)
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

func (ow *OrderedWriter) writeResult(r Result) {
	if r.Err != nil {
		return
	}
	data := ow.formatter.Format(r, ow.multiFile)
	ow.writer.Write(data)
}
