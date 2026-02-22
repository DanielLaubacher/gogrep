package output

import (
	"testing"

	"github.com/dl/gogrep/internal/matcher"
)

func TestTextFormatter_SingleFile(t *testing.T) {
	f := NewTextFormatter(true, false, false, false)
	result := Result{
		FilePath: "test.txt",
		Matches: []matcher.Match{
			{LineNum: 1, LineBytes: []byte("hello world"), Positions: [][2]int{{0, 5}}},
			{LineNum: 3, LineBytes: []byte("hello again"), Positions: [][2]int{{0, 5}}},
		},
	}

	got := string(f.Format(nil, result, false))
	want := "1:hello world\n3:hello again\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTextFormatter_MultiFile(t *testing.T) {
	f := NewTextFormatter(true, false, false, false)
	result := Result{
		FilePath: "test.txt",
		Matches: []matcher.Match{
			{LineNum: 5, LineBytes: []byte("match line")},
		},
	}

	got := string(f.Format(nil, result, true))
	want := "test.txt:5:match line\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTextFormatter_CountOnly(t *testing.T) {
	f := NewTextFormatter(false, true, false, false)
	result := Result{
		FilePath: "test.txt",
		Matches:  make([]matcher.Match, 3),
	}

	// Single file
	got := string(f.Format(nil, result, false))
	if got != "3\n" {
		t.Errorf("count single: got %q, want %q", got, "3\n")
	}

	// Multi file
	got = string(f.Format(nil, result, true))
	if got != "test.txt:3\n" {
		t.Errorf("count multi: got %q, want %q", got, "test.txt:3\n")
	}
}

func TestTextFormatter_FilesOnly(t *testing.T) {
	f := NewTextFormatter(false, false, true, false)

	// Has matches
	result := Result{
		FilePath: "test.txt",
		Matches:  make([]matcher.Match, 1),
	}
	got := string(f.Format(nil, result, true))
	if got != "test.txt\n" {
		t.Errorf("got %q, want %q", got, "test.txt\n")
	}

	// No matches
	result.Matches = nil
	got = string(f.Format(nil, result, true))
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
