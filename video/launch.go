package video

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"owlcms-launcher/shared"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

var (
	camerasPIDFile = filepath.Join(getInstallDir(), "cameras.pid")
	replaysPIDFile = filepath.Join(getInstallDir(), "replays.pid")
)

// camerasExeName returns the platform-specific binary name for cameras
func camerasExeName() string {
	switch shared.GetGoos() {
	case "windows":
		return "cameras_windows.exe"
	case "linux":
		if shared.GetGoarch() == "arm64" {
			return "cameras_linux_arm64"
		}
		return "cameras_linux_amd64"
	default:
		return "cameras_linux_amd64"
	}
}

// replaysExeName returns the platform-specific binary name for replays
func replaysExeName() string {
	switch shared.GetGoos() {
	case "windows":
		return "replays_windows.exe"
	case "linux":
		if shared.GetGoarch() == "arm64" {
			return "replays_linux_arm64"
		}
		return "replays_linux_amd64"
	default:
		return "replays_linux_amd64"
	}
}

func launchCameras(version string, _ *widget.Button, _ fyne.Window) error {
	versionDir := filepath.Join(installDir, version)
	exePath := filepath.Join(versionDir, camerasExeName())

	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		return fmt.Errorf("cameras binary not found: %s", exePath)
	}

	// Make executable on Linux
	if shared.GetGoos() != "windows" {
		if err := os.Chmod(exePath, 0755); err != nil {
			log.Printf("Warning: could not chmod cameras binary: %v", err)
		}
	}

	cmd := exec.Command(exePath)
	cmd.Dir = versionDir

	env := os.Environ()
	env = append(env, fmt.Sprintf("VIDEO_CONFIGDIR=%s", versionDir))
	env = append(env, fmt.Sprintf("VIDEO_LAUNCHER=%s", shared.GetLauncherVersionSemver()))
	env = append(env, fmt.Sprintf("OWLCMS_CONTROLPANEL=%s", shared.GetLauncherVersionSemver()))
	cmd.Env = env

	log.Printf("Starting cameras %s: %s", version, exePath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start cameras %s: %w", version, err)
	}

	camerasProcess = cmd
	camerasVersion = version

	pid := cmd.Process.Pid
	if err := os.WriteFile(camerasPIDFile, []byte(strconv.Itoa(pid)), 0644); err != nil {
		log.Printf("Failed to write cameras PID file: %v", err)
	}

	if statusLabel != nil {
		statusLabel.SetText(fmt.Sprintf("Cameras %s running (PID: %d)", version, pid))
	}
	cameraStopButton.SetText(fmt.Sprintf("Stop Cameras %s", version))
	cameraStopButton.Show()
	updateStopContainer()
	downloadContainer.Hide()
	versionContainer.Hide()

	if appDirLink != nil {
		appDirLink.SetText(fmt.Sprintf("Open video %s directory", version))
		appDirLink.SetURL(nil)
		appDirLink.OnTapped = func() { shared.OpenFileExplorer(versionDir) }
		appDirLink.Show()
	}

	go func() {
		err := cmd.Wait()
		pid := cmd.Process.Pid

		if killedByUs {
			log.Printf("Cameras %s (PID: %d) stopped by user\n", version, pid)
		} else if err != nil {
			log.Printf("Cameras %s (PID: %d) exited with error: %v\n", version, pid, err)
			// Show error on UI thread
			if fyne.CurrentApp() != nil {
				windows := fyne.CurrentApp().Driver().AllWindows()
				if len(windows) > 0 {
					fyne.CurrentApp().Driver().AllWindows()[0].Canvas().Refresh(statusLabel)
				}
			}
		} else {
			log.Printf("Cameras %s (PID: %d) exited normally\n", version, pid)
		}

		camerasProcess = nil
		killedByUs = false
		os.Remove(camerasPIDFile)
		cameraStopButton.Hide()
		updateStopContainer()

		if replaysProcess == nil {
			// Both stopped — restore the full UI
			if statusLabel != nil {
				statusLabel.SetText("")
			}
			downloadContainer.Show()
			versionContainer.Show()
			if appDirLink != nil {
				appDirLink.Hide()
			}
			checkForNewerVersion()
		} else {
			if statusLabel != nil {
				statusLabel.SetText(fmt.Sprintf("Replays %s running (PID: %d)", replaysVersion, replaysProcess.Process.Pid))
			}
		}
	}()

	return nil
}

func launchReplays(version string, _ *widget.Button, _ fyne.Window) error {
	versionDir := filepath.Join(installDir, version)
	exePath := filepath.Join(versionDir, replaysExeName())

	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		return fmt.Errorf("replays binary not found: %s", exePath)
	}

	// Make executable on Linux
	if shared.GetGoos() != "windows" {
		if err := os.Chmod(exePath, 0755); err != nil {
			log.Printf("Warning: could not chmod replays binary: %v", err)
		}
	}

	cmd := exec.Command(exePath)
	cmd.Dir = versionDir

	env := os.Environ()
	env = append(env, fmt.Sprintf("VIDEO_CONFIGDIR=%s", versionDir))
	env = append(env, fmt.Sprintf("VIDEO_LAUNCHER=%s", shared.GetLauncherVersionSemver()))
	env = append(env, fmt.Sprintf("OWLCMS_CONTROLPANEL=%s", shared.GetLauncherVersionSemver()))
	cmd.Env = env

	log.Printf("Starting replays %s: %s", version, exePath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start replays %s: %w", version, err)
	}

	replaysProcess = cmd
	replaysVersion = version

	pid := cmd.Process.Pid
	if err := os.WriteFile(replaysPIDFile, []byte(strconv.Itoa(pid)), 0644); err != nil {
		log.Printf("Failed to write replays PID file: %v", err)
	}

	if statusLabel != nil {
		statusLabel.SetText(fmt.Sprintf("Replays %s running (PID: %d)", version, pid))
	}
	replaysStopButton.SetText(fmt.Sprintf("Stop Replays %s", version))
	replaysStopButton.Show()
	updateStopContainer()
	downloadContainer.Hide()
	versionContainer.Hide()

	if appDirLink != nil {
		appDirLink.SetText(fmt.Sprintf("Open video %s directory", version))
		appDirLink.SetURL(nil)
		appDirLink.OnTapped = func() { shared.OpenFileExplorer(versionDir) }
		appDirLink.Show()
	}

	go func() {
		err := cmd.Wait()
		pid := cmd.Process.Pid

		if killedByUs {
			log.Printf("Replays %s (PID: %d) stopped by user\n", version, pid)
		} else if err != nil {
			log.Printf("Replays %s (PID: %d) exited with error: %v\n", version, pid, err)
		} else {
			log.Printf("Replays %s (PID: %d) exited normally\n", version, pid)
		}

		replaysProcess = nil
		killedByUs = false
		os.Remove(replaysPIDFile)
		replaysStopButton.Hide()
		updateStopContainer()

		if camerasProcess == nil {
			// Both stopped — restore the full UI
			if statusLabel != nil {
				statusLabel.SetText("")
			}
			downloadContainer.Show()
			versionContainer.Show()
			if appDirLink != nil {
				appDirLink.Hide()
			}
			checkForNewerVersion()
		} else {
			if statusLabel != nil {
				statusLabel.SetText(fmt.Sprintf("Cameras %s running (PID: %d)", camerasVersion, camerasProcess.Process.Pid))
			}
		}
	}()

	return nil
}
