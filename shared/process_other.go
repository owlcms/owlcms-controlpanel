//go:build !windows

package shared

import "os/exec"

// ConfigureNoConsoleWindow is a no-op on non-Windows platforms.
func ConfigureNoConsoleWindow(cmd *exec.Cmd) {
	_ = cmd
}
