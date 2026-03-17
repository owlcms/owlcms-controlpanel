//go:build windows

package shared

import (
	"errors"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// ConfigureNoConsoleWindow applies Windows-specific process attributes so starting a
// console-subsystem executable (like node.exe) won't flash/show a console window.
func ConfigureNoConsoleWindow(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW
}

// ConfigureDetachedDaemonProcess is a no-op on Windows.
func ConfigureDetachedDaemonProcess(cmd *exec.Cmd, detach bool) {
	_ = cmd
	_ = detach
}

// ShouldRestartProcess decides whether a process should be restarted based on
// its exit error.  On Windows, signal deaths do not apply the same way as on
// Unix — ExitCode() always returns the actual exit code, never -1.
// Non-zero exit → restart; nil error (exit 0) → don't restart.
func ShouldRestartProcess(waitErr error) bool {
	if waitErr == nil {
		return false
	}
	var exitErr *exec.ExitError
	if !errors.As(waitErr, &exitErr) {
		return false
	}
	return exitErr.ExitCode() > 0
}
