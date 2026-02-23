package matcher

import (
	"testing"
)

func TestRegexMatcher_FindAll(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		ignoreCase bool
		invert     bool
		input      string
		wantCount  int
		wantLines  []int // expected 1-based line numbers
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
			name:      "regex metacharacters",
			pattern:   `\d+`,
			input:     "abc\n123\ndef456\n",
			wantCount: 2,
			wantLines: []int{2, 3},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewRegexMatcher(tt.pattern, tt.ignoreCase, tt.invert)
			if err != nil {
				t.Fatalf("NewRegexMatcher() error: %v", err)
			}
			m.needLineNums = true

			ms := m.FindAll([]byte(tt.input))
			if len(ms.Matches) != tt.wantCount {
				t.Errorf("got %d matches, want %d", len(ms.Matches), tt.wantCount)
			}

			for i, wantLine := range tt.wantLines {
				if i >= len(ms.Matches) {
					break
				}
				if ms.Matches[i].LineNum != wantLine {
					t.Errorf("match[%d].LineNum = %d, want %d", i, ms.Matches[i].LineNum, wantLine)
				}
			}
		})
	}
}

func TestRegexMatcher_Positions(t *testing.T) {
	m, err := NewRegexMatcher("ab", false, false)
	if err != nil {
		t.Fatal(err)
	}

	ms := m.FindAll([]byte("xabcabd\n"))
	if len(ms.Matches) != 1 {
		t.Fatalf("got %d matches, want 1", len(ms.Matches))
	}
	positions := ms.MatchPositions(0)
	if len(positions) != 2 {
		t.Fatalf("got %d positions, want 2", len(positions))
	}
	if positions[0] != [2]int{1, 3} {
		t.Errorf("position[0] = %v, want [1,3]", positions[0])
	}
	if positions[1] != [2]int{4, 6} {
		t.Errorf("position[1] = %v, want [4,6]", positions[1])
	}
}

func TestFixedMatcher_FindAll(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		ignoreCase bool
		invert     bool
		input      string
		wantCount  int
	}{
		{
			name:      "simple match",
			pattern:   "hello",
			input:     "hello world\ngoodbye\n",
			wantCount: 1,
		},
		{
			name:       "case insensitive",
			pattern:    "hello",
			ignoreCase: true,
			input:      "Hello\nhello\nHELLO\nworld\n",
			wantCount:  3,
		},
		{
			name:      "no match",
			pattern:   "xyz",
			input:     "hello\nworld\n",
			wantCount: 0,
		},
		{
			name:      "invert",
			pattern:   "hello",
			invert:    true,
			input:     "hello\nworld\nhello\n",
			wantCount: 1,
		},
		{
			name:      "pattern at end of line",
			pattern:   "end",
			input:     "the end\n",
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewFixedMatcher(tt.pattern, tt.ignoreCase, tt.invert)
			ms := m.FindAll([]byte(tt.input))
			if len(ms.Matches) != tt.wantCount {
				t.Errorf("got %d matches, want %d", len(ms.Matches), tt.wantCount)
			}
		})
	}
}

func TestNewMatcher_Fixed(t *testing.T) {
	m, err := NewMatcher([]string{"hello"}, true, false, false, false, MatcherOpts{})
	if err != nil {
		t.Fatal(err)
	}

	ms := m.FindAll([]byte("hello world\ngoodbye\n"))
	if len(ms.Matches) != 1 {
		t.Errorf("got %d matches, want 1", len(ms.Matches))
	}
}

func TestNewMatcher_MultiFixed(t *testing.T) {
	m, err := NewMatcher([]string{"apple", "cherry"}, true, false, false, false, MatcherOpts{})
	if err != nil {
		t.Fatal(err)
	}

	ms := m.FindAll([]byte("apple\nbanana\ncherry\n"))
	if len(ms.Matches) != 2 {
		t.Errorf("got %d matches, want 2", len(ms.Matches))
	}
}

func TestNewMatcher_MultiRegex(t *testing.T) {
	m, err := NewMatcher([]string{"hello", "world"}, false, false, false, false, MatcherOpts{})
	if err != nil {
		t.Fatal(err)
	}

	ms := m.FindAll([]byte("hello\nfoo\nworld\n"))
	if len(ms.Matches) != 2 {
		t.Errorf("got %d matches, want 2", len(ms.Matches))
	}
}

func TestNewMatcher_NoPatterns(t *testing.T) {
	_, err := NewMatcher(nil, false, false, false, false, MatcherOpts{})
	if err == nil {
		t.Error("expected error for no patterns")
	}
}
