package input

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/unix"
)

// MmapReader reads files by memory-mapping them with aggressive Linux kernel hints.
type MmapReader struct{}

// NewMmapReader creates a new MmapReader.
func NewMmapReader() *MmapReader {
	return &MmapReader{}
}

// readMmap memory-maps an already-opened fd of known size.
func readMmap(fd int, size int64, path string) (ReadResult, error) {
	// Hint kernel: sequential read pattern
	unix.Fadvise(fd, 0, size, unix.FADV_SEQUENTIAL)

	// Memory-map the file with MAP_POPULATE to prefault pages
	data, err := syscall.Mmap(fd, 0, int(size), syscall.PROT_READ, syscall.MAP_PRIVATE|syscall.MAP_POPULATE)
	if err != nil {
		// Fall back to buffered read from the already-open fd
		return readBuffered(fd, size)
	}

	// Additional hint: sequential access pattern
	unix.Madvise(data, unix.MADV_SEQUENTIAL)

	return ReadResult{
		Data: data,
		Closer: func() error {
			unix.Madvise(data, unix.MADV_DONTNEED)
			syscall.Munmap(data)
			unix.Close(fd)
			return nil
		},
	}, nil
}

func (r *MmapReader) Read(path string) (ReadResult, error) {
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

	return readMmap(fd, stat.Size, path)
}

// NewAdaptiveReader returns a Reader that opens the file once, stats it via fstat
// (no path-based stat), then selects between buffered and mmap based on size.
// This eliminates the redundant unix.Stat + ByteSliceFromString allocation.
func NewAdaptiveReader(mmapThreshold int64) Reader {
	return &adaptiveReader{
		threshold: mmapThreshold,
	}
}

type adaptiveReader struct {
	threshold int64
}

func (r *adaptiveReader) Read(path string) (ReadResult, error) {
	// Single open, single fstat — no redundant Stat(path) allocation
	fd, err := openFile(path)
	if err != nil {
		return ReadResult{}, fmt.Errorf("open %s: %w", path, err)
	}

	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		unix.Close(fd)
		return ReadResult{}, fmt.Errorf("stat %s: %w", path, err)
	}

	size := stat.Size
	if size == 0 {
		unix.Close(fd)
		return ReadResult{Data: nil, Closer: noopCloser}, nil
	}

	if size >= r.threshold {
		return readMmap(fd, size, path)
	}
	return readBuffered(fd, size)
}

// ReadFromFd reads from a pre-opened fd with known size.
// Skips open and fstat — the walker already did both via openat.
// Takes ownership of fd: closes it on error or via ReadResult.Closer.
func (r *adaptiveReader) ReadFromFd(fd int, size int64) (ReadResult, error) {
	if size == 0 {
		unix.Close(fd)
		return ReadResult{Data: nil, Closer: noopCloser}, nil
	}
	if size >= r.threshold {
		return readMmap(fd, size, "")
	}
	return readBuffered(fd, size)
}

// openFile opens a file with O_NOATIME, falling back without it.
func openFile(path string) (int, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOATIME, 0)
	if err != nil {
		fd, err = unix.Open(path, unix.O_RDONLY, 0)
	}
	return fd, err
}
