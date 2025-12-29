//go:build windows

package shared

import (
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
