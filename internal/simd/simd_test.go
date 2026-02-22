package simd

import (
	"bytes"
	"testing"
)

func TestIndexByte(t *testing.T) {
	tests := []struct {
		name string
		data string
		c    byte
		want int
	}{
		{"empty", "", 'a', -1},
		{"single found", "a", 'a', 0},
		{"single not found", "b", 'a', -1},
		{"at start", "abc", 'a', 0},
		{"at end", "abc", 'c', 2},
		{"middle", "abc", 'b', 1},
		{"not found", "abc", 'z', -1},
		{"newline", "hello\nworld", '\n', 5},
		{"null byte", "abc\x00def", 0, 3},
		// Test sizes around the 32-byte boundary
		{"31 bytes found at end", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaab", 'b', 30},
		{"32 bytes found at end", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaX", 'X', 31},
		{"33 bytes found at end", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaX", 'X', 32},
		{"64 bytes found at 33", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" + "Xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 'X', 33},
		{"100 bytes found at 99", string(make([]byte, 99)) + "X", 'X', 99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IndexByte([]byte(tt.data), tt.c)
			want := bytes.IndexByte([]byte(tt.data), tt.c)
			if got != want {
				t.Errorf("IndexByte(%q, %q) = %d, want %d (bytes.IndexByte = %d)", tt.data, tt.c, got, want, tt.want)
			}
		})
	}
}

func TestIndexByte_LargeData(t *testing.T) {
	// Test with large data to exercise the SIMD loop
	sizes := []int{31, 32, 33, 63, 64, 65, 100, 255, 256, 1000, 4096, 65536}
	for _, size := range sizes {
		data := make([]byte, size)
		for i := range data {
			data[i] = 'a'
		}

		// Target at various positions
		positions := []int{0, size / 4, size / 2, size - 1}
		for _, pos := range positions {
			data[pos] = 'X'
			got := IndexByte(data, 'X')
			want := bytes.IndexByte(data, 'X')
			if got != want {
				t.Errorf("size=%d pos=%d: IndexByte = %d, want %d", size, pos, got, want)
			}
			data[pos] = 'a' // restore
		}

		// Not found
		got := IndexByte(data, 'Z')
		if got != -1 {
			t.Errorf("size=%d not found: IndexByte = %d, want -1", size, got)
		}
	}
}

func TestLastIndexByte(t *testing.T) {
	tests := []struct {
		name string
		data string
		c    byte
		want int
	}{
		{"empty", "", 'a', -1},
		{"single found", "a", 'a', 0},
		{"single not found", "b", 'a', -1},
		{"at start", "abc", 'a', 0},
		{"at end", "abc", 'c', 2},
		{"duplicate last", "abac", 'a', 2},
		{"not found", "abc", 'z', -1},
		{"newline last", "hello\nworld\n", '\n', 11},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LastIndexByte([]byte(tt.data), tt.c)
			want := bytes.LastIndexByte([]byte(tt.data), tt.c)
			if got != want {
				t.Errorf("LastIndexByte(%q, %q) = %d, want %d", tt.data, tt.c, got, want)
			}
		})
	}
}

func TestLastIndexByte_LargeData(t *testing.T) {
	sizes := []int{31, 32, 33, 63, 64, 65, 100, 256, 1000, 4096}
	for _, size := range sizes {
		data := make([]byte, size)
		for i := range data {
			data[i] = 'a'
		}

		positions := []int{0, size / 4, size / 2, size - 1}
		for _, pos := range positions {
			data[pos] = 'X'
			got := LastIndexByte(data, 'X')
			want := bytes.LastIndexByte(data, 'X')
			if got != want {
				t.Errorf("size=%d pos=%d: LastIndexByte = %d, want %d", size, pos, got, want)
			}
			data[pos] = 'a'
		}
	}
}

func TestCount(t *testing.T) {
	tests := []struct {
		name string
		data string
		c    byte
		want int
	}{
		{"empty", "", 'a', 0},
		{"single found", "a", 'a', 1},
		{"single not found", "b", 'a', 0},
		{"multiple", "abacada", 'a', 4},
		{"all same", "aaaa", 'a', 4},
		{"none", "bbbb", 'a', 0},
		{"newlines", "a\nb\nc\n", '\n', 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Count([]byte(tt.data), tt.c)
			want := bytes.Count([]byte(tt.data), []byte{tt.c})
			if got != want {
				t.Errorf("Count(%q, %q) = %d, want %d", tt.data, tt.c, got, want)
			}
		})
	}
}

func TestCount_LargeData(t *testing.T) {
	sizes := []int{31, 32, 33, 64, 100, 256, 1000, 4096, 65536}
	for _, size := range sizes {
		// Every 3rd byte is the target
		data := make([]byte, size)
		expected := 0
		for i := range data {
			if i%3 == 0 {
				data[i] = 'X'
				expected++
			} else {
				data[i] = 'a'
			}
		}

		got := Count(data, 'X')
		if got != expected {
			t.Errorf("size=%d: Count = %d, want %d", size, got, expected)
		}
	}
}

// Benchmarks

func BenchmarkIndexByte_SIMD(b *testing.B) {
	data := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog\n"), 10000)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for b.Loop() {
		IndexByte(data, '\n')
	}
}

func BenchmarkIndexByte_Stdlib(b *testing.B) {
	data := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog\n"), 10000)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for b.Loop() {
		bytes.IndexByte(data, '\n')
	}
}

func BenchmarkLastIndexByte_SIMD(b *testing.B) {
	data := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog\n"), 10000)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for b.Loop() {
		LastIndexByte(data, '\n')
	}
}

func BenchmarkLastIndexByte_Stdlib(b *testing.B) {
	data := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog\n"), 10000)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for b.Loop() {
		bytes.LastIndexByte(data, '\n')
	}
}

func BenchmarkCount_SIMD(b *testing.B) {
	data := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog\n"), 10000)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for b.Loop() {
		Count(data, '\n')
	}
}

func BenchmarkCount_Stdlib(b *testing.B) {
	data := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog\n"), 10000)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for b.Loop() {
		bytes.Count(data, []byte{'\n'})
	}
}

// Benchmark searching for a byte that's far into the buffer (worst case for IndexByte)
func BenchmarkIndexByte_SIMD_Far(b *testing.B) {
	data := make([]byte, 440000)
	for i := range data {
		data[i] = 'a'
	}
	data[len(data)-1] = 'X'
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for b.Loop() {
		IndexByte(data, 'X')
	}
}

func BenchmarkIndexByte_Stdlib_Far(b *testing.B) {
	data := make([]byte, 440000)
	for i := range data {
		data[i] = 'a'
	}
	data[len(data)-1] = 'X'
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for b.Loop() {
		bytes.IndexByte(data, 'X')
	}
}
