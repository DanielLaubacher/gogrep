package matcher

import (
	"testing"
)

func TestContextMatcher_NoContext(t *testing.T) {
	inner, _ := NewRegexMatcher("hello", false, false)
	m := NewContextMatcher(inner, 0, 0)
	// Should return the inner matcher directly
	if _, ok := m.(*ContextMatcher); ok {
		t.Error("expected inner matcher to be returned when before=0 and after=0")
	}
}

func TestContextMatcher_After(t *testing.T) {
	inner, _ := NewRegexMatcher("hello", false, false)
	m := NewContextMatcher(inner, 0, 1)

	matches := m.FindAll([]byte("hello\nworld\nfoo\n"))
	// Should get: hello (match) + world (context)
	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(matches))
	}
	if matches[0].LineNum != 1 || matches[0].IsContext {
		t.Errorf("match[0]: LineNum=%d, IsContext=%v, want LineNum=1, IsContext=false", matches[0].LineNum, matches[0].IsContext)
	}
	if matches[1].LineNum != 2 || !matches[1].IsContext {
		t.Errorf("match[1]: LineNum=%d, IsContext=%v, want LineNum=2, IsContext=true", matches[1].LineNum, matches[1].IsContext)
	}
}

func TestContextMatcher_Before(t *testing.T) {
	inner, _ := NewRegexMatcher("foo", false, false)
	m := NewContextMatcher(inner, 1, 0)

	matches := m.FindAll([]byte("hello\nworld\nfoo\nbar\n"))
	// Should get: world (context) + foo (match)
	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(matches))
	}
	if matches[0].LineNum != 2 || !matches[0].IsContext {
		t.Errorf("match[0]: LineNum=%d, IsContext=%v, want LineNum=2, IsContext=true", matches[0].LineNum, matches[0].IsContext)
	}
	if matches[1].LineNum != 3 || matches[1].IsContext {
		t.Errorf("match[1]: LineNum=%d, IsContext=%v, want LineNum=3, IsContext=false", matches[1].LineNum, matches[1].IsContext)
	}
}

func TestContextMatcher_BeforeAndAfter(t *testing.T) {
	inner, _ := NewRegexMatcher("middle", false, false)
	m := NewContextMatcher(inner, 1, 1)

	matches := m.FindAll([]byte("a\nb\nmiddle\nd\ne\n"))
	// Should get: b (context) + middle (match) + d (context)
	if len(matches) != 3 {
		t.Fatalf("got %d matches, want 3", len(matches))
	}
	if matches[0].LineNum != 2 || !matches[0].IsContext {
		t.Errorf("match[0]: LineNum=%d, IsContext=%v", matches[0].LineNum, matches[0].IsContext)
	}
	if matches[1].LineNum != 3 || matches[1].IsContext {
		t.Errorf("match[1]: LineNum=%d, IsContext=%v", matches[1].LineNum, matches[1].IsContext)
	}
	if matches[2].LineNum != 4 || !matches[2].IsContext {
		t.Errorf("match[2]: LineNum=%d, IsContext=%v", matches[2].LineNum, matches[2].IsContext)
	}
}

func TestContextMatcher_Separator(t *testing.T) {
	inner, _ := NewRegexMatcher("match", false, false)
	m := NewContextMatcher(inner, 0, 0)
	// With context=0 returns inner directly, use context=1 with distant matches
	m = NewContextMatcher(inner, 0, 1)

	// Two matches far apart should have a separator
	matches := m.FindAll([]byte("match\na\nb\nc\nmatch\nd\n"))
	// match(1) + a(context) + separator + match(5) + d(context)
	hasSeparator := false
	for _, match := range matches {
		if match.LineNum == 0 && string(match.LineBytes) == "--" {
			hasSeparator = true
		}
	}
	if !hasSeparator {
		t.Errorf("expected separator between non-contiguous groups, got matches: %v", matches)
	}
}

func TestContextMatcher_NoMatch(t *testing.T) {
	inner, _ := NewRegexMatcher("xyz", false, false)
	m := NewContextMatcher(inner, 2, 2)

	matches := m.FindAll([]byte("hello\nworld\n"))
	if len(matches) != 0 {
		t.Errorf("got %d matches, want 0", len(matches))
	}
}

func TestContextMatcher_OverlappingContext(t *testing.T) {
	inner, _ := NewRegexMatcher("x", false, false)
	m := NewContextMatcher(inner, 1, 1)

	// Two adjacent matches — context should not duplicate lines
	matches := m.FindAll([]byte("a\nxb\nxc\nd\n"))
	// a(ctx) + xb(match) + xc(match) + d(ctx) — no separator, no duplicates
	lineNums := make([]int, 0, len(matches))
	for _, match := range matches {
		lineNums = append(lineNums, match.LineNum)
	}
	expected := []int{1, 2, 3, 4}
	if len(lineNums) != len(expected) {
		t.Fatalf("got lines %v, want %v", lineNums, expected)
	}
	for i := range expected {
		if lineNums[i] != expected[i] {
			t.Errorf("line[%d] = %d, want %d", i, lineNums[i], expected[i])
		}
	}
}

func TestContextMatcher_FindLine(t *testing.T) {
	inner, _ := NewRegexMatcher("test", false, false)
	m := NewContextMatcher(inner, 2, 2)

	// FindLine delegates to inner
	match, ok := m.FindLine([]byte("this is a test"), 5, 100)
	if !ok {
		t.Fatal("expected match")
	}
	if match.LineNum != 5 {
		t.Errorf("LineNum = %d, want 5", match.LineNum)
	}
}
