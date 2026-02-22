package walker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIgnoreStack_BasicMatching(t *testing.T) {
	// Create a temp directory with a .gitignore
	dir := t.TempDir()
	gitignore := filepath.Join(dir, ".gitignore")
	os.WriteFile(gitignore, []byte("*.log\nbuild/\n!important.log\n"), 0644)

	s := newIgnoreStack()
	s.push(dir)

	tests := []struct {
		name  string
		path  string
		isDir bool
		want  bool
	}{
		{"matches glob", filepath.Join(dir, "app.log"), false, true},
		{"no match", filepath.Join(dir, "app.txt"), false, false},
		{"dir pattern matches dir", filepath.Join(dir, "build"), true, true},
		{"dir pattern skips file", filepath.Join(dir, "build"), false, false},
		{"negation", filepath.Join(dir, "important.log"), false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.isIgnored(tt.path, tt.isDir)
			if got != tt.want {
				t.Errorf("isIgnored(%q, isDir=%v) = %v, want %v", tt.path, tt.isDir, got, tt.want)
			}
		})
	}

	s.pop()
}

func TestIgnoreStack_NestedGitignore(t *testing.T) {
	// Create nested directories with .gitignore files
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	os.Mkdir(sub, 0755)

	os.WriteFile(filepath.Join(root, ".gitignore"), []byte("*.tmp\n"), 0644)
	os.WriteFile(filepath.Join(sub, ".gitignore"), []byte("*.dat\n"), 0644)

	s := newIgnoreStack()
	s.push(root)
	s.push(sub)

	// Root rule applies
	if !s.isIgnored(filepath.Join(root, "test.tmp"), false) {
		t.Error("expected root .gitignore to match *.tmp")
	}

	// Sub rule applies
	if !s.isIgnored(filepath.Join(sub, "test.dat"), false) {
		t.Error("expected sub .gitignore to match *.dat")
	}

	// Neither matches
	if s.isIgnored(filepath.Join(sub, "test.txt"), false) {
		t.Error("expected test.txt to not be ignored")
	}

	s.pop()
	s.pop()
}

func TestIgnoreStack_NoGitignore(t *testing.T) {
	dir := t.TempDir()
	s := newIgnoreStack()
	s.push(dir) // no .gitignore file exists

	// Should not ignore anything
	if s.isIgnored(filepath.Join(dir, "anything.txt"), false) {
		t.Error("expected no ignoring when .gitignore doesn't exist")
	}

	s.pop()
}
