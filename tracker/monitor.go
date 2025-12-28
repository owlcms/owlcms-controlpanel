package tracker

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"

	"owlcms-launcher/tracker/downloadutils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

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

		// Try connecting to the port for up to 30 seconds (Node.js starts faster than Java)
		timeout := time.After(30 * time.Second)
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

func stopProcess(proc *exec.Cmd, version string, stopBtn *widget.Button, downloadGroup, versionCont *fyne.Container, statusLbl *widget.Label, w fyne.Window) {
	log.Printf("Stopping owlcms-tracker %s...\n", version)
	statusLbl.SetText(fmt.Sprintf("Stopping owlcms-tracker %s...", version))

	if proc == nil || proc.Process == nil {
		return
	}
	pid := proc.Process.Pid
	killedByUs = true

	var err error
	if downloadutils.GetGoos() == "windows" {
		err = proc.Process.Signal(os.Interrupt)
	} else {
		err = proc.Process.Signal(syscall.SIGINT)
	}

	if err != nil {
		log.Printf("Failed to send interrupt signal to owlcms-tracker %s (PID: %d): %v\n", version, pid, err)
		err = proc.Process.Kill()
		if err != nil {
			killedByUs = false
			dialog.ShowError(fmt.Errorf("failed to stop owlcms-tracker %s (PID: %d): %w", version, pid, err), w)
			return
		}
	}

	log.Printf("owlcms-tracker %s (PID: %d) has been stopped\n", version, pid)
	statusLbl.SetText(fmt.Sprintf("owlcms-tracker %s (PID: %d) has been stopped", version, pid))
	currentProcess = nil
	stopBtn.Hide()
	urlLink.Hide() // Hide the URL when stopping
	checkForNewerVersion()
	downloadGroup.Show()
	versionCont.Show()
	releaseTrackerLock()
}
