package walker

import (
	"bytes"
	"testing"
)

func TestIsBinary(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"text only", []byte("hello world\nfoo bar\n"), false},
		{"empty", []byte{}, false},
		{"nul byte", []byte("hello\x00world"), true},
		{"nul at start", []byte{0, 'h', 'e', 'l', 'l', 'o'}, true},
		{"nul at 8KB boundary", append(bytes.Repeat([]byte("a"), 8191), 0), true},
		{"nul past 8KB", append(append(bytes.Repeat([]byte("a"), 8192), 'b'), 0), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsBinary(tt.data); got != tt.want {
				t.Errorf("IsBinary() = %v, want %v", got, tt.want)
			}
		})
	}
}
