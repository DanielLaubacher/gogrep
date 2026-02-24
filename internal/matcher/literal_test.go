package matcher

import (
	"bytes"
	"testing"
)

func TestExtractLiteral(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		ignoreCase bool
		wantLit    string
		wantOK     bool
		wantCI     bool
	}{
		// Basic literals
		{"pure literal long", "timeout", false, "timeout", true, false},
		{"pure literal 3 chars", "foo", false, "foo", true, false},
		{"below min length", "ab", false, "", false, false},
		{"single char", "x", false, "", false, false},

		// Regex with extractable literal
		{"dot-star prefix", ".*timeout", false, "timeout", true, false},
		{"dot-star suffix", "error.*", false, "error", true, false},
		{"word boundary", `\bconnection\b`, false, "connection", true, false},
		{"digit suffix", `error\d+`, false, "error", true, false},
		{"whitespace middle picks longest", `error\s+timeout`, false, "timeout", true, false},
		{"dot-star both sides", ".*timeout.*", false, "timeout", true, false},
		{"anchored", `^error\d+$`, false, "error", true, false},

		// Case-insensitive
		{"case insensitive flag", "timeout", true, "timeout", true, true},
		{"embedded case insensitive", "(?i)timeout", false, "timeout", true, true},

		// No extractable literal
		{"pure digit class", `\d+`, false, "", false, false},
		{"alternation", "foo|bar", false, "", false, false},
		{"char class only", `[abc]+`, false, "", false, false},
		{"dot star only", ".*", false, "", false, false},
		{"anchors only", `^$`, false, "", false, false},
		{"any char", `.`, false, "", false, false},

		// Optional parts don't contribute
		{"optional group", `(?:error)?timeout`, false, "timeout", true, false},
		{"star quantifier", `x*timeout`, false, "timeout", true, false},
		{"question quantifier", `x?timeout`, false, "timeout", true, false},

		// Plus: err+or → Concat[Literal("er"), Plus(Literal("r")), Literal("or")]
		// "er" and "or" are both 2 chars, below min. No extractable literal.
		{"plus on literal below min", `err+or`, false, "", false, false},
		// connection+timeout → Concat[Literal("connectio"), Plus(Literal("n")), Literal("timeout")]
		// "connectio" (9) > "timeout" (7), so "connectio" wins
		{"plus on longer literal", `connection+timeout`, false, "connectio", true, false},

		// Capture group is transparent
		{"capture group", `(error)\d+`, false, "error", true, false},
		{"nested capture", `((timeout))`, false, "timeout", true, false},

		// Repeat with Min >= 1
		{"repeat min 1", `x{1,3}timeout`, false, "timeout", true, false},
		{"repeat min 0", `x{0,3}timeout`, false, "timeout", true, false},

		// DotNL ((?s)) disables prefilter
		{"dot-s flag", `(?s).*timeout`, false, "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, ok := extractLiteral(tt.pattern, tt.ignoreCase)
			if ok != tt.wantOK {
				t.Errorf("extractLiteral(%q, %v) ok = %v, want %v", tt.pattern, tt.ignoreCase, ok, tt.wantOK)
				return
			}
			if !ok {
				return
			}
			if info.literal != tt.wantLit {
				t.Errorf("extractLiteral(%q, %v) literal = %q, want %q", tt.pattern, tt.ignoreCase, info.literal, tt.wantLit)
			}
			if info.ignoreCase != tt.wantCI {
				t.Errorf("extractLiteral(%q, %v) ignoreCase = %v, want %v", tt.pattern, tt.ignoreCase, info.ignoreCase, tt.wantCI)
			}
		})
	}
}

func TestRegexPrefilter_Correctness(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		ignoreCase bool
		input      string
		wantCount  int
		wantLines  []int
	}{
		{
			name:      "dot-star prefix",
			pattern:   ".*timeout",
			input:     "connection timeout\nall good\nread timeout\n",
			wantCount: 2,
			wantLines: []int{1, 3},
		},
		{
			name:      "literal not at match start",
			pattern:   `\d+error`,
			input:     "123error here\nno match\n456error there\n",
			wantCount: 2,
			wantLines: []int{1, 3},
		},
		{
			name:      "prefilter candidate but no regex match",
			pattern:   `^error\d+`,
			input:     "has error in middle\nerror123\n",
			wantCount: 1,
			wantLines: []int{2},
		},
		{
			name:       "case insensitive prefilter",
			pattern:    "timeout",
			ignoreCase: true,
			input:      "TIMEOUT\nall good\nTimeOut\n",
			wantCount:  2,
			wantLines:  []int{1, 3},
		},
		{
			name:      "no match despite literal present",
			pattern:   `error\d{3}`,
			input:     "error in system\nerror123\n",
			wantCount: 1,
			wantLines: []int{2},
		},
		{
			name:      "multiple matches same line",
			pattern:   `error\d+`,
			input:     "error1 and error2\nno match\n",
			wantCount: 1,
			wantLines: []int{1},
		},
		{
			name:      "no matches at all",
			pattern:   ".*timeout",
			input:     "hello\nworld\n",
			wantCount: 0,
		},
		{
			name:      "dense matches every line",
			pattern:   ".*the",
			input:     "the quick\nthe slow\nthe lazy\n",
			wantCount: 3,
			wantLines: []int{1, 2, 3},
		},
		{
			name:      "empty input",
			pattern:   ".*timeout",
			input:     "",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewRegexMatcher(tt.pattern, tt.ignoreCase, false)
			if err != nil {
				t.Fatalf("NewRegexMatcher(%q): %v", tt.pattern, err)
			}
			m.needLineNums = true

			data := []byte(tt.input)

			// Test FindAll
			ms := m.FindAll(data)
			if len(ms.Matches) != tt.wantCount {
				t.Errorf("FindAll: got %d matches, want %d", len(ms.Matches), tt.wantCount)
			}
			for i, wantLine := range tt.wantLines {
				if i < len(ms.Matches) && ms.Matches[i].LineNum != wantLine {
					t.Errorf("FindAll: match[%d].LineNum = %d, want %d", i, ms.Matches[i].LineNum, wantLine)
				}
			}

			// Test CountAll
			count := m.CountAll(data)
			if count != tt.wantCount {
				t.Errorf("CountAll: got %d, want %d", count, tt.wantCount)
			}

			// Test MatchExists
			exists := m.MatchExists(data)
			wantExists := tt.wantCount > 0
			if exists != wantExists {
				t.Errorf("MatchExists: got %v, want %v", exists, wantExists)
			}
		})
	}
}

// BenchmarkRegex_Prefilter_NoMatch benchmarks prefilter fast-reject on no-match data.
func BenchmarkRegex_Prefilter_NoMatch(b *testing.B) {
	data := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog\n"), 10000)
	m, _ := NewRegexMatcher(".*timeout", false, false)
	b.ResetTimer()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		m.FindAll(data)
	}
}

// BenchmarkRegex_Prefilter_Sparse benchmarks prefilter with 1% matching lines.
func BenchmarkRegex_Prefilter_Sparse(b *testing.B) {
	var buf []byte
	for i := range 10000 {
		if i%100 == 0 {
			buf = append(buf, []byte("ERROR: connection timeout at 2024-01-01\n")...)
		} else {
			buf = append(buf, []byte("the quick brown fox jumps over the lazy dog\n")...)
		}
	}
	m, _ := NewRegexMatcher(`.*timeout`, false, false)
	b.ResetTimer()
	b.SetBytes(int64(len(buf)))
	for b.Loop() {
		m.FindAll(buf)
	}
}

// BenchmarkRegex_Prefilter_Dense benchmarks prefilter when every line matches.
func BenchmarkRegex_Prefilter_Dense(b *testing.B) {
	data := bytes.Repeat([]byte("ERROR: connection timeout at port 8080\n"), 10000)
	m, _ := NewRegexMatcher(`.*timeout`, false, false)
	b.ResetTimer()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		m.FindAll(data)
	}
}

// BenchmarkRegex_Prefilter_MatchExists benchmarks -l mode with prefilter.
func BenchmarkRegex_Prefilter_MatchExists(b *testing.B) {
	var buf []byte
	for i := range 10000 {
		if i == 9999 {
			buf = append(buf, []byte("ERROR: connection timeout\n")...)
		} else {
			buf = append(buf, []byte("the quick brown fox jumps over the lazy dog\n")...)
		}
	}
	m, _ := NewRegexMatcher(`.*timeout`, false, false)
	b.ResetTimer()
	b.SetBytes(int64(len(buf)))
	for b.Loop() {
		m.MatchExists(buf)
	}
}

// BenchmarkRegex_NoPrefilter_Sparse benchmarks regex without extractable literal (baseline).
func BenchmarkRegex_NoPrefilter_Sparse(b *testing.B) {
	var buf []byte
	for i := range 10000 {
		if i%100 == 0 {
			buf = append(buf, []byte("2024-01-15 connection error\n")...)
		} else {
			buf = append(buf, []byte("the quick brown fox jumps over the lazy dog\n")...)
		}
	}
	m, _ := NewRegexMatcher(`\d{4}-\d{2}-\d{2}`, false, false)
	b.ResetTimer()
	b.SetBytes(int64(len(buf)))
	for b.Loop() {
		m.FindAll(buf)
	}
}
