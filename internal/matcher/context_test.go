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

	ms := m.FindAll([]byte("hello\nworld\nfoo\n"))
	// Should get: hello (match) + world (context)
	if len(ms.Matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(ms.Matches))
	}
	if ms.Matches[0].LineNum != 1 || ms.Matches[0].IsContext {
		t.Errorf("match[0]: LineNum=%d, IsContext=%v, want LineNum=1, IsContext=false", ms.Matches[0].LineNum, ms.Matches[0].IsContext)
	}
	if ms.Matches[1].LineNum != 2 || !ms.Matches[1].IsContext {
		t.Errorf("match[1]: LineNum=%d, IsContext=%v, want LineNum=2, IsContext=true", ms.Matches[1].LineNum, ms.Matches[1].IsContext)
	}
}

func TestContextMatcher_Before(t *testing.T) {
	inner, _ := NewRegexMatcher("foo", false, false)
	m := NewContextMatcher(inner, 1, 0)

	ms := m.FindAll([]byte("hello\nworld\nfoo\nbar\n"))
	// Should get: world (context) + foo (match)
	if len(ms.Matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(ms.Matches))
	}
	if ms.Matches[0].LineNum != 2 || !ms.Matches[0].IsContext {
		t.Errorf("match[0]: LineNum=%d, IsContext=%v, want LineNum=2, IsContext=true", ms.Matches[0].LineNum, ms.Matches[0].IsContext)
	}
	if ms.Matches[1].LineNum != 3 || ms.Matches[1].IsContext {
		t.Errorf("match[1]: LineNum=%d, IsContext=%v, want LineNum=3, IsContext=false", ms.Matches[1].LineNum, ms.Matches[1].IsContext)
	}
}

func TestContextMatcher_BeforeAndAfter(t *testing.T) {
	inner, _ := NewRegexMatcher("middle", false, false)
	m := NewContextMatcher(inner, 1, 1)

	ms := m.FindAll([]byte("a\nb\nmiddle\nd\ne\n"))
	// Should get: b (context) + middle (match) + d (context)
	if len(ms.Matches) != 3 {
		t.Fatalf("got %d matches, want 3", len(ms.Matches))
	}
	if ms.Matches[0].LineNum != 2 || !ms.Matches[0].IsContext {
		t.Errorf("match[0]: LineNum=%d, IsContext=%v", ms.Matches[0].LineNum, ms.Matches[0].IsContext)
	}
	if ms.Matches[1].LineNum != 3 || ms.Matches[1].IsContext {
		t.Errorf("match[1]: LineNum=%d, IsContext=%v", ms.Matches[1].LineNum, ms.Matches[1].IsContext)
	}
	if ms.Matches[2].LineNum != 4 || !ms.Matches[2].IsContext {
		t.Errorf("match[2]: LineNum=%d, IsContext=%v", ms.Matches[2].LineNum, ms.Matches[2].IsContext)
	}
}

func TestContextMatcher_Separator(t *testing.T) {
	inner, _ := NewRegexMatcher("match", false, false)
	m := NewContextMatcher(inner, 0, 0)
	// With context=0 returns inner directly, use context=1 with distant matches
	m = NewContextMatcher(inner, 0, 1)

	// Two matches far apart should have a separator
	ms := m.FindAll([]byte("match\na\nb\nc\nmatch\nd\n"))
	// match(1) + a(context) + separator + match(5) + d(context)
	hasSeparator := false
	for _, match := range ms.Matches {
		if match.LineNum == 0 && match.LineStart == -1 {
			hasSeparator = true
		}
	}
	if !hasSeparator {
		t.Errorf("expected separator between non-contiguous groups, got %d matches", len(ms.Matches))
	}
}

func TestContextMatcher_NoMatch(t *testing.T) {
	inner, _ := NewRegexMatcher("xyz", false, false)
	m := NewContextMatcher(inner, 2, 2)

	ms := m.FindAll([]byte("hello\nworld\n"))
	if len(ms.Matches) != 0 {
		t.Errorf("got %d matches, want 0", len(ms.Matches))
	}
}

func TestContextMatcher_OverlappingContext(t *testing.T) {
	inner, _ := NewRegexMatcher("x", false, false)
	m := NewContextMatcher(inner, 1, 1)

	// Two adjacent matches — context should not duplicate lines
	ms := m.FindAll([]byte("a\nxb\nxc\nd\n"))
	// a(ctx) + xb(match) + xc(match) + d(ctx) — no separator, no duplicates
	lineNums := make([]int, 0, len(ms.Matches))
	for _, match := range ms.Matches {
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
	ms, ok := m.FindLine([]byte("this is a test"), 5, 100)
	if !ok {
		t.Fatal("expected match")
	}
	if ms.Matches[0].LineNum != 5 {
		t.Errorf("LineNum = %d, want 5", ms.Matches[0].LineNum)
	}
}
