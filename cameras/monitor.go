package cameras

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"owlcms-launcher/shared"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func stopCamerasProcess(curProcess *exec.Cmd, curVersion string, stopBtn *widget.Button, w fyne.Window) {
	log.Printf("Stopping cameras %s...\n", curVersion)
	if statusLabel != nil {
		statusLabel.SetText(fmt.Sprintf("Stopping cameras %s...", curVersion))
	}

	if curProcess == nil || curProcess.Process == nil {
		return
	}
	pid := curProcess.Process.Pid
	killedByUs = true

	var err error
	if shared.GetGoos() == "windows" {
		cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid))
		err = cmd.Run()
	} else {
		err = curProcess.Process.Signal(syscall.SIGINT)
	}

	if err != nil {
		log.Printf("Failed to gracefully stop cameras %s (PID: %d): %v\n", curVersion, pid, err)
		err = curProcess.Process.Kill()
		if err != nil {
			killedByUs = false
			dialog.ShowError(fmt.Errorf("failed to stop cameras %s (PID: %d): %w", curVersion, pid, err), w)
			return
		}
	}

	log.Printf("Cameras %s (PID: %d) stopped\n", curVersion, pid)
	if statusLabel != nil {
		statusLabel.SetText(fmt.Sprintf("Cameras %s stopped", curVersion))
	}
	camerasProcess = nil
	os.Remove(camerasPIDFile)
	if stopBtn != nil {
		stopBtn.Hide()
	}
	updateStopContainer()
	checkForNewerVersion()
	downloadContainer.Show()
	versionContainer.Show()
}

func stopReplaysProcess(curProcess *exec.Cmd, curVersion string, stopBtn *widget.Button, w fyne.Window) {
	log.Printf("Stopping replays %s...\n", curVersion)
	if statusLabel != nil {
		statusLabel.SetText(fmt.Sprintf("Stopping replays %s...", curVersion))
	}

	if curProcess == nil || curProcess.Process == nil {
		return
	}
	pid := curProcess.Process.Pid
	killedByUs = true

	var err error
	if shared.GetGoos() == "windows" {
		cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid))
		err = cmd.Run()
	} else {
		err = curProcess.Process.Signal(syscall.SIGINT)
	}

	if err != nil {
		log.Printf("Failed to gracefully stop replays %s (PID: %d): %v\n", curVersion, pid, err)
		err = curProcess.Process.Kill()
		if err != nil {
			killedByUs = false
			dialog.ShowError(fmt.Errorf("failed to stop replays %s (PID: %d): %w", curVersion, pid, err), w)
			return
		}
	}

	log.Printf("Replays %s (PID: %d) stopped\n", curVersion, pid)
	if statusLabel != nil {
		statusLabel.SetText(fmt.Sprintf("Replays %s stopped", curVersion))
	}
	replaysProcess = nil
	os.Remove(replaysPIDFile)
	if stopBtn != nil {
		stopBtn.Hide()
	}
	updateStopContainer()
	checkForNewerVersion()
	downloadContainer.Show()
	versionContainer.Show()
}

func killLockingProcess() error {
	// Try to kill any stale cameras or replays processes by reading PID files
	for _, pidFile := range []string{camerasPIDFile, replaysPIDFile} {
		data, err := os.ReadFile(pidFile)
		if err != nil {
			continue
		}
		pid, err := strconv.Atoi(string(data))
		if err != nil {
			os.Remove(pidFile)
			continue
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			os.Remove(pidFile)
			continue
		}
		if shared.GetGoos() == "windows" {
			cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/F")
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to kill process %d: %w", pid, err)
			}
		} else {
			if err := proc.Signal(syscall.SIGKILL); err != nil {
				return fmt.Errorf("failed to kill process %d: %w", pid, err)
			}
		}
		os.Remove(pidFile)
		log.Printf("Killed stale video process PID %d\n", pid)
	}
	return nil
}

// updateStopContainer refreshes the stop container visibility based on what's running
func updateStopContainer() {
	if stopContainer == nil {
		return
	}
	if camerasProcess == nil && replaysProcess == nil {
		stopContainer.Hide()
	} else {
		stopContainer.Show()
	}
	stopContainer.Refresh()
}
