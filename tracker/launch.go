package tracker

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"controlpanel/shared"
	customdialog "controlpanel/tracker/dialog"
	"controlpanel/tracker/downloadutils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/gofrs/flock"
	"github.com/shirou/gopsutil/process"
)

func configureTailLogLink(version, appDir string) {
	if tailLogLink == nil {
		return
	}

	logPath := filepath.Join(appDir, "logs", "tracker.log")
	tailLogLink.SetText(fmt.Sprintf("Tail tracker %s logs", version))
	tailLogLink.SetURL(nil)
	tailLogLink.OnTapped = func() {
		if err := shared.TailLogFile(logPath); err != nil {
			log.Printf("Failed to tail tracker logs: %v", err)
			if statusLabel != nil {
				statusLabel.SetText("Failed to tail logs")
			}
		}
	}

	tailLogLink.Show()
}

var (
	lockFilePath = filepath.Join(getInstallDir(), "tracker.lock")
	pidFilePath  = filepath.Join(getInstallDir(), "tracker.pid")
	nodePID      int          // Store the Node process PID
	lock         *flock.Flock // Store the lock
)

func acquireTrackerLock() (*flock.Flock, error) {
	data, err := os.ReadFile(pidFilePath)
	if err == nil && len(data) > 0 {
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err == nil && pid != 0 {
			// Check if process is still running
			proc, procErr := process.NewProcess(int32(pid))
			if procErr == nil {
				running, _ := proc.IsRunning()
				if running {
					log.Printf("Another instance of owlcms-tracker is already running with PID %d", pid)
					return nil, fmt.Errorf("another instance of owlcms-tracker is already running with PID %d", pid)
				}
			}
			// Process is not running, clean up stale PID file
			os.Remove(pidFilePath)
		} else {
			log.Printf("Failed to parse PID from PID file: %v", err)
			os.Remove(pidFilePath)
		}
	}

	return nil, nil
}

func releaseTrackerLock() {
	log.Println("Released Tracker lock")
	if lock != nil {
		lock.Unlock()
		lock = nil
	}
	os.Remove(lockFilePath)
	os.Remove(pidFilePath)
}

func killLockingProcess() error {
	data, err := os.ReadFile(pidFilePath)
	if err != nil {
		return fmt.Errorf("failed to read PID file: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return fmt.Errorf("failed to parse PID from PID file: %w", err)
	}

	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		releaseTrackerLock()
		return fmt.Errorf("failed to find process with PID %d: %w", pid, err)
	}

	if downloadutils.GetGoos() == "windows" && !downloadutils.IsWSL() {
		if err := proc.Terminate(); err != nil {
			releaseTrackerLock()
			return fmt.Errorf("failed to terminate process with PID %d: %w", pid, err)
		}
	} else {
		if err := proc.SendSignal(syscall.SIGKILL); err != nil {
			releaseTrackerLock()
			return fmt.Errorf("failed to kill process with PID %d: %w", pid, err)
		}
	}
	releaseTrackerLock()
	log.Printf("Killed process with PID %d\n", pid)
	return nil
}

func launchTracker(version string, launchButton, stopBtn *widget.Button) error {
	currentVersion = version // Store current version

	// Acquire lock file
	var err error
	lock, err = acquireTrackerLock()
	if err != nil {
		goBackToMainScreen()
		return err
	}

	// Check if port is already in use
	if err := checkPort(); err == nil {
		statusLabel.SetText(fmt.Sprintf("Another program is running on port %s", GetPort()))
		statusLabel.Refresh()
		goBackToMainScreen()
		log.Printf("Another program is running on port %s", GetPort())
		return fmt.Errorf("another program is running on port %s", GetPort())
	}

	statusLabel.SetText(fmt.Sprintf("Starting owlcms-tracker %s...", version))
	statusLabel.Refresh()
	statusLabel.Show()

	// Store current directory to restore it later
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	// Ensure the tracker directory exists
	trackerDir := installDir
	if _, err := os.Stat(trackerDir); os.IsNotExist(err) {
		if err := shared.EnsureDir0755(trackerDir); err != nil {
			return fmt.Errorf("creating tracker directory: %w", err)
		}
	}

	// Get the version directory path
	versionDir := filepath.Join(trackerDir, version)
	if _, err := os.Stat(versionDir); os.IsNotExist(err) {
		launchButton.Show()
		return fmt.Errorf("version directory not found: %s", versionDir)
	}

	// Change to version directory
	if err := os.Chdir(versionDir); err != nil {
		launchButton.Show()
		return fmt.Errorf("changing to version directory: %w", err)
	}
	defer os.Chdir(originalDir)

	// Determine the startup script based on platform
	var cmd *exec.Cmd
	goos := downloadutils.GetGoos()

	if err := InitEnv(); err != nil {
		launchButton.Show()
		goBackToMainScreen()
		return fmt.Errorf("failed to initialize environment: %w", err)
	}

	env := os.Environ()
	newVar := shared.GetLauncherVersionSemver()
	env = append(env, fmt.Sprintf("TRACKER_LAUNCHER=%s", newVar))
	env = append(env, fmt.Sprintf("OWLCMS_CONTROLPANEL=%s", newVar))
	// Map TRACKER_PORT to PORT for the tracker application
	env = append(env, fmt.Sprintf("PORT=%s", GetPort()))

	// Add all properties from environment to the process env
	if environment != nil {
		for _, key := range environment.Keys() {
			value, _ := environment.Get(key)
			log.Printf("   %s=%s", key, value)
			// Skip TRACKER_PORT since we already set PORT above
			if key == "TRACKER_PORT" {
				continue
			}
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	// Check if Node.js is available locally, download if needed
	var nodeExe string

	// Check for NODE_VERSION requirement in env.properties
	var requiredNodeVersion string
	if environment != nil {
		if version, exists := environment.Get("NODE_VERSION"); exists {
			requiredNodeVersion = version
			log.Printf("Found NODE_VERSION requirement: %s\n", requiredNodeVersion)
		}
	}

	nodePath, err := shared.FindLocalNodeForVersion(requiredNodeVersion, shared.GetGoos)
	if err != nil {
		// No suitable Node.js found, download appropriate version
		var targetVersion string
		if requiredNodeVersion != "" {
			log.Printf("No Node.js found meeting requirement %s, downloading...\n", requiredNodeVersion)
			targetVersion = requiredNodeVersion
		} else {
			log.Printf("No Node.js found locally, downloading latest LTS version...")
			var err error
			targetVersion, err = shared.FindLatestNodeRelease("")
			if err != nil {
				launchButton.Show()
				goBackToMainScreen()
				return fmt.Errorf("failed to find latest Node.js release: %w", err)
			}
		}

		statusLabel.SetText("Downloading Node.js...")
		statusLabel.Refresh()

		log.Printf("Target Node.js release: %s\n", targetVersion)

		// Create a cancel channel for the download
		cancel := make(chan bool)

		// Get the active window for the dialog
		var appWindow fyne.Window
		windows := fyne.CurrentApp().Driver().AllWindows()
		if len(windows) > 0 {
			appWindow = windows[0]
		}

		// Show the progress dialog
		var progressDialog dialog.Dialog
		var progressBar *widget.ProgressBar
		if appWindow != nil {
			progressDialog, progressBar = customdialog.NewDownloadDialog(
				"Installing Node.js",
				appWindow,
				cancel)
			progressDialog.Show()
			progressBar.SetValue(0.01) // Set initial activity indicator
		}

		nodePath, err = shared.DownloadAndInstallNode(targetVersion, func(downloaded, total int64) {
			if progressBar != nil && total > 0 {
				percentage := float64(downloaded) / float64(total)
				progressBar.SetValue(percentage)
			}
		})

		if progressDialog != nil {
			progressDialog.Hide()
		}

		if err != nil {
			launchButton.Show()
			goBackToMainScreen()
			return fmt.Errorf("failed to download Node.js: %w", err)
		}

		log.Printf("Successfully installed Node.js at: %s\n", nodePath)
		statusLabel.SetText(fmt.Sprintf("Starting owlcms-tracker %s...", version))
		statusLabel.Refresh()
	}

	nodeExe = nodePath

	if _, err := os.Stat(nodeExe); os.IsNotExist(err) {
		launchButton.Show()
		goBackToMainScreen()
		return fmt.Errorf("node executable not found at %s", nodeExe)
	}

	// Make sure node is executable on Unix systems
	if goos != "windows" {
		os.Chmod(nodeExe, 0755)
	}

	// The main script is start-with-ws.js
	mainScript := filepath.Join(versionDir, "start-with-ws.js")
	if _, err := os.Stat(mainScript); os.IsNotExist(err) {
		launchButton.Show()
		goBackToMainScreen()
		return fmt.Errorf("start-with-ws.js not found in %s", versionDir)
	}

	// On Windows, strip shebang from the JS file if present (causes syntax error in Node.js)
	scriptToRun := mainScript
	if downloadutils.GetGoos() == "windows" {
		content, err := os.ReadFile(mainScript)
		if err == nil && len(content) > 2 && content[0] == '#' && content[1] == '!' {
			// Found shebang - create temp file without it
			lines := strings.Split(string(content), "\n")
			if len(lines) > 1 && strings.HasPrefix(lines[0], "#!") {
				tempScript := filepath.Join(versionDir, "start-with-ws-temp.js")
				cleanContent := strings.Join(lines[1:], "\n")
				if err := os.WriteFile(tempScript, []byte(cleanContent), 0644); err == nil {
					scriptToRun = tempScript
					log.Printf("Created temp script without shebang: %s\n", tempScript)
				}
			}
		}
	}

	cmd = exec.Command(nodeExe, scriptToRun)
	shared.ConfigureNoConsoleWindow(cmd)

	cmd.Env = env
	cmd.Dir = versionDir

	// Capture stdout/stderr to logs/tracker.log (remove previous file first)
	appDir := filepath.Join(installDir, version)
	logPath := filepath.Join(appDir, "logs", "tracker.log")
	_ = os.Remove(logPath)
	if err := shared.EnsureDir0755(filepath.Dir(logPath)); err != nil {
		log.Printf("Failed to create log directory: %v", err)
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Printf("Failed to open tracker log file %s: %v", logPath, err)
		logFile = nil
	} else {
		cmd.Stdout = io.MultiWriter(logFile)
		cmd.Stderr = io.MultiWriter(logFile)
	}

	log.Printf("Starting owlcms-tracker %s\n", version)
	log.Printf("  Working directory: %s\n", cmd.Dir)
	log.Printf("  Command: %s %s\n", nodeExe, mainScript)
	log.Printf("  Full command: %v\n", cmd.Args)

	if err := cmd.Start(); err != nil {
		statusLabel.SetText(fmt.Sprintf("Failed to start owlcms-tracker %s", version))
		releaseTrackerLock()
		launchButton.Show()
		goBackToMainScreen()
		log.Printf("Failed to start owlcms-tracker %s: %v\n", version, err)
		if logFile != nil {
			_ = logFile.Close()
		}
		return fmt.Errorf("failed to start owlcms-tracker %s: %w", version, err)
	}

	// Store the PID in the PID file and globally (after Start() succeeds)
	nodePID = cmd.Process.Pid
	if err := os.WriteFile(pidFilePath, []byte(fmt.Sprintf("%d\n", nodePID)), 0644); err != nil {
		log.Printf("Failed to write PID to PID file: %v\n", err)
	} else {
		log.Printf("Wrote PID %d to PID file %s\n", nodePID, pidFilePath)
	}

	log.Printf("Launching owlcms-tracker %s (PID: %d), waiting for port %s...\n", version, nodePID, GetPort())
	statusLabel.SetText(fmt.Sprintf("Starting owlcms-tracker %s (PID: %d), waiting for port %s.\nFull startup can take up to 15 seconds.", version, nodePID, GetPort()))
	currentProcess = cmd
	stopBtn.SetText(fmt.Sprintf("Stop owlcms-tracker %s", version))
	stopBtn.Show()
	stopContainer.Show()
	setTrackerTabModeRunning()

	appDirLink.SetText(fmt.Sprintf("Open tracker %s directory", version))
	appDirLink.SetURL(nil)
	appDirLink.OnTapped = func() {
		shared.OpenFileExplorer(appDir)
	}
	appDirLink.Show()
	configureTailLogLink(version, appDir)

	// Monitor the process in background
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	monitorChan := monitorProcess(done)

	// Wait for monitoring result in background
	go func() {
		if err := <-monitorChan; err != nil {
			log.Printf("owlcms-tracker process %d failed to start properly: %v\n", nodePID, err)
			statusLabel.SetText(fmt.Sprintf("owlcms-tracker process %d failed to start properly", nodePID))
			stopBtn.Hide()
			stopContainer.Hide()
			launchButton.Show()
			currentProcess = nil
			setTrackerTabMode(mainWindow)
			releaseTrackerLock()
			return
		}

		log.Printf("owlcms-tracker process %d is ready (port %s responding)\n", nodePID, GetPort())
		statusLabel.SetText(fmt.Sprintf("owlcms-tracker running (PID: %d) on port %s", nodePID, GetPort()))
		url := fmt.Sprintf("http://localhost:%s", GetPort())
		urlLink.SetURLFromString(url)
		urlLink.SetText("Open owlcms-tracker in a browser")
		urlLink.Show()

		appDirLink.SetText(fmt.Sprintf("Open tracker %s directory", version))
		appDirLink.SetURL(nil)
		appDirLink.OnTapped = func() {
			shared.OpenFileExplorer(appDir)
		}
		appDirLink.Show()
		configureTailLogLink(version, appDir)
		stopContainer.Refresh()

		// Auto-open the browser when the tracker is ready
		if err := shared.OpenBrowser(url); err != nil {
			log.Printf("Failed to open browser: %v\n", err)
		}

		// Process is stable, wait for it to end
		err := <-done
		pid := cmd.Process.Pid
		if logFile != nil {
			_ = logFile.Close()
		}

		if killedByUs {
			// If we killed it, just report normal termination
			log.Printf("owlcms-tracker %s (PID: %d) was stopped by user\n", version, pid)
			statusLabel.SetText(fmt.Sprintf("owlcms-tracker %s (PID: %d) was stopped by user", version, pid))
		} else if err != nil {
			// Only report error if it wasn't killed by us
			log.Printf("owlcms-tracker %s (PID: %d) terminated with error: %v\n", version, pid, err)
			statusLabel.SetText(fmt.Sprintf("owlcms-tracker %s (PID: %d) terminated with error", version, pid))
		} else {
			log.Printf("owlcms-tracker %s (PID: %d) exited normally\n", version, pid)
			statusLabel.SetText(fmt.Sprintf("owlcms-tracker %s (PID: %d) exited normally", version, pid))
		}

		currentProcess = nil
		killedByUs = false // Reset flag
		stopBtn.Hide()
		stopContainer.Hide()
		launchButton.Show()
		setTrackerTabMode(mainWindow)
		urlLink.Hide()
		if appDirLink != nil {
			appDirLink.Hide()
		}
		if tailLogLink != nil {
			tailLogLink.Hide()
		}
		releaseTrackerLock()
	}()

	return nil
}

func goBackToMainScreen() {
	setTrackerTabMode(fyne.CurrentApp().Driver().AllWindows()[0])
}
