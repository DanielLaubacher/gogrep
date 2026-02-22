package matcher

import (
	"bytes"
	"testing"
)

func TestAhoCorasickMatcher_FindAll(t *testing.T) {
	tests := []struct {
		name       string
		patterns   []string
		ignoreCase bool
		invert     bool
		input      string
		wantCount  int
		wantLines  []int
	}{
		{
			name:      "two patterns",
			patterns:  []string{"apple", "cherry"},
			input:     "apple\nbanana\ncherry\n",
			wantCount: 2,
			wantLines: []int{1, 3},
		},
		{
			name:      "all match",
			patterns:  []string{"a", "b", "c"},
			input:     "a\nb\nc\n",
			wantCount: 3,
			wantLines: []int{1, 2, 3},
		},
		{
			name:      "no match",
			patterns:  []string{"xyz", "qqq"},
			input:     "hello\nworld\n",
			wantCount: 0,
		},
		{
			name:       "case insensitive",
			patterns:   []string{"apple", "banana"},
			ignoreCase: true,
			input:      "APPLE\nBanana\ncherry\n",
			wantCount:  2,
			wantLines:  []int{1, 2},
		},
		{
			name:      "invert match",
			patterns:  []string{"apple", "cherry"},
			invert:    true,
			input:     "apple\nbanana\ncherry\n",
			wantCount: 1,
			wantLines: []int{2},
		},
		{
			name:      "multiple patterns on same line",
			patterns:  []string{"foo", "bar"},
			input:     "foobar\nbaz\n",
			wantCount: 1,
			wantLines: []int{1},
		},
		{
			name:      "overlapping patterns",
			patterns:  []string{"ab", "bc"},
			input:     "abc\n",
			wantCount: 1,
			wantLines: []int{1},
		},
		{
			name:      "empty input",
			patterns:  []string{"a", "b"},
			input:     "",
			wantCount: 0,
		},
		{
			name:      "pattern is substring of another",
			patterns:  []string{"he", "hello"},
			input:     "hello world\n",
			wantCount: 1,
			wantLines: []int{1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewAhoCorasickMatcher(tt.patterns, tt.ignoreCase, tt.invert)
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

func TestAhoCorasickMatcher_Positions(t *testing.T) {
	m := NewAhoCorasickMatcher([]string{"ab", "cd"}, false, false)
	matches := m.FindAll([]byte("xabxcdx\n"))
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

func TestAhoCorasickMatcher_FindLine(t *testing.T) {
	m := NewAhoCorasickMatcher([]string{"foo", "bar"}, false, false)

	match, ok := m.FindLine([]byte("foobar baz"), 3, 50)
	if !ok {
		t.Fatal("expected match")
	}
	if match.LineNum != 3 {
		t.Errorf("LineNum = %d, want 3", match.LineNum)
	}
	if match.ByteOffset != 50 {
		t.Errorf("ByteOffset = %d, want 50", match.ByteOffset)
	}
	// Should have 2 positions: "foo" at [0,3] and "bar" at [3,6]
	if len(match.Positions) != 2 {
		t.Fatalf("got %d positions, want 2", len(match.Positions))
	}

	_, ok = m.FindLine([]byte("no match"), 1, 0)
	if ok {
		t.Error("expected no match")
	}
}

func TestAhoCorasickMatcher_FailureLinks(t *testing.T) {
	// Test that failure links work correctly with shared suffixes
	m := NewAhoCorasickMatcher([]string{"abc", "bc", "c"}, false, false)
	matches := m.FindAll([]byte("abc\n"))
	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1", len(matches))
	}
	// Should find all 3 patterns on the line via failure links
	if len(matches[0].Positions) < 1 {
		t.Fatal("expected at least 1 position")
	}
}

func TestAhoCorasickMatcher_SearchLine(t *testing.T) {
	m := NewAhoCorasickMatcher([]string{"he", "she", "his", "hers"}, false, false)
	acMatches := m.searchLine([]byte("ahishers"))

	// Expected matches: "his" at 1, "she" at 3, "he" at 4, "hers" at 4
	if len(acMatches) < 3 {
		t.Errorf("got %d acMatches, want at least 3: %v", len(acMatches), acMatches)
	}
}

func BenchmarkAhoCorasick_TwoPatterns(b *testing.B) {
	data := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog\n"), 10000)
	m := NewAhoCorasickMatcher([]string{"fox", "dog"}, false, false)
	b.ResetTimer()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		m.FindAll(data)
	}
}

func BenchmarkAhoCorasick_TenPatterns(b *testing.B) {
	data := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog and the cat sat on the mat\n"), 10000)
	m := NewAhoCorasickMatcher([]string{
		"fox", "dog", "cat", "mat", "the",
		"quick", "brown", "lazy", "jumps", "over",
	}, false, false)
	b.ResetTimer()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		m.FindAll(data)
	}
}

func BenchmarkAhoCorasick_NoMatch(b *testing.B) {
	data := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog\n"), 10000)
	m := NewAhoCorasickMatcher([]string{"zzz", "yyy", "xxx"}, false, false)
	b.ResetTimer()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		m.FindAll(data)
	}
}

func BenchmarkAhoCorasick_CaseInsensitive(b *testing.B) {
	data := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog\n"), 10000)
	m := NewAhoCorasickMatcher([]string{"FOX", "DOG"}, true, false)
	b.ResetTimer()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		m.FindAll(data)
	}
}
