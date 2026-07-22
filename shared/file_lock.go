package shared

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

// FileLockingProcess identifies a process reported by Windows Restart Manager
// as using a file.
type FileLockingProcess struct {
	PID         int
	Name        string
	ServiceName string
	Restartable bool
}

// DisplayName returns the most useful human-readable name for a locking process.
func (p FileLockingProcess) DisplayName() string {
	if p.Name != "" {
		return p.Name
	}
	if p.ServiceName != "" {
		return p.ServiceName
	}
	return "Unknown process"
}

// FileInUseError reports a Windows sharing or lock violation together with the
// processes that Restart Manager identified as using the file.
type FileInUseError struct {
	Path      string
	Processes []FileLockingProcess
	Cause     error
}

func (e *FileInUseError) Error() string {
	if len(e.Processes) == 0 {
		return fmt.Sprintf("file %s is in use: %v", e.Path, e.Cause)
	}

	processes := make([]string, 0, len(e.Processes))
	for _, process := range e.Processes {
		processes = append(processes, fmt.Sprintf("%s (PID %d)", process.DisplayName(), process.PID))
	}
	return fmt.Sprintf("file %s is in use by %s: %v", e.Path, strings.Join(processes, ", "), e.Cause)
}

func (e *FileInUseError) Unwrap() error {
	return e.Cause
}

// WrapFileInUseError adds process information to Windows sharing and lock
// violations. Other errors are returned unchanged.
func WrapFileInUseError(path string, err error) error {
	if err == nil || runtime.GOOS != "windows" || !isWindowsFileLockError(err) {
		return err
	}

	processes, lookupErr := lockingProcesses(path)
	if lookupErr != nil {
		return &FileInUseError{Path: path, Cause: fmt.Errorf("%w (could not identify the owning process: %v)", err, lookupErr)}
	}

	return &FileInUseError{Path: path, Processes: processes, Cause: err}
}

func isWindowsFileLockError(err error) bool {
	return errors.Is(err, syscall.Errno(32)) || errors.Is(err, syscall.Errno(33))
}

// FileName returns the final path component for concise user-facing messages.
func (e *FileInUseError) FileName() string {
	return filepath.Base(e.Path)
}
