package owlcms

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

func monitorProcess(done <-chan error) chan error {
	result := make(chan error, 1)
	go func() {
		timeout := time.After(60 * time.Second)
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
				if err := checkPort(); err == nil {
					result <- nil
					return
				}
			}
		}
	}()
	return result
}

func stopProcess(process *exec.Cmd, version string, stopBtn *widget.Button, downloadGroup, versionCont *fyne.Container, statusLbl *widget.Label, w fyne.Window) {
	log.Printf("Stopping OWLCMS %s...\n", version)
	statusLbl.SetText(fmt.Sprintf("Stopping OWLCMS %s...", version))

	if process == nil || process.Process == nil {
		return
	}
	pid := process.Process.Pid
	killedByUs = true

	err := GracefullyStopProcess(pid)
	if err != nil {
		killedByUs = false
		dialog.ShowError(fmt.Errorf("failed to stop OWLCMS %s (PID: %d): %w", version, pid, err), w)
		return
	}

	log.Printf("OWLCMS %s (PID: %d) has been stopped\n", version, pid)
	statusLbl.SetText(fmt.Sprintf("OWLCMS %s (PID: %d) has been stopped", version, pid))
	currentProcess = nil

	stopBtn.Hide()
	stopContainer.Hide()

	// Restore UI using centralized mode switching (prevents desync).
	_ = downloadGroup
	_ = versionCont
	setOwlcmsTabMode(w)

	urlLink.Hide()
	appDirLink.Hide()
	if tailLogLink != nil {
		tailLogLink.Hide()
	}
	releaseJavaLock()
}
