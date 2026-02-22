package watch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcher_CreateAndClose(t *testing.T) {
	w, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
}

func TestWatcher_AddFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("initial content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	w, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if err := w.Add(path); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
}

func TestWatcher_DetectModify(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("initial\n"), 0644); err != nil {
		t.Fatal(err)
	}

	w, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if err := w.Add(path); err != nil {
		t.Fatal(err)
	}

	events := w.Events()

	// Modify the file after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		f.WriteString("new line\n")
		f.Close()
	}()

	// Wait for the event
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()

	select {
	case evt := <-events:
		if evt.Err != nil {
			t.Fatalf("event error: %v", evt.Err)
		}
		if evt.Type != EventModified {
			t.Errorf("event type = %d, want EventModified(%d)", evt.Type, EventModified)
		}
	case <-timer.C:
		t.Fatal("timeout waiting for modify event")
	}
}

func TestWatcher_ReadNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	initialContent := "initial content\n"
	if err := os.WriteFile(path, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	w, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if err := w.Add(path); err != nil {
		t.Fatal(err)
	}

	// Append new content
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	newContent := "new line\n"
	f.WriteString(newContent)
	f.Close()

	// ReadNew should return only the new content
	data, err := w.ReadNew(path)
	if err != nil {
		t.Fatalf("ReadNew() error: %v", err)
	}
	if string(data) != newContent {
		t.Errorf("got %q, want %q", string(data), newContent)
	}
}

func TestWatcher_ReadNew_Truncated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("initial content with lots of data\n"), 0644); err != nil {
		t.Fatal(err)
	}

	w, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if err := w.Add(path); err != nil {
		t.Fatal(err)
	}

	// Truncate and write smaller content (simulates log rotation)
	if err := os.WriteFile(path, []byte("new\n"), 0644); err != nil {
		t.Fatal(err)
	}

	data, err := w.ReadNew(path)
	if err != nil {
		t.Fatalf("ReadNew() error: %v", err)
	}
	if string(data) != "new\n" {
		t.Errorf("got %q, want %q", string(data), "new\n")
	}
}

func TestWatcher_DetectCreate(t *testing.T) {
	dir := t.TempDir()

	w, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	// Watch the directory
	if err := w.Add(dir); err != nil {
		t.Fatal(err)
	}

	events := w.Events()

	// Create a new file
	go func() {
		time.Sleep(50 * time.Millisecond)
		os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello\n"), 0644)
	}()

	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()

	select {
	case evt := <-events:
		if evt.Err != nil {
			t.Fatalf("event error: %v", evt.Err)
		}
		if evt.Type != EventCreated {
			t.Errorf("event type = %d, want EventCreated(%d)", evt.Type, EventCreated)
		}
	case <-timer.C:
		t.Fatal("timeout waiting for create event")
	}
}

func TestParseEvents(t *testing.T) {
	w := &Watcher{
		watches: map[int]string{1: "/tmp/test"},
	}

	// Manually construct an inotify event buffer
	// wd=1, mask=IN_MODIFY, cookie=0, len=0
	buf := make([]byte, inotifyEventSize)
	buf[0] = 1 // wd (little-endian int32)
	buf[4] = byte(0x02) // IN_MODIFY = 0x02

	ch := make(chan Event, 1)
	w.parseEvents(buf, ch)

	select {
	case evt := <-ch:
		if evt.Type != EventModified {
			t.Errorf("event type = %d, want EventModified", evt.Type)
		}
		if evt.Path != "/tmp/test" {
			t.Errorf("path = %q, want /tmp/test", evt.Path)
		}
	default:
		t.Error("no event received")
	}
}
