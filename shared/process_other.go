//go:build !windows && !linux

package shared

import (
	"errors"
	"os/exec"
	"syscall"
)

// ShouldRestartProcess inspects the error returned by cmd.Wait() and decides
// whether the process should be restarted.  Same POSIX logic as Linux.
// See process_linux.go for the full rationale.
func ShouldRestartProcess(waitErr error) bool {
	if waitErr == nil {
		return false
	}
	var exitErr *exec.ExitError
	if !errors.As(waitErr, &exitErr) {
		return false
	}
	if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		sig := ws.Signal()
		return sig != syscall.SIGTERM && sig != syscall.SIGINT && sig != syscall.SIGKILL
	}
	if code := exitErr.ExitCode(); code == 130 || code == 137 || code == 143 {
		return false
	}
	return exitErr.ExitCode() > 0
}

// ConfigureNoConsoleWindow is a no-op on non-Windows platforms.
func ConfigureNoConsoleWindow(cmd *exec.Cmd) {
	_ = cmd
}

// ConfigureDetachedDaemonProcess is a no-op on non-Linux non-Windows platforms.
func ConfigureDetachedDaemonProcess(cmd *exec.Cmd, detach bool) {
	_ = cmd
	_ = detach
}
