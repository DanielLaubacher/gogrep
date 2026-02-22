# gogrep Architecture

gogrep is a Linux-only, high-performance grep alternative written in pure Go. Every layer is designed to minimize syscalls, avoid allocations on hot paths, and exploit Linux-specific kernel features.

## Design Goals

1. **Linux-only** -- use raw syscalls (`getdents64`, `openat`, `mmap`, `fadvise`, `madvise`, `writev`, `inotify`, `epoll`) instead of portable Go abstractions.
2. **SIMD-accelerated** -- fixed-string search uses AVX2 intrinsics via Go 1.26 `simd/archsimd`.
3. **Search-then-split** -- search the entire file buffer first, then extract line boundaries only around matches. Avoids per-line overhead for files with sparse matches.
4. **Zero allocations on hot paths** -- `[]byte` everywhere, `sync.Pool` for buffers, no `string` conversions during search.
5. **Pure Go, no cgo** -- no C bindings. PCRE2 support comes from a pure Go port (`go.elara.ws/pcre`), SIMD from `simd/archsimd`.

## Pipeline

```
                  +-----------+
                  |  CLI      |  cmd/gogrep/main.go
                  |  (cobra)  |  parses flags, builds Config
                  +-----+-----+
                        |
                  +-----v-----+
                  |  run.go   |  internal/cli/run.go
                  |           |  orchestrates one of 4 modes:
                  |           |    stdin | files | recursive | watch
                  +-----+-----+
                        |
          +-------------+-------------+
          |             |             |
    +-----v-----+ +----v----+ +------v------+
    |  Walker   | | Reader  | |  Matcher    |
    | getdents  | | mmap /  | | regex / BM  |
    | + openat  | | pread   | | AC / PCRE   |
    +-----------+ +---------+ |   + SIMD    |
                              +------+------+
                                     |
                              +------v------+
                              | Formatter   |
                              | text / json |
                              +------+------+
                                     |
                              +------v------+
                              |  Writer     |
                              |  writev()   |
                              +-------------+
```

In recursive mode, a **Scheduler** (worker pool) sits between the Walker and Matcher, distributing files across `NumCPU * 2` goroutines. An **OrderedWriter** reassembles results in deterministic order using sequence numbers.

## Directory Traversal

`internal/walker/` replaces `filepath.WalkDir` with raw Linux syscalls.

1. Open directory with `unix.Open(path, O_RDONLY | O_DIRECTORY | O_NOATIME, 0)`.
2. Read entries with `unix.Getdents(fd, buf)` into a 32 KB buffer.
3. Parse raw `linux_dirent64` structs in-place (`unsafe.Pointer`). Each entry's `d_type` field classifies it as `DT_REG`, `DT_DIR`, `DT_LNK`, or `DT_UNKNOWN` without any `stat` syscall.
4. Regular files: emit path-only `FileEntry{Path}` — file opening and stat are deferred to the reader.
5. Directories: recurse with a parallel BFS (`NumCPU` walker goroutines). Skip `.git`, `.svn`, `.hg`, `node_modules`, hidden dirs (`.` prefix).
6. `DT_UNKNOWN` (rare, some filesystems like XFS): fall back to `fstatat` to determine type.
7. `.gitignore` support: loads and stacks ignore rules per directory, matching patterns against relative paths.

**Result**: eliminates one `lstat` per file. On a tree with 100K files, that's 100K fewer syscalls compared to `filepath.WalkDir`.

## File Reading

`internal/input/` provides two strategies, selected by file size.

### Buffered Reader (files < 8 MB)

1. `unix.Open` with `O_RDONLY | O_NOATIME`.
2. `unix.Fstat` to get size.
3. Allocate `[]byte` of exact size.
4. `unix.Pread` in a loop (positional read, no seek state).
5. Close fd immediately.

### Mmap Reader (files >= 8 MB)

1. `unix.Open` with `O_RDONLY | O_NOATIME`.
2. `unix.Fadvise(fd, 0, size, FADV_SEQUENTIAL)` -- hint the kernel to read ahead aggressively.
3. `syscall.Mmap(fd, 0, size, PROT_READ, MAP_PRIVATE | MAP_POPULATE)` -- `MAP_POPULATE` prefaults all pages at map time, avoiding page faults during search.
4. `unix.Madvise(data, MADV_SEQUENTIAL)` -- reinforce sequential access hint.
5. On cleanup: `unix.Madvise(data, MADV_DONTNEED)` to release page cache, then `syscall.Munmap`, then close fd.

An `AdaptiveReader` automatically selects between the two based on a configurable threshold (default 8 MB).

`O_NOATIME` is used on every file open to eliminate atime inode writes. Falls back gracefully if the process lacks `CAP_FOWNER`.

## Pattern Matching

`internal/matcher/` provides four matcher backends, all implementing the same interface:

```go
type Matcher interface {
    FindAll(data []byte) []Match
    MatchExists(data []byte) bool
    FindLine(line []byte, lineNum int, byteOffset int64) (Match, bool)
}
```

`MatchExists` provides a fast path for `-l` / `--files-with-matches` mode, skipping line boundary extraction entirely.

### Selection Logic

| Condition | Matcher | Engine |
|---|---|---|
| `-P` (PCRE) | `PCREMatcher` | `go.elara.ws/pcre` (pure Go PCRE2 port) |
| `-F` + 1 pattern | `BoyerMooreMatcher` | SIMD-friendly Horspool via `simd/archsimd` |
| `-F` + N patterns | `AhoCorasickMatcher` | Hand-written trie with `[256]*node` children + BFS failure links |
| Default (regex) | `RegexMatcher` | Go stdlib `regexp` (RE2) |

### Search-then-Split

All matchers search the entire file buffer in a single pass, then extract line boundaries only around match positions. This inverts the traditional "split into lines, then search each line" approach.

For a 500K-line file with 3 matches, the old approach made 500K `findInLine()` calls. The new approach makes 1 whole-buffer search + 3 line extractions.

A helper function `lineFromOffset()` efficiently extracts line boundaries and computes line numbers incrementally from a previous known position, avoiding redundant newline counting.

### SIMD Acceleration

`internal/simd/` uses Go 1.26's `simd/archsimd` for AVX2 intrinsics (requires `GOEXPERIMENT=simd`).

The **SIMD-friendly Horspool** algorithm (`simd.IndexAll`):

1. Broadcast pattern's first byte and last byte into 32-byte AVX2 vectors.
2. For each 32-byte block in the data:
   - Load 32 bytes at position `i` and at position `i + patternLen - 1`.
   - `VPCMPEQB` to compare all 32 positions against first/last bytes simultaneously.
   - `VPAND` the two result masks.
   - `VPMOVMSKB` to extract a 32-bit bitmask of candidate positions.
3. For each set bit in the bitmask (typically 0-1 per block): verify the middle bytes with `bytes.Equal`.
4. `VZEROUPPER` before returning to avoid AVX/SSE transition penalties.

This processes 32 candidate positions per iteration. For typical text, the first+last byte filter eliminates >99% of false positives, making the inner verification extremely rare.

Case-insensitive search broadcasts both lower and upper forms of the first/last bytes and ORs the masks.

## Output

### Text Formatter

Uses raw ANSI escape codes for zero-allocation coloring:
- Filenames: `\x1b[35m` magenta
- Line numbers: `\x1b[32m` green
- Separators: `\x1b[36m` cyan
- Matches: `\x1b[1;31m` bold red

Color mode is auto-detected via `unix.IoctlGetTermios(fd, TCGETS)` (raw TTY detection, no external package). Output buffer is pre-allocated based on match count to avoid `growslice` overhead.

### JSON Formatter

Outputs one JSON object per match line in JSON Lines format.

### Writer

All output goes through `unix.Writev` for scatter-gather I/O, batching filename, separator, line content, and newline into a single syscall.

An `OrderedWriter` buffers out-of-order results from parallel workers and emits them in sequence-number order to maintain deterministic output.

## Watch Mode

`internal/watch/` implements file watching with raw Linux inotify + epoll:

1. `unix.InotifyInit1(IN_CLOEXEC | IN_NONBLOCK)` -- create inotify instance.
2. `unix.InotifyAddWatch(fd, path, IN_MODIFY | IN_CREATE | IN_MOVED_TO | IN_MOVE_SELF | IN_DELETE_SELF)` -- watch for modifications, new files, and log rotation.
3. `unix.EpollCreate1(EPOLL_CLOEXEC)` + `unix.EpollWait` with 100ms timeout -- efficient event loop.
4. On `IN_MODIFY`: `unix.Pread` from last known offset to read only new content. Handles truncation (log rotation) by resetting the offset.

## Concurrency Model

```
Walker goroutine
    |
    | FileEntry channel (buffer 256)
    v
Scheduler (NumCPU * 2 workers)
    |
    | each worker: read file -> match -> emit Result with sequence number
    |
    | Result channel (buffer workers * 2)
    v
OrderedWriter goroutine
    |
    | reorders by sequence number, formats, writes via writev
    v
stdout
```

## Key Constants

| Parameter | Value |
|---|---|
| Mmap threshold | 8 MB |
| Worker count | `NumCPU * 2` |
| getdents buffer | 32 KB |
| Binary detection | first 8192 bytes |
| SIMD block width | 32 bytes (AVX2) |
| File channel buffer | 256 |
| Result channel buffer | `workers * 2` |
| Epoll timeout | 100 ms |

## Dependencies

| Package | Purpose |
|---|---|
| `golang.org/x/sys` | Linux syscalls (getdents, openat, mmap, writev, inotify, epoll) |
| `github.com/spf13/cobra` | CLI framework |
| `go.elara.ws/pcre` | Pure Go PCRE2 port (no cgo) |
| `github.com/charmbracelet/log` | Structured stderr logging |
| `simd/archsimd` | Go 1.26 experimental AVX2 intrinsics |
