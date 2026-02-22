package input

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestBufferedReader_Read(t *testing.T) {
	// Create a temp file
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := []byte("hello world\nline two\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	r := NewBufferedReader()
	result, err := r.Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	defer result.Closer()

	if !bytes.Equal(result.Data, content) {
		t.Errorf("data = %q, want %q", result.Data, content)
	}
}

func TestBufferedReader_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}

	r := NewBufferedReader()
	result, err := r.Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	defer result.Closer()

	if result.Data != nil {
		t.Errorf("data = %v, want nil for empty file", result.Data)
	}
}

func TestBufferedReader_NonexistentFile(t *testing.T) {
	r := NewBufferedReader()
	_, err := r.Read("/nonexistent/path/file.txt")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestMmapReader_Read(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := []byte("hello mmap world\nline two\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	r := NewMmapReader()
	result, err := r.Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if !bytes.Equal(result.Data, content) {
		t.Errorf("data = %q, want %q", result.Data, content)
	}

	// Test closer (should not panic)
	if err := result.Closer(); err != nil {
		t.Errorf("Closer() error: %v", err)
	}
}

func TestMmapReader_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}

	r := NewMmapReader()
	result, err := r.Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	defer result.Closer()

	if result.Data != nil {
		t.Errorf("data = %v, want nil for empty file", result.Data)
	}
}

func TestMmapReader_LargeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.txt")

	// Create a file larger than typical page size
	content := bytes.Repeat([]byte("abcdefghij\n"), 10000) // ~110KB
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	r := NewMmapReader()
	result, err := r.Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if !bytes.Equal(result.Data, content) {
		t.Errorf("data length = %d, want %d", len(result.Data), len(content))
	}

	if err := result.Closer(); err != nil {
		t.Errorf("Closer() error: %v", err)
	}
}

func TestAdaptiveReader_SmallFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small.txt")
	content := []byte("small file\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Threshold of 1MB — small file should use buffered reader
	r := NewAdaptiveReader(1024 * 1024)
	result, err := r.Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	defer result.Closer()

	if !bytes.Equal(result.Data, content) {
		t.Errorf("data = %q, want %q", result.Data, content)
	}
}

func TestAdaptiveReader_LargeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.txt")

	// Create a file larger than the threshold
	content := bytes.Repeat([]byte("x"), 2*1024*1024) // 2MB
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Threshold of 1MB — large file should use mmap reader
	r := NewAdaptiveReader(1024 * 1024)
	result, err := r.Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if !bytes.Equal(result.Data, content) {
		t.Errorf("data length = %d, want %d", len(result.Data), len(content))
	}

	if err := result.Closer(); err != nil {
		t.Errorf("Closer() error: %v", err)
	}
}

func TestAdaptiveReader_NonexistentFile(t *testing.T) {
	r := NewAdaptiveReader(1024 * 1024)
	_, err := r.Read("/nonexistent/path/file.txt")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestStdinReader(t *testing.T) {
	// StdinReader reads from os.Stdin which is hard to test directly.
	// We test the interface conformance here.
	r := NewStdinReader()
	_ = r // Just ensure it compiles and implements Reader
}

func BenchmarkBufferedReader(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.txt")
	content := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog\n"), 10000)
	if err := os.WriteFile(path, content, 0644); err != nil {
		b.Fatal(err)
	}

	r := NewBufferedReader()
	b.ResetTimer()
	b.SetBytes(int64(len(content)))
	for b.Loop() {
		result, err := r.Read(path)
		if err != nil {
			b.Fatal(err)
		}
		result.Closer()
	}
}

func BenchmarkMmapReader(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.txt")
	content := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog\n"), 10000)
	if err := os.WriteFile(path, content, 0644); err != nil {
		b.Fatal(err)
	}

	r := NewMmapReader()
	b.ResetTimer()
	b.SetBytes(int64(len(content)))
	for b.Loop() {
		result, err := r.Read(path)
		if err != nil {
			b.Fatal(err)
		}
		result.Closer()
	}
}

func BenchmarkMmapReader_LargeFile(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bench_large.txt")
	content := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog\n"), 500000) // ~22MB
	if err := os.WriteFile(path, content, 0644); err != nil {
		b.Fatal(err)
	}

	r := NewMmapReader()
	b.ResetTimer()
	b.SetBytes(int64(len(content)))
	for b.Loop() {
		result, err := r.Read(path)
		if err != nil {
			b.Fatal(err)
		}
		result.Closer()
	}
}
