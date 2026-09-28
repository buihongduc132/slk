package main

import (
	"os"
	"path/filepath"

	"github.com/gammons/slk/internal/xdg"
)

func xdgConfig() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "slk")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "slk")
}

func xdgData() string {
	dir, err := xdg.DataDir()
	if err != nil {
		// Deliberately swallowing the error for now, as existing call sites
		// do not expect one. This will leave callers joining onto an empty
		// string if resolution fails.
		return ""
	}
	return dir
}

func xdgCache() string {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "slk")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "slk")
}
