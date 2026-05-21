package replays

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"controlpanel/shared"

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

	if err := shared.StopOwnedProcess(curProcess, 10*time.Second); err != nil {
		killedByUs = false
		dialog.ShowError(fmt.Errorf("failed to stop cameras %s (PID: %d): %w", curVersion, pid, err), w)
		return
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
	hideAllRunLinks()
}

func stopReplaysProcess(curProcess *exec.Cmd, curVersion string, stopBtn *widget.Button, w fyne.Window) {
	log.Printf("Stopping replays %s...\n", curVersion)
	if statusLabel != nil {
		statusLabel.SetText(fmt.Sprintf("Stopping replays %s...", curVersion))
	}
	port := getPortForRelease(curVersion)
	if port == "" {
		port = runtimeReplaysPort()
	}
	killedByUs = true

	var err error
	if curProcess != nil && curProcess.Process != nil {
		pid := curProcess.Process.Pid
		log.Printf("Stopping owned replays process with Go process handle (PID: %d)", pid)
		err = shared.StopOwnedProcess(curProcess, 10*time.Second)
	} else {
		err = shared.StopPIDFileOrPortProcess(replaysPIDFile, port)
	}
	if err != nil {
		killedByUs = false
		dialog.ShowError(fmt.Errorf("failed to stop replays %s: %w", curVersion, err), w)
		return
	}

	log.Printf("Replays %s stopped\n", curVersion)
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
	hideAllRunLinks()
}

func killLockingProcess() error {
	if err := shared.StopPIDFileOrPortProcess(camerasPIDFile, ""); err != nil {
		return err
	}
	os.Remove(camerasPIDFile)

	if err := shared.StopPIDFileOrPortProcess(replaysPIDFile, runtimeReplaysPort()); err != nil {
		return err
	}
	os.Remove(replaysPIDFile)
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
