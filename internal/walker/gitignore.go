package walker

import (
	"path/filepath"

	ignore "github.com/sabhiram/go-gitignore"
)

// ignoreStack tracks .gitignore rules as we descend into directories.
// Each layer corresponds to a directory that contains a .gitignore file.
type ignoreStack struct {
	layers []ignoreLayer
}

type ignoreLayer struct {
	dir    string
	parser *ignore.GitIgnore
}

func newIgnoreStack() *ignoreStack {
	return &ignoreStack{}
}

// push loads .gitignore from a directory and pushes its rules onto the stack.
func (s *ignoreStack) push(dir string) {
	gitignorePath := filepath.Join(dir, ".gitignore")
	parser, err := ignore.CompileIgnoreFile(gitignorePath)
	if err != nil {
		// No .gitignore or parse error — push nil layer to maintain stack depth
		s.layers = append(s.layers, ignoreLayer{dir: dir, parser: nil})
		return
	}
	s.layers = append(s.layers, ignoreLayer{dir: dir, parser: parser})
}

// pop removes the top layer.
func (s *ignoreStack) pop() {
	if len(s.layers) > 0 {
		s.layers = s.layers[:len(s.layers)-1]
	}
}

// isIgnored checks if a path should be ignored by any active .gitignore layer.
func (s *ignoreStack) isIgnored(fullPath string, isDir bool) bool {
	return isIgnoredByLayers(s.layers, fullPath, isDir)
}

// cloneLayers returns a copy of the current layers slice.
// The underlying *GitIgnore parsers are immutable and shared safely across goroutines.
func (s *ignoreStack) cloneLayers() []ignoreLayer {
	if s == nil || len(s.layers) == 0 {
		return nil
	}
	c := make([]ignoreLayer, len(s.layers))
	copy(c, s.layers)
	return c
}

// loadIgnoreLayer loads and compiles a .gitignore from the given directory.
// Returns a layer with nil parser if no .gitignore exists or on parse error.
func loadIgnoreLayer(dir string) ignoreLayer {
	var path string
	if len(dir) > 0 && dir[len(dir)-1] == '/' {
		path = dir + ".gitignore"
	} else {
		path = dir + "/.gitignore"
	}
	parser, err := ignore.CompileIgnoreFile(path)
	if err != nil {
		return ignoreLayer{dir: dir, parser: nil}
	}
	return ignoreLayer{dir: dir, parser: parser}
}

// isIgnoredByLayers checks if a path should be ignored by any layer in the slice.
func isIgnoredByLayers(layers []ignoreLayer, fullPath string, isDir bool) bool {
	for _, layer := range layers {
		if layer.parser == nil {
			continue
		}
		rel, err := filepath.Rel(layer.dir, fullPath)
		if err != nil {
			continue
		}
		checkPath := rel
		if isDir {
			checkPath = rel + "/"
		}
		if layer.parser.MatchesPath(checkPath) {
			return true
		}
	}
	return false
}
