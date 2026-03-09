//go:build !windows && !linux

package shared

import "os/exec"

// ConfigureNoConsoleWindow is a no-op on non-Windows platforms.
func ConfigureNoConsoleWindow(cmd *exec.Cmd) {
	_ = cmd
}

// ConfigureDetachedDaemonProcess is a no-op on non-Linux non-Windows platforms.
func ConfigureDetachedDaemonProcess(cmd *exec.Cmd, detach bool) {
	_ = cmd
	_ = detach
}
