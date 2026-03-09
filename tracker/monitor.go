package tracker

import (
	"fmt"
	"log"
	"os/exec"
	"time"

	"controlpanel/shared"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func monitorProcess(done <-chan error, port string) chan error {
	result := make(chan error, 1)
	go func() {
		// Try connecting to the port for up to 30 seconds (Node.js starts faster than Java)
		timeout := time.After(30 * time.Second)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case err := <-done:
				if err != nil {
					result <- fmt.Errorf("process failed: %w", err)
				} else {
					result <- fmt.Errorf("process exited before becoming ready")
				}
				return
			case <-timeout:
				result <- fmt.Errorf("timed out waiting for process to become ready")
				return
			case <-ticker.C:
				if shared.CheckPort(port) == nil {
					// Port is responding, process is ready
					result <- nil
					return
				}
			}
		}
	}()
	return result
}

func stopProcess(proc *exec.Cmd, version string, stopBtn *widget.Button, downloadGroup, versionCont *fyne.Container, statusLbl *widget.Label, w fyne.Window) {
	log.Printf("Stopping owlcms-tracker %s...\n", version)
	statusLbl.SetText(fmt.Sprintf("Stopping owlcms-tracker %s...", version))

	var pid int
	if proc != nil && proc.Process != nil {
		pid = proc.Process.Pid
	} else if activeRuntime != nil {
		pid = activeRuntime.PID
		if !shared.PIDMatchesStartTicks(pid, activeRuntime.ProcessStartTicks) {
			clearRuntimeState()
			releaseTrackerLock()
			dialog.ShowError(fmt.Errorf("tracker PID %d no longer matches the saved runtime metadata", pid), w)
			return
		}
	} else {
		return
	}
	killedByUs = true

	err := shared.GracefullyStopPID(pid)
	if err != nil {
		killedByUs = false
		dialog.ShowError(fmt.Errorf("failed to stop owlcms-tracker %s (PID: %d): %w", version, pid, err), w)
		return
	}

	log.Printf("owlcms-tracker %s (PID: %d) has been stopped\n", version, pid)
	statusLbl.SetText(fmt.Sprintf("owlcms-tracker %s (PID: %d) has been stopped", version, pid))
	currentProcess = nil
	clearRuntimeState()
	stopBtn.Hide()
	urlLink.Hide() // Hide the URL when stopping
	if appDirLink != nil {
		appDirLink.Hide()
	}
	if tailLogLink != nil {
		tailLogLink.Hide()
	}
	checkForNewerVersion()
	downloadGroup.Show()
	versionCont.Show()
	releaseTrackerLock()
}
