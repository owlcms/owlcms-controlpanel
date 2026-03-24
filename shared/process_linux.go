//go:build linux

package shared

import (
	"errors"
	"os/exec"
	"syscall"
)

// ShouldRestartProcess inspects the error returned by cmd.Wait() and decides
// whether the process should be restarted.
//
//   - exit 0                              → false  (clean shutdown)
//   - shell-style 128+SIGINT/SIGTERM/SIGKILL → false  (deliberate external stop)
//   - killed by SIGINT/SIGTERM/SIGKILL    → false  (deliberate external stop)
//   - exit non-zero (e.g. 1)              → true   (database import or unexpected failure)
//   - killed by abnormal signal           → true   (JVM native crash: SIGSEGV/SIGABRT in JIT/GC/JNI)
//
// Background: some JVM/runtime combinations report external TERM/INT handling as
// ordinary exit codes 143/130 instead of a signaled WaitStatus.  We treat both
// shapes as intentional user stops so the control panel returns to the launch tab
// instead of relaunching OWLCMS after a deliberate kill.
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
		// SIGTERM, SIGINT, and SIGKILL are treated as deliberate external stops.
		return sig != syscall.SIGTERM && sig != syscall.SIGINT && sig != syscall.SIGKILL
	}
	if code := exitErr.ExitCode(); code == 130 || code == 137 || code == 143 {
		return false
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
