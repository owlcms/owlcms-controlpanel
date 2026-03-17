//go:build linux

package shared

import (
	"errors"
	"os/exec"
	"syscall"
)

// ShouldRestartProcess inspects the error returned by cmd.Wait() and decides
// whether the process should be restarted.  The rules match Docker/systemd
// Restart=on-failure + SuccessExitStatus=SIGTERM:
//
//   - exit 0                    → false  (clean shutdown)
//   - exit non-zero (e.g. 1)    → true   (database import triggers restart)
//   - killed by SIGTERM/SIGINT  → false  (intentional stop by control panel or systemctl stop)
//   - killed by abnormal signal → true   (JVM native crash: SIGSEGV/SIGABRT in JIT/GC/JNI)
//
// Background: Go's ExitCode() returns -1 for signal deaths (not the shell 128+N
// convention), so the old "> 0 && < 128" check silently skipped all signal deaths.
func ShouldRestartProcess(waitErr error) bool {
	if waitErr == nil {
		return false
	}
	var exitErr *exec.ExitError
	if !errors.As(waitErr, &exitErr) {
		return false
	}
	// Signal death: ExitCode() is -1; inspect the signal directly.
	if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		sig := ws.Signal()
		// SIGTERM and SIGINT are intentional stops — do not restart.
		return sig != syscall.SIGTERM && sig != syscall.SIGINT
	}
	// Voluntary non-zero exit (e.g. exit 1 from database import) → restart.
	return exitErr.ExitCode() > 0
}

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
