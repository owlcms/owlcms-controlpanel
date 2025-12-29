package shared

import (
	"os"
	"runtime"
)

const (
	DefaultDirMode os.FileMode = 0755
)

// EnsureDir0755 ensures the given directory exists and has 0755 permissions.
// On Windows, permissions are effectively a no-op.
func EnsureDir0755(path string) error {
	if err := os.MkdirAll(path, DefaultDirMode); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	// Chmod even if it already exists, so updates/imports can fix older installs.
	return os.Chmod(path, DefaultDirMode)
}
