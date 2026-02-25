package tmuxconfig

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed tmux.conf
var defaultTmuxConf []byte

// EnsureDefault creates a default tmux config if the user does not already
// have one.  It checks both ~/.tmux.conf and the XDG location; if either
// exists it returns (false, "", nil).  When neither exists it writes the
// embedded sample config to the XDG path and returns (true, path, nil).
func EnsureDefault() (bool, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, "", fmt.Errorf("determine home directory: %w", err)
	}

	// Legacy location.
	legacy := filepath.Join(home, ".tmux.conf")
	if fileExists(legacy) {
		return false, "", nil
	}

	// XDG location.
	xdgBase := os.Getenv("XDG_CONFIG_HOME")
	if xdgBase == "" {
		xdgBase = filepath.Join(home, ".config")
	}
	xdgPath := filepath.Join(xdgBase, "tmux", "tmux.conf")
	if fileExists(xdgPath) {
		return false, "", nil
	}

	// Neither exists — write to the XDG path.
	if err := os.MkdirAll(filepath.Dir(xdgPath), 0o755); err != nil {
		return false, "", fmt.Errorf("create tmux config directory: %w", err)
	}

	f, err := os.OpenFile(xdgPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return false, "", fmt.Errorf("create tmux config %q: %w", xdgPath, err)
	}
	defer f.Close()

	if _, err := f.Write(defaultTmuxConf); err != nil {
		return false, "", fmt.Errorf("write tmux config: %w", err)
	}

	return true, xdgPath, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
