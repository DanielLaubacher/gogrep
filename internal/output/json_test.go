package output

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dl/gogrep/internal/matcher"
)

func TestJSONFormatter_BasicMatch(t *testing.T) {
	f := NewJSONFormatter()
	result := Result{
		FilePath: "test.txt",
		Matches: []matcher.Match{
			{
				LineNum:    1,
				LineBytes:  []byte("hello world"),
				ByteOffset: 0,
				Positions:  [][2]int{{0, 5}},
			},
		},
	}

	got := string(f.Format(nil, result, false))
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}

	var jm map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &jm); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if jm["type"] != "match" {
		t.Errorf("type = %v, want match", jm["type"])
	}
	if jm["file"] != "test.txt" {
		t.Errorf("file = %v, want test.txt", jm["file"])
	}
	if jm["text"] != "hello world" {
		t.Errorf("text = %v, want hello world", jm["text"])
	}
	if jm["line_number"].(float64) != 1 {
		t.Errorf("line_number = %v, want 1", jm["line_number"])
	}
}

func TestJSONFormatter_MultipleMatches(t *testing.T) {
	f := NewJSONFormatter()
	result := Result{
		FilePath: "test.txt",
		Matches: []matcher.Match{
			{LineNum: 1, LineBytes: []byte("first"), ByteOffset: 0, Positions: [][2]int{{0, 5}}},
			{LineNum: 3, LineBytes: []byte("third"), ByteOffset: 20, Positions: [][2]int{{0, 5}}},
		},
	}

	got := string(f.Format(nil, result, true))
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}

	// Verify each line is valid JSON
	for i, line := range lines {
		var jm map[string]interface{}
		if err := json.Unmarshal([]byte(line), &jm); err != nil {
			t.Errorf("line %d: invalid JSON: %v", i, err)
		}
	}
}

func TestJSONFormatter_ContextLinesSkipped(t *testing.T) {
	f := NewJSONFormatter()
	result := Result{
		FilePath: "test.txt",
		Matches: []matcher.Match{
			{LineNum: 1, LineBytes: []byte("context"), IsContext: true},
			{LineNum: 2, LineBytes: []byte("match"), Positions: [][2]int{{0, 5}}},
			{LineNum: 3, LineBytes: []byte("context"), IsContext: true},
		},
	}

	got := string(f.Format(nil, result, false))
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1 (context should be skipped)", len(lines))
	}
}

func TestJSONFormatter_NoMatches(t *testing.T) {
	f := NewJSONFormatter()
	result := Result{
		FilePath: "test.txt",
		Matches:  nil,
	}

	got := f.Format(nil, result, false)
	if got != nil {
		t.Errorf("got %q, want nil for no matches", got)
	}
}


func TestJSONFormatter_MatchPositions(t *testing.T) {
	f := NewJSONFormatter()
	result := Result{
		FilePath: "test.txt",
		Matches: []matcher.Match{
			{
				LineNum:    1,
				LineBytes:  []byte("hello world hello"),
				ByteOffset: 0,
				Positions:  [][2]int{{0, 5}, {12, 17}},
			},
		},
	}

	got := string(f.Format(nil, result, false))
	var jm map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(got)), &jm); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	matches := jm["matches"].([]interface{})
	if len(matches) != 2 {
		t.Fatalf("got %d match positions, want 2", len(matches))
	}

	pos0 := matches[0].(map[string]interface{})
	if pos0["start"].(float64) != 0 || pos0["end"].(float64) != 5 {
		t.Errorf("position[0] = %v, want {start:0, end:5}", pos0)
	}
}
