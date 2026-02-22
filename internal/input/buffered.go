package input

import (
	"fmt"
	"sync"

	"golang.org/x/sys/unix"
)

// bufPool pools read buffers to reduce per-file heap allocations.
// Buffers are stored as *[]byte so the pool can reuse the backing array
// even when the slice grows beyond its original capacity.
var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 64*1024) // 64KB initial capacity
		return &b
	},
}

// BufferedReader reads files using unix.Open with O_NOATIME and unix.Pread.
// Uses sync.Pool to reuse buffers across files, avoiding per-file heap allocation.
type BufferedReader struct{}

// NewBufferedReader creates a new BufferedReader.
func NewBufferedReader() *BufferedReader {
	return &BufferedReader{}
}

func (r *BufferedReader) Read(path string) (ReadResult, error) {
	fd, err := openFile(path)
	if err != nil {
		return ReadResult{}, fmt.Errorf("open %s: %w", path, err)
	}

	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		unix.Close(fd)
		return ReadResult{}, fmt.Errorf("stat %s: %w", path, err)
	}

	if stat.Size == 0 {
		unix.Close(fd)
		return ReadResult{Data: nil, Closer: noopCloser}, nil
	}

	return readBuffered(fd, stat.Size)
}

// readBuffered reads a file from an already-open fd into a pooled buffer.
// Takes ownership of fd — caller must not close it.
func readBuffered(fd int, size int64) (ReadResult, error) {
	// Get a pooled buffer and grow it to fit the file
	bp := bufPool.Get().(*[]byte)
	buf := *bp
	if cap(buf) < int(size) {
		buf = make([]byte, size)
	} else {
		buf = buf[:size]
	}

	// Read the entire file using pread (no seek state)
	var totalRead int
	for totalRead < int(size) {
		n, err := unix.Pread(fd, buf[totalRead:], int64(totalRead))
		if err != nil {
			unix.Close(fd)
			*bp = buf
			bufPool.Put(bp)
			return ReadResult{}, err
		}
		if n == 0 {
			break // EOF
		}
		totalRead += n
	}

	unix.Close(fd)

	return ReadResult{
		Data: buf[:totalRead],
		Closer: func() error {
			*bp = buf
			bufPool.Put(bp)
			return nil
		},
	}, nil
}
