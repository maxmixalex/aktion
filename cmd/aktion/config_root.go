package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func resolveConfigRoot(root string) (string, error) {
	if root != "" {
		return filepath.Abs(root)
	}
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate executable for configuration root (use --root to override): %w", err)
	}
	// A symlink in PATH should use the installation's config, not the link directory.
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return "", fmt.Errorf("resolve executable for configuration root (use --root to override): %w", err)
	}
	return filepath.Dir(executable), nil
}
