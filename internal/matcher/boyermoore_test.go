package matcher

import (
	"bytes"
	"testing"
)

func TestBoyerMooreMatcher_FindAll(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		ignoreCase bool
		invert     bool
		input      string
		wantCount  int
		wantLines  []int
	}{
		{
			name:      "simple match",
			pattern:   "hello",
			input:     "hello world\ngoodbye world\n",
			wantCount: 1,
			wantLines: []int{1},
		},
		{
			name:      "no match",
			pattern:   "xyz",
			input:     "hello world\ngoodbye world\n",
			wantCount: 0,
		},
		{
			name:       "case insensitive",
			pattern:    "hello",
			ignoreCase: true,
			input:      "Hello World\nhello world\nHELLO\n",
			wantCount:  3,
			wantLines:  []int{1, 2, 3},
		},
		{
			name:      "multiple matches same line",
			pattern:   "ab",
			input:     "ababab\n",
			wantCount: 1,
			wantLines: []int{1},
		},
		{
			name:      "invert match",
			pattern:   "hello",
			invert:    true,
			input:     "hello\nworld\nhello again\n",
			wantCount: 1,
			wantLines: []int{2},
		},
		{
			name:      "empty input",
			pattern:   "hello",
			input:     "",
			wantCount: 0,
		},
		{
			name:      "no trailing newline",
			pattern:   "end",
			input:     "start\nend",
			wantCount: 1,
			wantLines: []int{2},
		},
		{
			name:      "pattern at start of line",
			pattern:   "start",
			input:     "start of line\n",
			wantCount: 1,
			wantLines: []int{1},
		},
		{
			name:      "pattern at end of line",
			pattern:   "end",
			input:     "the end\n",
			wantCount: 1,
			wantLines: []int{1},
		},
		{
			name:      "single char pattern",
			pattern:   "x",
			input:     "ax\nbx\nc\n",
			wantCount: 2,
			wantLines: []int{1, 2},
		},
		{
			name:      "pattern longer than any line",
			pattern:   "abcdefghij",
			input:     "abc\ndef\n",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewBoyerMooreMatcher(tt.pattern, tt.ignoreCase, tt.invert)
			matches := m.FindAll([]byte(tt.input))
			if len(matches) != tt.wantCount {
				t.Errorf("got %d matches, want %d", len(matches), tt.wantCount)
			}
			for i, wantLine := range tt.wantLines {
				if i >= len(matches) {
					break
				}
				if matches[i].LineNum != wantLine {
					t.Errorf("match[%d].LineNum = %d, want %d", i, matches[i].LineNum, wantLine)
				}
			}
		})
	}
}

func TestBoyerMooreMatcher_Positions(t *testing.T) {
	m := NewBoyerMooreMatcher("ab", false, false)
	matches := m.FindAll([]byte("xabcabd\n"))
	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1", len(matches))
	}
	if len(matches[0].Positions) != 2 {
		t.Fatalf("got %d positions, want 2", len(matches[0].Positions))
	}
	if matches[0].Positions[0] != [2]int{1, 3} {
		t.Errorf("position[0] = %v, want [1,3]", matches[0].Positions[0])
	}
	if matches[0].Positions[1] != [2]int{4, 6} {
		t.Errorf("position[1] = %v, want [4,6]", matches[0].Positions[1])
	}
}

func TestBoyerMooreMatcher_FindLine(t *testing.T) {
	m := NewBoyerMooreMatcher("test", false, false)

	match, ok := m.FindLine([]byte("this is a test"), 5, 100)
	if !ok {
		t.Fatal("expected match")
	}
	if match.LineNum != 5 {
		t.Errorf("LineNum = %d, want 5", match.LineNum)
	}
	if match.ByteOffset != 100 {
		t.Errorf("ByteOffset = %d, want 100", match.ByteOffset)
	}
	if len(match.Positions) != 1 || match.Positions[0] != [2]int{10, 14} {
		t.Errorf("Positions = %v, want [[10 14]]", match.Positions)
	}

	_, ok = m.FindLine([]byte("no match here"), 1, 0)
	if ok {
		t.Error("expected no match")
	}
}

func TestBoyerMooreMatcher_CaseInsensitivePositions(t *testing.T) {
	m := NewBoyerMooreMatcher("hello", true, false)
	matches := m.FindAll([]byte("Hello HELLO hElLo\n"))
	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1", len(matches))
	}
	// Should find 3 positions on the single line
	if len(matches[0].Positions) != 3 {
		t.Fatalf("got %d positions, want 3", len(matches[0].Positions))
	}
	expected := [][2]int{{0, 5}, {6, 11}, {12, 17}}
	for i, pos := range matches[0].Positions {
		if pos != expected[i] {
			t.Errorf("position[%d] = %v, want %v", i, pos, expected[i])
		}
	}
}

func TestBoyerMooreMatcher_SIMDSearch(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		ignoreCase bool
		text       string
		wantCount  int
		wantLines  []int
	}{
		{
			name:      "basic fixed string",
			pattern:   "abc",
			text:      "xabcxxabc\nxyz\n",
			wantCount: 1,
			wantLines: []int{1},
		},
		{
			name:       "case insensitive",
			pattern:    "ABC",
			ignoreCase: true,
			text:       "xAbCxaBC\nxyz\n",
			wantCount:  1,
			wantLines:  []int{1},
		},
		{
			name:      "multiple lines",
			pattern:   "abc",
			text:      "abc\ndef\nabc\n",
			wantCount: 2,
			wantLines: []int{1, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewBoyerMooreMatcher(tt.pattern, tt.ignoreCase, false)
			matches := m.FindAll([]byte(tt.text))
			if len(matches) != tt.wantCount {
				t.Fatalf("got %d matches, want %d", len(matches), tt.wantCount)
			}
			for i, wantLine := range tt.wantLines {
				if i >= len(matches) {
					break
				}
				if matches[i].LineNum != wantLine {
					t.Errorf("match[%d].LineNum = %d, want %d", i, matches[i].LineNum, wantLine)
				}
			}
		})
	}
}

func BenchmarkBoyerMoore_ShortPattern(b *testing.B) {
	data := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog\n"), 10000)
	m := NewBoyerMooreMatcher("lazy", false, false)
	b.ResetTimer()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		m.FindAll(data)
	}
}

func BenchmarkBoyerMoore_LongPattern(b *testing.B) {
	data := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog\n"), 10000)
	m := NewBoyerMooreMatcher("jumps over the lazy", false, false)
	b.ResetTimer()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		m.FindAll(data)
	}
}

func BenchmarkBoyerMoore_NoMatch(b *testing.B) {
	data := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog\n"), 10000)
	m := NewBoyerMooreMatcher("zzzzz", false, false)
	b.ResetTimer()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		m.FindAll(data)
	}
}

func BenchmarkBoyerMoore_CaseInsensitive(b *testing.B) {
	data := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog\n"), 10000)
	m := NewBoyerMooreMatcher("LAZY", true, false)
	b.ResetTimer()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		m.FindAll(data)
	}
}

// BenchmarkBoyerMoore_SparseMatch: 10K lines, only 10 match (1 in 1000)
func BenchmarkBoyerMoore_SparseMatch(b *testing.B) {
	var buf []byte
	for i := range 10000 {
		if i%1000 == 0 {
			buf = append(buf, []byte("ERROR: connection refused at port 8080\n")...)
		} else {
			buf = append(buf, []byte("the quick brown fox jumps over the lazy dog\n")...)
		}
	}
	m := NewBoyerMooreMatcher("ERROR", false, false)
	b.ResetTimer()
	b.SetBytes(int64(len(buf)))
	for b.Loop() {
		m.FindAll(buf)
	}
}

// BenchmarkRegex_SparseMatch: same data, regex matcher
func BenchmarkRegex_SparseMatch(b *testing.B) {
	var buf []byte
	for i := range 10000 {
		if i%1000 == 0 {
			buf = append(buf, []byte("ERROR: connection refused at port 8080\n")...)
		} else {
			buf = append(buf, []byte("the quick brown fox jumps over the lazy dog\n")...)
		}
	}
	m, _ := NewRegexMatcher("ERROR", false, false)
	b.ResetTimer()
	b.SetBytes(int64(len(buf)))
	for b.Loop() {
		m.FindAll(buf)
	}
}

// Baseline: bytes.Index for comparison
func BenchmarkBytesIndex_ShortPattern(b *testing.B) {
	data := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog\n"), 10000)
	pattern := []byte("lazy")
	b.ResetTimer()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		d := data
		for {
			idx := bytes.Index(d, pattern)
			if idx < 0 {
				break
			}
			d = d[idx+len(pattern):]
		}
	}
}
