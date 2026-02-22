package input

// ReadResult holds the data read from a file and a cleanup function.
type ReadResult struct {
	Data   []byte
	Closer func() error
}

// noopCloser is a package-level no-op closer to avoid allocating a func literal per file.
func noopCloser() error { return nil }

// Reader reads file content into a byte slice.
type Reader interface {
	Read(path string) (ReadResult, error)
}

// FdReader reads from a pre-opened file descriptor with known size.
// This avoids redundant open+fstat syscalls when the walker has already
// opened the file with openat and knows its size.
type FdReader interface {
	ReadFromFd(fd int, size int64) (ReadResult, error)
}
