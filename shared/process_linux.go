//go:build linux

package shared

import (
	"os/exec"
	"syscall"
)

// ConfigureNoConsoleWindow is a no-op on Linux.
func ConfigureNoConsoleWindow(cmd *exec.Cmd) {
	_ = cmd
}

// ConfigureDetachedDaemonProcess detaches a process from the controlling terminal
// so it can survive terminal closure when daemon mode is enabled.
func ConfigureDetachedDaemonProcess(cmd *exec.Cmd, detach bool) {
	if cmd == nil || !detach {
		return
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	// Setsid creates a new session (and process group), detaching from the
	// controlling terminal so the child survives terminal closure.
	cmd.SysProcAttr.Setsid = true
}
