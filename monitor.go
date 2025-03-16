package main

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

var killedByUs bool

// checkPort tries to connect to localhost:port and returns nil if successful
func checkPort() error {
	resp, err := http.Get(fmt.Sprintf("http://localhost:%s", GetPort()))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func monitorProcess(cmd *exec.Cmd) chan error {
	result := make(chan error, 1)
	go func() {
		// Start a goroutine to wait for process exit
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		// Try connecting to port 8080 for up to 60 seconds
		timeout := time.After(60 * time.Second)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case err := <-done:
				// Process exited before port was available
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
				if err := checkPort(); err == nil {
					// Port is responding, process is ready
					result <- nil
					return
				}
			}
		}
	}()
	return result
}

func stopProcess(currentProcess *exec.Cmd, currentVersion string, stopButton *widget.Button, downloadGroup, versionContainer *fyne.Container, statusLabel *widget.Label, w fyne.Window) {
	log.Printf("Stopping OWLCMS %s...\n", currentVersion)
	statusLabel.SetText(fmt.Sprintf("Stopping OWLCMS %s...", currentVersion))

	if currentProcess == nil || currentProcess.Process == nil {
		return
	}
	pid := currentProcess.Process.Pid
	killedByUs = true

	err := GracefullyStopProcess(pid)
	if err != nil {
		killedByUs = false
		dialog.ShowError(fmt.Errorf("failed to stop OWLCMS %s (PID: %d): %w", currentVersion, pid, err), w)
		return
	}

	log.Printf("OWLCMS %s (PID: %d) has been stopped\n", currentVersion, pid)
	statusLabel.SetText(fmt.Sprintf("OWLCMS %s (PID: %d) has been stopped", currentVersion, pid))
	currentProcess = nil
	stopButton.Hide()
	urlLink.Hide() // Hide the URL when stopping
	checkForNewerVersion()
	downloadGroup.Show()
	versionContainer.Show()
	releaseJavaLock()
}
