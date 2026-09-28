package xdg

import (
	"errors"
	"os"
	"path/filepath"
)

// DataDir returns the base directory for slk data, honoring XDG_DATA_HOME.
// It returns an absolute path or an error if neither XDG_DATA_HOME nor HOME is set.
func DataDir() (string, error) {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "slk"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.New("neither XDG_DATA_HOME nor HOME is set")
	}
	return filepath.Join(home, ".local", "share", "slk"), nil
}
