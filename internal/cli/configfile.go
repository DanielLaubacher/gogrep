package cli

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// LoadConfigArgs reads the gogrep config file and returns parsed arguments.
// Config file location: GOGREP_CONFIG_PATH env var, or ~/.gogrep.
// Format: one flag per line, # comments, empty lines ignored.
// Returns nil if no config file found.
func LoadConfigArgs() []string {
	path := os.Getenv("GOGREP_CONFIG_PATH")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		path = filepath.Join(home, ".gogrep")
	}

	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var args []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		args = append(args, line)
	}
	return args
}
