package cli

import (
	"fmt"
	"os"
	"sync/atomic"
	"unicode"

	"github.com/dl/gogrep/internal/input"
	"github.com/dl/gogrep/internal/matcher"
	"github.com/dl/gogrep/internal/output"
	"github.com/dl/gogrep/internal/scheduler"
	"github.com/dl/gogrep/internal/walker"
	"github.com/dl/gogrep/internal/watch"
)

// logWarn writes a warning to stderr.
func logWarn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gogrep: "+format+"\n", args...)
}

// searchMode determines the fast path in searchReader.
type searchMode int

const (
	searchFull      searchMode = iota // full match extraction
	searchFilesOnly                   // just check if any match exists
	searchCountOnly                   // count matching lines, skip line extraction
)

// Run executes the search with the given config.
// Returns exit code: 0 = match found, 1 = no match, 2 = error.
func Run(cfg Config) int {
	// Smart case: if enabled and all patterns are lowercase, enable case-insensitive
	if cfg.SmartCase && !cfg.IgnoreCase {
		allLower := true
		for _, p := range cfg.Patterns {
			for _, r := range p {
				if unicode.IsUpper(r) {
					allLower = false
					break
				}
			}
			if !allLower {
				break
			}
		}
		if allLower {
			cfg.IgnoreCase = true
		}
	}

	// Resolve maxCols for matcher
	maxCols := cfg.MaxColumns
	if maxCols == 0 {
		maxCols = 75
	}
	if maxCols < 0 {
		maxCols = 0 // -1 from CLI means no limit
	}

	// Create matcher
	m, err := matcher.NewMatcher(cfg.Patterns, cfg.Fixed, cfg.PCRE, cfg.IgnoreCase, cfg.Invert, matcher.MatcherOpts{
		MaxCols:      maxCols,
		NeedLineNums: cfg.LineNumbers,
	})
	if err != nil {
		logWarn("invalid pattern: %v", err)
		return 2
	}

	// Wrap with context if needed (not for watch mode — watch handles context via streaming)
	if !cfg.WatchMode {
		m = matcher.NewContextMatcher(m, cfg.ContextBefore, cfg.ContextAfter)
	}

	// Determine color mode
	useColor := false
	switch cfg.Color {
	case ColorAlways:
		useColor = true
	case ColorNever:
		useColor = false
	case ColorAuto:
		useColor = output.StdoutIsTerminal()
	}

	// Create formatter and writer
	w := output.NewWriter()
	var formatter output.Formatter
	if cfg.JSONOutput {
		formatter = output.NewJSONFormatter()
	} else {
		formatter = output.NewTextFormatter(cfg.LineNumbers, cfg.CountOnly, cfg.FileNamesOnly, useColor, maxCols)
	}

	reader := input.NewAdaptiveReader(cfg.MmapThreshold)
	stdinReader := input.NewStdinReader()

	// Determine search mode
	mode := searchFull
	if cfg.FileNamesOnly {
		mode = searchFilesOnly
	} else if cfg.CountOnly {
		mode = searchCountOnly
	}

	// Determine input sources
	paths := cfg.Paths
	readFromStdin := len(paths) == 0

	if cfg.WatchMode {
		return runWatch(paths, m, formatter, w, cfg)
	}

	if readFromStdin {
		return runStdin(stdinReader, m, formatter, w)
	}

	if cfg.Recursive {
		return runRecursive(paths, m, reader, formatter, w, cfg, mode)
	}

	return runFiles(paths, m, reader, formatter, w, mode)
}

func runStdin(reader input.Reader, m matcher.Matcher, formatter output.Formatter, w *output.Writer) int {
	result := searchReader(reader, "", m, searchFull)
	if result.HasMatch() {
		buf := formatter.Format(nil, result, false)
		if result.Closer != nil {
			result.Closer()
		}
		w.Write(buf)
		return 0
	}
	if result.Closer != nil {
		result.Closer()
	}
	return 1
}

func runFiles(paths []string, m matcher.Matcher, reader input.Reader, formatter output.Formatter, w *output.Writer, mode searchMode) int {
	multiFile := len(paths) > 1
	hasMatch := false
	var buf []byte

	for _, path := range paths {
		result := searchReader(reader, path, m, mode)
		if result.Err != nil {
			logWarn("%s: %v", path, result.Err)
			continue
		}
		if result.HasMatch() {
			hasMatch = true
		}
		buf = formatter.Format(buf[:0], result, multiFile)
		if result.Closer != nil {
			result.Closer()
		}
		w.Write(buf)
	}

	if hasMatch {
		return 0
	}
	return 1
}

func runRecursive(paths []string, m matcher.Matcher, reader input.Reader, formatter output.Formatter, w *output.Writer, cfg Config, mode searchMode) int {
	fileCh, errCh := walker.Walk(paths, walker.WalkOptions{
		Recursive:      true,
		NoIgnore:       cfg.NoIgnore,
		Hidden:         cfg.Hidden,
		FollowSymlinks: cfg.FollowSymlinks,
		Globs:          cfg.Globs,
	})

	// Log walk errors in background
	go func() {
		for err := range errCh {
			logWarn("walk: %v", err)
		}
	}()

	// Create scheduler and run workers
	sched := scheduler.New(cfg.Workers, m, reader, mode == searchFilesOnly, mode == searchCountOnly)
	resultCh := sched.Run(fileCh)

	// Write results in order
	var hasMatch atomic.Bool
	ow := output.NewOrderedWriter(w, formatter, true)
	ow.WriteOrdered(resultCh, func() {
		hasMatch.Store(true)
	})

	if hasMatch.Load() {
		return 0
	}
	return 1
}

func runWatch(paths []string, m matcher.Matcher, formatter output.Formatter, w *output.Writer, cfg Config) int {
	watcher, err := watch.New()
	if err != nil {
		logWarn("failed to create watcher: %v", err)
		return 2
	}
	defer watcher.Close()

	// Add all paths to watch
	for _, path := range paths {
		if err := watcher.Add(path); err != nil {
			logWarn("failed to watch %s: %v", path, err)
			return 2
		}
	}

	hasMatch := false
	events := watcher.Events()

	for evt := range events {
		if evt.Err != nil {
			logWarn("watch: %v", evt.Err)
			continue
		}

		switch evt.Type {
		case watch.EventModified:
			data, err := watcher.ReadNew(evt.Path)
			if err != nil {
				logWarn("%s: read: %v", evt.Path, err)
				continue
			}
			if len(data) == 0 {
				continue
			}

			// Search the new content
			ms := m.FindAll(data)
			if ms.HasMatch() {
				hasMatch = true
				result := output.Result{
					FilePath: evt.Path,
					MatchSet: ms,
				}
				buf := formatter.Format(nil, result, true)
				w.Write(buf)
			}

		case watch.EventCreated:
			// Add newly created files to the watch
			if err := watcher.Add(evt.Path); err != nil {
				logWarn("failed to watch %s: %v", evt.Path, err)
			}

		case watch.EventDeleted:
			logWarn("watched file removed: %s", evt.Path)
		}
	}

	if hasMatch {
		return 0
	}
	return 1
}

func searchReader(r input.Reader, path string, m matcher.Matcher, mode searchMode) output.Result {
	result := output.Result{FilePath: path}

	readResult, err := r.Read(path)
	if err != nil {
		result.Err = err
		return result
	}

	closeReader := func() {
		if readResult.Closer != nil {
			readResult.Closer()
		}
	}

	if readResult.Data == nil {
		closeReader()
		return result
	}

	// Binary detection: skip binary files entirely (like ripgrep)
	if walker.IsBinary(readResult.Data) {
		closeReader()
		return result
	}

	switch mode {
	case searchFilesOnly:
		if m.MatchExists(readResult.Data) {
			result.MatchSet = matcher.MatchSet{Matches: []matcher.Match{{}}}
		}
		closeReader()
	case searchCountOnly:
		count := m.CountAll(readResult.Data)
		result.MatchCount = count
		closeReader()
	default:
		result.MatchSet = m.FindAll(readResult.Data)
		// MatchSet.Data is the file buffer — pass Closer
		// to the caller so the buffer stays alive until formatting is done.
		if result.MatchSet.HasMatch() {
			result.Closer = closeReader
		} else {
			closeReader()
		}
	}
	return result
}
