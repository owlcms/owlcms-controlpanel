package tracker

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strconv"
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

func monitorProcess(done <-chan error) chan error {
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
		// On Windows, use taskkill for graceful termination like OWLCMS does
		log.Printf("Using taskkill for graceful termination of owlcms-tracker %s (PID: %d)\n", version, pid)
		cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid))
		err = cmd.Run()

		if err != nil {
			log.Printf("Graceful termination failed for owlcms-tracker %s (PID: %d): %v\n", version, pid, err)
		} else {
			// Give it a moment to shut down gracefully
			time.Sleep(500 * time.Millisecond)
			log.Printf("owlcms-tracker %s (PID: %d) gracefully terminated\n", version, pid)
			statusLbl.SetText(fmt.Sprintf("owlcms-tracker %s (PID: %d) gracefully terminated", version, pid))
			currentProcess = nil
			killedByUs = true
			stopBtn.Hide()
			urlLink.Hide()
			if appDirLink != nil {
				appDirLink.Hide()
			}
			if tailLogLink != nil {
				tailLogLink.Hide()
			}
			return
		}
	} else {
		err = proc.Process.Signal(syscall.SIGINT)
	}

	if err != nil {
		log.Printf("Failed to send interrupt signal to owlcms-tracker %s (PID: %d): %v\n", version, pid, err)
		log.Printf("Attempting forceful termination of owlcms-tracker %s (PID: %d)\n", version, pid)
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
