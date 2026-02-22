package cli

import (
	"os"
	"sync/atomic"

	"github.com/charmbracelet/log"

	"github.com/dl/gogrep/internal/input"
	"github.com/dl/gogrep/internal/matcher"
	"github.com/dl/gogrep/internal/output"
	"github.com/dl/gogrep/internal/scheduler"
	"github.com/dl/gogrep/internal/walker"
	"github.com/dl/gogrep/internal/watch"
)

// Run executes the search with the given config.
// Returns exit code: 0 = match found, 1 = no match, 2 = error.
func Run(cfg Config) int {
	logger := log.NewWithOptions(os.Stderr, log.Options{
		Level: log.WarnLevel,
	})

	// Create matcher
	m, err := matcher.NewMatcher(cfg.Patterns, cfg.Fixed, cfg.PCRE, cfg.IgnoreCase, cfg.Invert)
	if err != nil {
		logger.Error("invalid pattern", "err", err)
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
		var styles output.Styles
		if useColor {
			styles = output.NewStyles()
		} else {
			styles = output.NoStyles()
		}
		formatter = output.NewTextFormatter(styles, cfg.LineNumbers, cfg.CountOnly, cfg.FileNamesOnly, useColor)
	}

	reader := input.NewAdaptiveReader(cfg.MmapThreshold)
	stdinReader := input.NewStdinReader()

	// Determine input sources
	paths := cfg.Paths
	readFromStdin := len(paths) == 0

	if cfg.WatchMode {
		return runWatch(paths, m, formatter, w, cfg, logger)
	}

	if readFromStdin {
		return runStdin(stdinReader, m, formatter, w)
	}

	if cfg.Recursive {
		return runRecursive(paths, m, reader, formatter, w, cfg, logger)
	}

	return runFiles(paths, m, reader, formatter, w, logger)
}

func runStdin(reader input.Reader, m matcher.Matcher, formatter output.Formatter, w *output.Writer) int {
	result := searchReader(reader, "", m)
	if len(result.Matches) > 0 {
		w.Write(formatter.Format(result, false))
		return 0
	}
	return 1
}

func runFiles(paths []string, m matcher.Matcher, reader input.Reader, formatter output.Formatter, w *output.Writer, logger *log.Logger) int {
	multiFile := len(paths) > 1
	hasMatch := false

	for _, path := range paths {
		result := searchReader(reader, path, m)
		if result.Err != nil {
			logger.Warn("error", "path", path, "err", result.Err)
			continue
		}
		if len(result.Matches) > 0 {
			hasMatch = true
		}
		data := formatter.Format(result, multiFile)
		w.Write(data)
	}

	if hasMatch {
		return 0
	}
	return 1
}

func runRecursive(paths []string, m matcher.Matcher, reader input.Reader, formatter output.Formatter, w *output.Writer, cfg Config, logger *log.Logger) int {
	fileCh, errCh := walker.Walk(paths, walker.WalkOptions{
		Recursive: true,
		NoIgnore:  cfg.NoIgnore,
		Hidden:    cfg.Hidden,
	})

	// Log walk errors in background
	go func() {
		for err := range errCh {
			logger.Warn("walk error", "err", err)
		}
	}()

	// Create scheduler and run workers
	sched := scheduler.New(cfg.Workers, m, reader, cfg.FileNamesOnly)
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

func runWatch(paths []string, m matcher.Matcher, formatter output.Formatter, w *output.Writer, cfg Config, logger *log.Logger) int {
	watcher, err := watch.New()
	if err != nil {
		logger.Error("failed to create watcher", "err", err)
		return 2
	}
	defer watcher.Close()

	// Add all paths to watch
	for _, path := range paths {
		if err := watcher.Add(path); err != nil {
			logger.Error("failed to watch", "path", path, "err", err)
			return 2
		}
	}

	hasMatch := false
	events := watcher.Events()

	for evt := range events {
		if evt.Err != nil {
			logger.Warn("watch error", "err", evt.Err)
			continue
		}

		switch evt.Type {
		case watch.EventModified:
			data, err := watcher.ReadNew(evt.Path)
			if err != nil {
				logger.Warn("read error", "path", evt.Path, "err", err)
				continue
			}
			if len(data) == 0 {
				continue
			}

			// Search the new content
			matches := m.FindAll(data)
			if len(matches) > 0 {
				hasMatch = true
				result := output.Result{
					FilePath: evt.Path,
					Matches:  matches,
				}
				w.Write(formatter.Format(result, true))
			}

		case watch.EventCreated:
			// Add newly created files to the watch
			if err := watcher.Add(evt.Path); err != nil {
				logger.Warn("failed to watch new file", "path", evt.Path, "err", err)
			}

		case watch.EventDeleted:
			logger.Warn("watched file removed", "path", evt.Path)
		}
	}

	if hasMatch {
		return 0
	}
	return 1
}

func searchReader(r input.Reader, path string, m matcher.Matcher) output.Result {
	result := output.Result{FilePath: path}

	readResult, err := r.Read(path)
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

	result.Matches = m.FindAll(readResult.Data)
	return result
}
