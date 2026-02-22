# gogrep Usage

## Synopsis

```
gogrep [OPTIONS] PATTERN [FILE...]
gogrep [OPTIONS] -e PATTERN [-e PATTERN...] [FILE...]
```

If no files are given, reads from stdin.

## Options

### Pattern Selection

| Flag | Short | Description |
|---|---|---|
| `--regexp PATTERN` | `-e` | Pattern to match (repeatable for multiple patterns) |
| `--fixed-strings` | `-F` | Treat pattern as a literal string, not a regex |
| `--perl-regexp` | `-P` | Use PCRE2 regex (supports lookahead, lookbehind, backreferences) |
| `--ignore-case` | `-i` | Case-insensitive matching |
| `--invert-match` | `-v` | Select lines that do NOT match |

### Output Control

| Flag | Short | Description |
|---|---|---|
| `--line-number` | `-n` | Print line numbers |
| `--count` | `-c` | Print only a count of matching lines per file |
| `--files-with-matches` | `-l` | Print only filenames containing matches |
| `--color MODE` | | Color output: `auto` (default), `always`, `never` |
| `--json` | | Output results as JSON Lines |

### Context

| Flag | Short | Description |
|---|---|---|
| `--before-context NUM` | `-B` | Print NUM lines before each match |
| `--after-context NUM` | `-A` | Print NUM lines after each match |
| `--context NUM` | `-C` | Print NUM lines before and after each match |

### Search Modes

| Flag | Short | Description |
|---|---|---|
| `--recursive` | `-r` | Recursively search directories |
| `--watch` | | Watch files for changes and search new content |

## Exit Codes

| Code | Meaning |
|---|---|
| 0 | Match found |
| 1 | No match |
| 2 | Error |

## Examples

### Basic Search

Search for a pattern in a file:

```sh
gogrep "error" app.log
```

Search stdin:

```sh
cat app.log | gogrep "timeout"
```

### Case-Insensitive Search

```sh
gogrep -i "warning" app.log
```

### Fixed String Search

Treat the pattern as a literal string (no regex metacharacters):

```sh
gogrep -F "[ERROR]" app.log
```

### Line Numbers

```sh
gogrep -n "TODO" src/*.go
```

### Recursive Search

Search all files in a directory tree:

```sh
gogrep -rn "func main" ./src/
```

### Invert Match

Show lines that do NOT contain the pattern:

```sh
gogrep -v "DEBUG" app.log
```

### Count Matches

```sh
gogrep -c "error" *.log
```

### Files With Matches

List only filenames that contain a match:

```sh
gogrep -rl "TODO" ./src/
```

### Context Lines

Show 2 lines before and after each match:

```sh
gogrep -C2 "panic" app.log
```

Show 3 lines after each match:

```sh
gogrep -A3 "FATAL" app.log
```

### Multiple Patterns

Search for any of several patterns:

```sh
gogrep -e "error" -e "warning" -e "fatal" app.log
```

Multiple fixed strings (uses Aho-Corasick for single-pass matching):

```sh
gogrep -F -e "connection refused" -e "timeout" -e "EOF" app.log
```

### PCRE2 Regex

Use Perl-compatible regex for lookahead, lookbehind, backreferences:

```sh
# Lookahead: words followed by "world"
gogrep -P '\w+(?=\s+world)' file.txt

# Lookbehind: words preceded by "hello "
gogrep -P '(?<=hello\s)\w+' file.txt

# Backreference: repeated words
gogrep -Pn '(\w+)\s+\1' document.txt
```

### JSON Output

Output matches as JSON Lines (one JSON object per match):

```sh
gogrep --json "error" app.log
```

```json
{"type":"match","file":"app.log","line_number":42,"byte_offset":1847,"text":"2024-01-15 ERROR: connection refused","matches":[[15,20]]}
```

### Watch Mode

Watch files for changes and search new content as it's appended:

```sh
gogrep --watch "ERROR" /var/log/syslog
```

Watch multiple files:

```sh
gogrep --watch "panic" app.log worker.log
```

### Color Control

Force color output (useful when piping to `less -R`):

```sh
gogrep --color=always "pattern" file.txt | less -R
```

Disable color:

```sh
gogrep --color=never "pattern" file.txt
```

### Combined Flags

Recursive, case-insensitive, with line numbers and context:

```sh
gogrep -rinC3 "fixme" ./src/
```

Count fixed-string matches per file recursively:

```sh
gogrep -rFc "TODO" ./src/
```

### Searching Binary Files

gogrep automatically detects binary files (by checking for NUL bytes in the first 8 KB). Binary files with matches print a summary instead of the matched content:

```sh
gogrep -r "magic" ./data/
# Binary file ./data/archive.bin matches
```
