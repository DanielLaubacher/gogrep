package scheduler

import (
	"runtime"
	"sync"
	"sync/atomic"

	"golang.org/x/sys/unix"

	"github.com/dl/gogrep/internal/input"
	"github.com/dl/gogrep/internal/matcher"
	"github.com/dl/gogrep/internal/output"
	"github.com/dl/gogrep/internal/walker"
)

// Scheduler manages a pool of workers that search files concurrently.
type Scheduler struct {
	workers   int
	matcher   matcher.Matcher
	reader    input.Reader
	fdReader  input.FdReader // nil if reader doesn't support pre-opened fds
	filesOnly bool           // when true, use MatchExists for faster -l mode
}

// New creates a Scheduler with the given number of workers.
// If workers is 0, defaults to NumCPU * 2.
// If filesOnly is true, uses fast MatchExists instead of FindAll (for -l mode).
func New(workers int, m matcher.Matcher, r input.Reader, filesOnly bool) *Scheduler {
	if workers <= 0 {
		workers = runtime.NumCPU() * 2
	}
	s := &Scheduler{
		workers:   workers,
		matcher:   m,
		reader:    r,
		filesOnly: filesOnly,
	}
	if fr, ok := r.(input.FdReader); ok {
		s.fdReader = fr
	}
	return s
}

// Run processes files from the file channel and returns results on the result channel.
// Results include sequence numbers for ordered output.
func (s *Scheduler) Run(files <-chan walker.FileEntry) <-chan output.Result {
	resultCh := make(chan output.Result, s.workers*2)
	var seq atomic.Int64

	var wg sync.WaitGroup
	for range s.workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entry := range files {
				seqNum := int(seq.Add(1))
				result := s.processFile(entry)
				result.SeqNum = seqNum
				resultCh <- result
			}
		}()
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	return resultCh
}

func (s *Scheduler) processFile(entry walker.FileEntry) output.Result {
	result := output.Result{FilePath: entry.Path}

	var readResult input.ReadResult
	var err error
	if s.fdReader != nil && entry.Fd >= 0 {
		// Fast path: walker pre-opened the file with openat, skip open+fstat
		readResult, err = s.fdReader.ReadFromFd(entry.Fd, entry.Size)
	} else {
		if entry.Fd >= 0 {
			unix.Close(entry.Fd) // close unused pre-opened fd
		}
		readResult, err = s.reader.Read(entry.Path)
	}
	if err != nil {
		result.Err = err
		return result
	}
	defer func() {
		if readResult.Closer != nil {
			readResult.Closer()
		}
	}()

	if readResult.Data == nil {
		return result
	}

	// Binary detection: skip binary files entirely (like ripgrep)
	if walker.IsBinary(readResult.Data) {
		return result
	}

	if s.filesOnly {
		// Fast path: only check if any match exists, skip line boundary computation
		if s.matcher.MatchExists(readResult.Data) {
			result.Matches = []matcher.Match{{}} // sentinel: at least one match
		}
	} else {
		result.Matches = s.matcher.FindAll(readResult.Data)
	}
	return result
}
