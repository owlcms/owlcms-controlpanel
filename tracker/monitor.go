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

	port := GetPort()
	if activeRuntime != nil && activeRuntime.Port != "" {
		port = activeRuntime.Port
	}

	killedByUs = true

	err := shared.EnsurePortFree(port)
	if err != nil {
		killedByUs = false
		dialog.ShowError(fmt.Errorf("failed to stop owlcms-tracker %s on port %s: %w", version, port, err), w)
		return
	}

	log.Printf("owlcms-tracker %s has been stopped (port %s freed)\n", version, port)
	statusLbl.SetText(fmt.Sprintf("owlcms-tracker %s has been stopped", version))
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
