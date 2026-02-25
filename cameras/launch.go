package cameras

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"controlpanel/shared"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

var (
	camerasPIDFile = filepath.Join(getInstallDir(), "cameras.pid")
	replaysPIDFile = filepath.Join(getInstallDir(), "replays.pid")
)

func replaysInstallDir() string {
	switch shared.GetGoos() {
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "owlcms-replays")
	case "darwin":
		return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "owlcms-replays")
	case "linux":
		return filepath.Join(os.Getenv("HOME"), ".local", "share", "owlcms-replays")
	default:
		return "./owlcms-replays"
	}
}

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

func launchCameras(version string, _ *widget.Button, w fyne.Window) error {
	versionDir := filepath.Join(installDir, version)
	configDir := versionDir
	exePath := filepath.Join(versionDir, camerasExeName())

	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		return fmt.Errorf("cameras binary not found: %s", exePath)
	}

	// Ensure FFmpeg is available (download if needed)
	if _, err := shared.EnsureFFmpegPrerequisite(w); err != nil {
		return fmt.Errorf("FFmpeg prerequisite: %w", err)
	}

	// Make executable on Linux
	if shared.GetGoos() != "windows" {
		if err := os.Chmod(exePath, 0755); err != nil {
			log.Printf("Warning: could not chmod cameras binary: %v", err)
		}
	}

	if shared.ShouldRunVideoExtract(versionDir, "cameras") {
		log.Printf("Running cameras extract bootstrap for %s", versionDir)
		if err := shared.RunVideoExtractBootstrap(exePath, versionDir); err != nil {
			return err
		}
	}

	cmd := exec.Command(exePath, "--configDir", configDir)
	cmd.Dir = versionDir

	logPath := filepath.Join(versionDir, "logs", "cameras.log")
	if err := shared.ResetLogFile(logPath); err != nil {
		return fmt.Errorf("failed to reset cameras log: %w", err)
	}

	cmd.Env = shared.BuildVideoLaunchEnv(versionDir)

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
	setVideoTabModeRunning()

	configureCamerasRunLinks(version, versionDir)

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
			setVideoTabMode(mainWindow)
			hideAllRunLinks()
			checkForNewerVersion()
		} else {
			if statusLabel != nil {
				statusLabel.SetText(fmt.Sprintf("Replays %s running (PID: %d)", replaysVersion, replaysProcess.Process.Pid))
			}
		}
	}()

	return nil
}

func launchReplays(version string, _ *widget.Button, w fyne.Window) error {
	versionDir := filepath.Join(replaysInstallDir(), version)
	configDir := versionDir
	exePath := filepath.Join(versionDir, replaysExeName())

	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		return fmt.Errorf("replays binary not found: %s", exePath)
	}

	// Ensure FFmpeg is available (download if needed)
	if _, err := shared.EnsureFFmpegPrerequisite(w); err != nil {
		return fmt.Errorf("FFmpeg prerequisite: %w", err)
	}

	// Make executable on Linux
	if shared.GetGoos() != "windows" {
		if err := os.Chmod(exePath, 0755); err != nil {
			log.Printf("Warning: could not chmod replays binary: %v", err)
		}
	}

	if shared.ShouldRunVideoExtract(versionDir, "replays") {
		log.Printf("Running replays extract bootstrap for %s", versionDir)
		if err := shared.RunVideoExtractBootstrap(exePath, versionDir); err != nil {
			return err
		}
	}

	cmd := exec.Command(exePath, "--configDir", configDir)
	cmd.Dir = versionDir

	logPath := filepath.Join(versionDir, "logs", "replays.log")
	if err := shared.ResetLogFile(logPath); err != nil {
		return fmt.Errorf("failed to reset replays log: %w", err)
	}

	cmd.Env = shared.BuildVideoLaunchEnv(versionDir)

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
	setVideoTabModeRunning()

	configureReplaysRunLinks(version, versionDir)

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
			setVideoTabMode(mainWindow)
			hideAllRunLinks()
			checkForNewerVersion()
		} else {
			if statusLabel != nil {
				statusLabel.SetText(fmt.Sprintf("Cameras %s running (PID: %d)", camerasVersion, camerasProcess.Process.Pid))
			}
		}
	}()

	return nil
}

func configureCamerasRunLinks(version, versionDir string) {
	if camerasDirLink != nil {
		camerasDirLink.SetText(fmt.Sprintf("Open Cameras %s configuration directory", version))
		camerasDirLink.SetURL(nil)
		camerasDirLink.OnTapped = func() {
			if err := shared.OpenFileExplorer(versionDir); err != nil && statusLabel != nil {
				statusLabel.SetText(fmt.Sprintf("Failed to open Cameras directory: %v", err))
			}
		}
		camerasDirLink.Show()
	}

	if camerasLogLink != nil {
		logPath := filepath.Join(versionDir, "logs", "cameras.log")
		camerasLogLink.SetText(fmt.Sprintf("Tail cameras %s logs", version))
		camerasLogLink.SetURL(nil)
		camerasLogLink.OnTapped = func() {
			if err := shared.TailLogFile(logPath); err != nil && statusLabel != nil {
				statusLabel.SetText(fmt.Sprintf("Failed to tail cameras logs: %v", err))
			}
		}
		camerasLogLink.Show()
	}
}

func configureReplaysRunLinks(version, versionDir string) {
	if replaysDirLink != nil {
		replaysDirLink.SetText(fmt.Sprintf("Open Replays %s configuration directory", version))
		replaysDirLink.SetURL(nil)
		replaysDirLink.OnTapped = func() {
			if err := shared.OpenFileExplorer(versionDir); err != nil && statusLabel != nil {
				statusLabel.SetText(fmt.Sprintf("Failed to open Replays directory: %v", err))
			}
		}
		replaysDirLink.Show()
	}

	if replaysLogLink != nil {
		logPath := filepath.Join(versionDir, "logs", "replays.log")
		replaysLogLink.SetText(fmt.Sprintf("Tail replays %s logs", version))
		replaysLogLink.SetURL(nil)
		replaysLogLink.OnTapped = func() {
			if err := shared.TailLogFile(logPath); err != nil && statusLabel != nil {
				statusLabel.SetText(fmt.Sprintf("Failed to tail replays logs: %v", err))
			}
		}
		replaysLogLink.Show()
	}
}

func hideAllRunLinks() {
	if appDirLink != nil {
		appDirLink.Hide()
	}
	if camerasDirLink != nil {
		camerasDirLink.Hide()
	}
	if replaysDirLink != nil {
		replaysDirLink.Hide()
	}
	if camerasLogLink != nil {
		camerasLogLink.Hide()
	}
	if replaysLogLink != nil {
		replaysLogLink.Hide()
	}
}
