package walker

import (
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"
)

// FileEntry represents a file discovered during directory traversal.
type FileEntry struct {
	Path string
}

// WalkOptions configures directory traversal behavior.
type WalkOptions struct {
	Recursive bool
	NoIgnore  bool // skip .gitignore processing
	Hidden    bool // include hidden files and directories
}

// Walk traverses directories and sends discovered files on the returned channel.
// It uses raw getdents64 for maximum Linux performance.
// Respects .gitignore files and skips hidden files/directories by default.
// If recursive is false, only the given paths are used as literal file paths.
func Walk(roots []string, opts WalkOptions) (<-chan FileEntry, <-chan error) {
	fileCh := make(chan FileEntry, 256)
	errCh := make(chan error, 16)

	go func() {
		defer close(fileCh)
		defer close(errCh)

		if !opts.Recursive {
			for _, root := range roots {
				var stat unix.Stat_t
				if err := unix.Stat(root, &stat); err != nil {
					errCh <- &WalkError{Path: root, Err: err}
					continue
				}
				if stat.Mode&unix.S_IFMT == unix.S_IFREG {
					fileCh <- FileEntry{Path: root}
				}
			}
			return
		}

		pw := &parallelWalker{
			fileCh:   fileCh,
			errCh:    errCh,
			hidden:   opts.Hidden,
			noIgnore: opts.NoIgnore,
		}
		pw.cond = sync.NewCond(&pw.mu)

		// Seed work queue with root directories.
		for _, root := range roots {
			var layers []ignoreLayer
			if !opts.NoIgnore {
				layers = []ignoreLayer{loadIgnoreLayer(root)}
			}
			pw.enqueue(walkItem{path: root, ignores: layers})
		}

		// Launch parallel walker goroutines.
		workers := runtime.NumCPU()
		var wg sync.WaitGroup
		for range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				pw.worker()
			}()
		}
		wg.Wait()
	}()

	return fileCh, errCh
}

// walkItem represents a directory to be traversed by a worker.
type walkItem struct {
	path    string
	ignores []ignoreLayer // snapshot of parent's ignore layers (nil if --no-ignore)
}

// parallelWalker coordinates concurrent BFS directory traversal.
type parallelWalker struct {
	fileCh   chan<- FileEntry
	errCh    chan<- error
	hidden   bool
	noIgnore bool

	mu      sync.Mutex
	queue   []walkItem
	pending int        // dirs enqueued but not yet fully processed
	cond    *sync.Cond // signaled when items are enqueued or work is done
	done    bool
}

// enqueue adds a directory to the work queue.
func (pw *parallelWalker) enqueue(item walkItem) {
	pw.mu.Lock()
	pw.queue = append(pw.queue, item)
	pw.pending++
	pw.mu.Unlock()
	pw.cond.Signal()
}

// dequeue retrieves a work item, blocking if the queue is temporarily empty.
// Returns false when all work is complete.
func (pw *parallelWalker) dequeue() (walkItem, bool) {
	pw.mu.Lock()
	for len(pw.queue) == 0 && !pw.done {
		pw.cond.Wait()
	}
	if pw.done && len(pw.queue) == 0 {
		pw.mu.Unlock()
		return walkItem{}, false
	}
	item := pw.queue[0]
	pw.queue = pw.queue[1:]
	pw.mu.Unlock()
	return item, true
}

// finish marks a directory as fully processed.
func (pw *parallelWalker) finish() {
	pw.mu.Lock()
	pw.pending--
	if pw.pending == 0 && len(pw.queue) == 0 {
		pw.done = true
		pw.cond.Broadcast()
	}
	pw.mu.Unlock()
}

// worker processes directories from the work queue until all work is done.
func (pw *parallelWalker) worker() {
	buf := make([]byte, 32*1024) // per-worker getdents buffer
	var dirents []Dirent          // per-worker reusable dirent slice
	for {
		item, ok := pw.dequeue()
		if !ok {
			return
		}
		dirents = pw.processDir(item, buf, dirents)
		pw.finish()
	}
}

// processDir opens a single directory, reads all entries, and dispatches files/subdirs.
// The directory fd is closed before returning — not held during subtree traversal.
// Returns the dirents slice for reuse by the next call.
func (pw *parallelWalker) processDir(item walkItem, buf []byte, dirents []Dirent) []Dirent {
	fd, err := unix.Open(item.path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOATIME, 0)
	if err != nil {
		fd, err = unix.Open(item.path, unix.O_RDONLY|unix.O_DIRECTORY, 0)
		if err != nil {
			pw.errCh <- &WalkError{Path: item.path, Err: err}
			return dirents
		}
	}

	// Collect subdirectories to enqueue after closing the fd.
	var subdirs []walkItem

	for {
		n, err := unix.Getdents(fd, buf)
		if err != nil {
			pw.errCh <- &WalkError{Path: item.path, Err: err}
			break
		}
		if n == 0 {
			break
		}

		dirents = ParseDirents(buf, n, dirents)
		for _, entry := range dirents {
			fullPath := joinPath(item.path, entry.Name)

			switch entry.Type {
			case DT_DIR:
				if skipDir(entry.Name, pw.hidden) {
					continue
				}
				if item.ignores != nil && isIgnoredByLayers(item.ignores, fullPath, true) {
					continue
				}
				// Build child ignore layers: clone parent + load this dir's .gitignore
				var childIgnores []ignoreLayer
				if !pw.noIgnore {
					childIgnores = make([]ignoreLayer, len(item.ignores)+1)
					copy(childIgnores, item.ignores)
					childIgnores[len(item.ignores)] = loadIgnoreLayer(fullPath)
				}
				subdirs = append(subdirs, walkItem{path: fullPath, ignores: childIgnores})

			case DT_REG:
				if !pw.hidden && len(entry.Name) > 0 && entry.Name[0] == '.' {
					continue
				}
				if item.ignores != nil && isIgnoredByLayers(item.ignores, fullPath, false) {
					continue
				}
				pw.fileCh <- FileEntry{Path: fullPath}

			case DT_LNK:
				var stat unix.Stat_t
				if err := unix.Stat(fullPath, &stat); err != nil {
					continue // silently skip broken symlinks
				}
				if stat.Mode&unix.S_IFMT == unix.S_IFREG {
					if !pw.hidden && len(entry.Name) > 0 && entry.Name[0] == '.' {
						continue
					}
					if item.ignores != nil && isIgnoredByLayers(item.ignores, fullPath, false) {
						continue
					}
					pw.fileCh <- FileEntry{Path: fullPath}
				} else if stat.Mode&unix.S_IFMT == unix.S_IFDIR {
					if skipDir(entry.Name, pw.hidden) {
						continue
					}
					if item.ignores != nil && isIgnoredByLayers(item.ignores, fullPath, true) {
						continue
					}
					var childIgnores []ignoreLayer
					if !pw.noIgnore {
						childIgnores = make([]ignoreLayer, len(item.ignores)+1)
						copy(childIgnores, item.ignores)
						childIgnores[len(item.ignores)] = loadIgnoreLayer(fullPath)
					}
					subdirs = append(subdirs, walkItem{path: fullPath, ignores: childIgnores})
				}

			case DT_UNKNOWN:
				var stat unix.Stat_t
				if err := unix.Stat(fullPath, &stat); err != nil {
					pw.errCh <- &WalkError{Path: fullPath, Err: err}
					continue
				}
				mode := stat.Mode & unix.S_IFMT
				if mode == unix.S_IFREG {
					if !pw.hidden && len(entry.Name) > 0 && entry.Name[0] == '.' {
						continue
					}
					if item.ignores != nil && isIgnoredByLayers(item.ignores, fullPath, false) {
						continue
					}
					pw.fileCh <- FileEntry{Path: fullPath}
				} else if mode == unix.S_IFDIR {
					if skipDir(entry.Name, pw.hidden) {
						continue
					}
					if item.ignores != nil && isIgnoredByLayers(item.ignores, fullPath, true) {
						continue
					}
					var childIgnores []ignoreLayer
					if !pw.noIgnore {
						childIgnores = make([]ignoreLayer, len(item.ignores)+1)
						copy(childIgnores, item.ignores)
						childIgnores[len(item.ignores)] = loadIgnoreLayer(fullPath)
					}
					subdirs = append(subdirs, walkItem{path: fullPath, ignores: childIgnores})
				}
			}
		}
	}

	unix.Close(fd)

	// Enqueue discovered subdirectories after closing fd.
	for _, sub := range subdirs {
		pw.enqueue(sub)
	}
	return dirents
}

// joinPath concatenates a directory and entry name with a single separator.
// Avoids filepath.Join overhead (no Clean, no validation) since we control
// the inputs: dirPath is always a valid directory path, name is a plain filename.
// Uses a single allocation via make+copy instead of string concatenation.
func joinPath(dirPath, name string) string {
	needsSep := len(dirPath) == 0 || dirPath[len(dirPath)-1] != '/'
	n := len(dirPath) + len(name)
	if needsSep {
		n++
	}
	buf := make([]byte, n)
	copy(buf, dirPath)
	i := len(dirPath)
	if needsSep {
		buf[i] = '/'
		i++
	}
	copy(buf[i:], name)
	return unsafe.String(&buf[0], len(buf))
}

// skipDir returns true for directories that should be skipped.
// VCS directories (.git, .svn, .hg) are always skipped.
// Other hidden directories are skipped unless hidden is true.
func skipDir(name string, hidden bool) bool {
	switch name {
	case ".git", ".svn", ".hg":
		return true
	}
	if !hidden && len(name) > 0 && name[0] == '.' {
		return true
	}
	return false
}

// WalkError represents an error during directory traversal.
type WalkError struct {
	Path string
	Err  error
}

func (e *WalkError) Error() string {
	return "walk " + e.Path + ": " + e.Err.Error()
}

func (e *WalkError) Unwrap() error {
	return e.Err
}
