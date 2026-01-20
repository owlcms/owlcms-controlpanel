package owlcms

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"owlcms-launcher/owlcms/javacheck"
	"owlcms-launcher/shared"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/Masterminds/semver/v3"
	"github.com/gofrs/flock"
)

const tailviewerPath = `C:\Program Files\Tailviewer\Tailviewer.exe`

// TEMP TEST FLAG: when true, pretend Tailviewer is not installed so we can
// exercise the PowerShell tail fallback.
var forceNoTailviewer = false

func bashSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func configureTailLogLink(version, appDir string) {
	if tailLogLink == nil {
		return
	}

	goos := shared.GetGoos()
	if goos != "windows" && goos != "darwin" && goos != "linux" {
		tailLogLink.Hide()
		return
	}

	logPath := filepath.Join(appDir, "logs", "owlcms.log")
	tailLogLink.SetText(fmt.Sprintf("Tail owlcms %s logs", version))
	tailLogLink.SetURL(nil)
	tailLogLink.OnTapped = func() {
		switch goos {
		case "windows":
			if !forceNoTailviewer {
				if _, err := os.Stat(tailviewerPath); err == nil {
					if err := exec.Command(tailviewerPath, logPath).Start(); err != nil {
						log.Printf("Failed to start Tailviewer: %v", err)
						if statusLabel != nil {
							statusLabel.SetText("Failed to start Tailviewer")
						}
					}
					return
				}
			}
			// Fallback: open a new PowerShell window tailing the log
			escaped := strings.ReplaceAll(logPath, "'", "''")
			psCmd := fmt.Sprintf("Get-Content -Path '%s' -Wait -Tail 10", escaped)
			if err := exec.Command("cmd", "/c", "start", "powershell", "-NoExit", "-Command", psCmd).Start(); err != nil {
				log.Printf("Failed to start PowerShell tail: %v", err)
				if statusLabel != nil {
					statusLabel.SetText("Failed to open PowerShell tail")
				}
			}
		case "darwin":
			// Open Terminal and run tail
			script := fmt.Sprintf(`tell application "Terminal" to do script "tail -n 10 -f %s"`, bashSingleQuote(logPath))
			if err := exec.Command("osascript", "-e", script, "-e", `tell application "Terminal" to activate`).Start(); err != nil {
				log.Printf("Failed to start Terminal tail: %v", err)
				if statusLabel != nil {
					statusLabel.SetText("Failed to open Terminal tail")
				}
			}
		case "linux":
			// Try common terminals in priority order
			cmd := fmt.Sprintf("tail -n 10 -f %s", bashSingleQuote(logPath))
			try := func(name string, args ...string) bool {
				if _, err := exec.LookPath(name); err != nil {
					return false
				}
				if err := exec.Command(name, args...).Start(); err != nil {
					log.Printf("Failed to start %s for tail: %v", name, err)
					return false
				}
				return true
			}
			if try("x-terminal-emulator", "-e", "bash", "-lc", cmd) {
				return
			}
			if try("gnome-terminal", "--", "bash", "-lc", cmd) {
				return
			}
			if try("konsole", "-e", "bash", "-lc", cmd) {
				return
			}
			if try("xfce4-terminal", "-e", "bash", "-lc", cmd) {
				return
			}
			if try("xterm", "-e", "bash", "-lc", cmd) {
				return
			}
			log.Printf("No terminal emulator found to tail logs")
			if statusLabel != nil {
				statusLabel.SetText("No terminal emulator found to tail logs")
			}
		}
	}

	tailLogLink.Show()
}

var (
	lockFilePath   = filepath.Join(installDir, "java.lock")
	pidFilePath    = filepath.Join(installDir, "java.pid")
	javaPID        int
	lock           *flock.Flock
	currentProcess *exec.Cmd
	currentVersion string
)

var (
	startupLogMu       sync.Mutex
	startupLogStopCh   chan struct{}
	startupLogUpdating bool
	startupLogLastText string
)

func acquireJavaLock() (*flock.Flock, error) {
	data, err := os.ReadFile(pidFilePath)
	if err == nil && len(data) > 0 {
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err == nil && pid != 0 {
			if IsProcessRunning(pid) {
				log.Printf("Another instance of OWLCMS is already running with PID %d", pid)
				return nil, fmt.Errorf("another instance of OWLCMS is already running with PID %d", pid)
			} else {
				log.Printf("Stale PID file found (PID %d is not running), removing and proceeding", pid)
				os.Remove(pidFilePath)
				os.Remove(lockFilePath)
			}
		} else {
			log.Printf("Failed to parse PID from PID file: %v", err)
			os.Remove(pidFilePath)
		}
	} else {
		return nil, nil
	}

	return nil, nil
}

func releaseJavaLock() {
	log.Println("Released Java lock")
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

	err = GracefullyStopProcess(pid)
	if err != nil {
		releaseJavaLock()
		return err
	}

	releaseJavaLock()
	log.Printf("Killed process with PID %d\n", pid)
	return nil
}

func launchOwlcms(version string, launchButton, stopBtn *widget.Button) error {
	currentVersion = version

	var err error
	lock, err = acquireJavaLock()
	if err != nil {
		goBackToMainScreen()
		return err
	}

	if err := checkPort(); err == nil {
		statusLabel.SetText(fmt.Sprintf("Another program is running on port %s", GetPort()))
		statusLabel.Refresh()
		goBackToMainScreen()
		log.Printf("Another program is running on port %s", GetPort())
		return fmt.Errorf("another program is running on port %s", GetPort())
	}

	statusLabel.SetText(fmt.Sprintf("Starting OWLCMS %s...", version))
	statusLabel.Refresh()
	statusLabel.Show()

	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	owlcmsDir := installDir
	if _, err := os.Stat(owlcmsDir); os.IsNotExist(err) {
		if err := shared.EnsureDir0755(owlcmsDir); err != nil {
			return fmt.Errorf("creating owlcms directory: %w", err)
		}
	}

	jarPath := filepath.Join(owlcmsDir, version, "owlcms.jar")
	if _, err := os.Stat(jarPath); os.IsNotExist(err) {
		launchButton.Show()
		return fmt.Errorf("owlcms.jar not found in %s directory", jarPath)
	}

	if err := os.Chdir(filepath.Join(owlcmsDir, version)); err != nil {
		launchButton.Show()
		return fmt.Errorf("changing to version directory: %w", err)
	}
	defer os.Chdir(originalDir)

	// Get version-specific Temurin version
	temurinVersion := GetTemurinVersionForRelease(version)
	localJava, err := javacheck.FindLocalJavaForVersion(temurinVersion)
	if err != nil {
		statusLabel.SetText(fmt.Sprintf("Failed to find local Java: %v", err))
		launchButton.Show()
		goBackToMainScreen()
		return fmt.Errorf("failed to find local Java: %w", err)
	}

	if err := InitEnv(); err != nil {
		statusLabel.SetText(fmt.Sprintf("Failed to initialize environment: %v", err))
		launchButton.Show()
		goBackToMainScreen()
		releaseJavaLock()
		return fmt.Errorf("failed to initialize environment: %w", err)
	}

	env := os.Environ()
	newVar := shared.GetLauncherVersionSemver()
	env = append(env, fmt.Sprintf("OWLCMS_LAUNCHER=%s", newVar))
	env = append(env, fmt.Sprintf("OWLCMS_CONTROLPANEL=%s", newVar))

	for _, key := range environment.Keys() {
		value, _ := environment.Get(key)
		log.Printf("   %s=%s", key, value)
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	cmd := exec.Command(localJava, "-jar", "owlcms.jar")
	cmd.Env = env

	// Remove startup.log if it exists to ensure fresh log output
	// (Only versions >= 64.0.0-rc08 generate this file.)
	if owlcmsSupportsStartupLog(version) {
		versionDir := filepath.Join(owlcmsDir, version)
		startupLogPath := filepath.Join(versionDir, "logs", "startup.log")
		if err := os.Remove(startupLogPath); err != nil && !os.IsNotExist(err) {
			log.Printf("Warning: Failed to remove old startup.log: %v", err)
		}
	}

	log.Printf("Starting OWLCMS %s with command: %v\n", version, cmd.Args)
	if err := cmd.Start(); err != nil {
		statusLabel.SetText(fmt.Sprintf("Failed to start OWLCMS %s", version))
		releaseJavaLock()
		launchButton.Show()
		goBackToMainScreen()
		log.Printf("Failed to start OWLCMS %s: %v\n", version, err)
		return fmt.Errorf("failed to start OWLCMS %s: %w", version, err)
	}

	javaPID = cmd.Process.Pid
	if err := os.WriteFile(pidFilePath, []byte(fmt.Sprintf("%d\n", javaPID)), 0644); err != nil {
		log.Printf("Failed to write PID to PID file: %v\n", err)
	} else {
		log.Printf("Wrote PID %d to PID file %s\n", javaPID, pidFilePath)
	}

	log.Printf("Launching OWLCMS %s (PID: %d), waiting for port %s...\n", version, javaPID, GetPort())
	statusLabel.SetText(fmt.Sprintf("Starting OWLCMS %s (PID: %d), waiting for port %s.\nFull startup can take up to 30 seconds.", version, javaPID, GetPort()))
	currentProcess = cmd
	stopBtn.SetText(fmt.Sprintf("Stop OWLCMS %s", version))
	stopBtn.Show()
	stopContainer.Show()
	downloadContainer.Hide()
	versionContainer.Hide()
	setOwlcmsTabModeRunning()

	appDir := filepath.Join(installDir, version)
	appDirLink.SetText(fmt.Sprintf("Open OWLCMS %s directory", version))
	appDirLink.SetURL(nil)
	appDirLink.OnTapped = func() {
		shared.OpenFileExplorer(appDir)
	}
	appDirLink.Show()
	configureTailLogLink(version, appDir)

	// Start monitoring for startup.log (when supported by the OWLCMS version)
	go monitorStartupLog(appDir, version)

	monitorChan := monitorProcess(cmd)

	go func() {
		if err := <-monitorChan; err != nil {
			log.Printf("OWLCMS process %d failed to start properly: %v\n", javaPID, err)
			statusLabel.SetText(fmt.Sprintf("OWLCMS process %d failed to start properly", javaPID))
			stopBtn.Hide()
			stopContainer.Hide()
			launchButton.Show()
			currentProcess = nil
			setOwlcmsTabMode(mainWindow)

			releaseJavaLock()
			return
		}

		log.Printf("OWLCMS process %d is ready (port %s responding)\n", javaPID, GetPort())
		statusLabel.SetText(fmt.Sprintf("OWLCMS running (PID: %d) on port %s", javaPID, GetPort()))
		url := fmt.Sprintf("http://localhost:%s", GetPort())
		urlLink.SetURLFromString(url)
		urlLink.SetText("Open OWLCMS in a browser")
		urlLink.Show()

		// Close the startup log area now that OWLCMS is ready
		hideStartupLogArea()

		appDir := filepath.Join(installDir, version)
		appDirLink.SetText(fmt.Sprintf("Open OWLCMS %s directory", version))
		appDirLink.SetURL(nil)
		appDirLink.OnTapped = func() {
			shared.OpenFileExplorer(appDir)
		}
		appDirLink.Show()
		configureTailLogLink(version, appDir)

		err := cmd.Wait()
		pid := cmd.Process.Pid

		if killedByUs {
			log.Printf("OWLCMS %s (PID: %d) was stopped by user\n", version, pid)
			statusLabel.SetText(fmt.Sprintf("OWLCMS %s (PID: %d) was stopped by user", version, pid))
		} else if err != nil {
			log.Printf("OWLCMS %s (PID: %d) terminated with error: %v\n", version, pid, err)
			statusLabel.SetText(fmt.Sprintf("OWLCMS %s (PID: %d) terminated with error", version, pid))
		} else {
			log.Printf("OWLCMS %s (PID: %d) exited normally\n", version, pid)
			statusLabel.SetText(fmt.Sprintf("OWLCMS %s (PID: %d) exited normally", version, pid))
		}

		currentProcess = nil
		killedByUs = false
		stopBtn.Hide()
		stopContainer.Hide()
		launchButton.Show()
		setOwlcmsTabMode(mainWindow)

		urlLink.Hide()
		appDirLink.Hide()
		if tailLogLink != nil {
			tailLogLink.Hide()
		}
		hideStartupLogArea()
		releaseJavaLock()
	}()

	return nil
}

// showStartupLogArea creates and shows the startup log text area
func showStartupLogArea() {
	// Reset/initialize stop channel for this run
	startupLogMu.Lock()
	if startupLogStopCh != nil {
		select {
		case <-startupLogStopCh:
			// already closed
		default:
			close(startupLogStopCh)
		}
	}
	startupLogStopCh = make(chan struct{})
	stopCh := startupLogStopCh
	startupLogMu.Unlock()

	if startupLogText == nil {
		startupLogText = widget.NewMultiLineEntry()
		startupLogText.SetPlaceHolder("Waiting for startup log...")
		startupLogText.Wrapping = fyne.TextWrapWord
		startupLogText.OnChanged = func(s string) {
			// Keep it non-editable while remaining visually enabled.
			if startupLogUpdating {
				return
			}
			if s != startupLogLastText {
				startupLogUpdating = true
				startupLogText.SetText(startupLogLastText)
				startupLogUpdating = false
			}
		}
	}

	if startupLogContainer == nil {
		// Use a Border layout so the text area expands to fill remaining space.
		// VBox would size children to MinSize and leave unused space below.
		header := container.NewVBox(
			widget.NewSeparator(),
			widget.NewLabel("Startup Progress:"),
		)
		scroller := container.NewScroll(startupLogText)
		startupLogContainer = container.NewBorder(header, nil, nil, nil, scroller)
	}

	// Show within the running layout center so it can expand.
	if startupLogHost != nil {
		log.Printf("showStartupLogArea: Showing startup log container")
		startupLogHost.Objects = []fyne.CanvasObject{startupLogContainer}
		startupLogHost.Show()
		startupLogHost.Refresh()
	} else {
		log.Printf("showStartupLogArea: WARNING - startupLogHost is nil!")
	}

	// Auto-close after 60 seconds unless OWLCMS becomes ready first.
	go func(stopCh chan struct{}) {
		select {
		case <-time.After(60 * time.Second):
			hideStartupLogArea()
		case <-stopCh:
			return
		}
	}(stopCh)
}

// appendStartupLogText adds text to the startup log display
func appendStartupLogText(text string) {
	if startupLogText != nil {
		startupLogUpdating = true
		currentText := startupLogText.Text
		startupLogLastText = currentText + text
		startupLogText.SetText(startupLogLastText)
		// Move cursor to end so the latest text is visible
		startupLogText.CursorRow = len(strings.Split(startupLogLastText, "\n")) - 1
		startupLogUpdating = false
	}
}

// setStartupLogText sets the complete text in the startup log display
func setStartupLogText(text string) {
	if startupLogText != nil {
		startupLogUpdating = true
		startupLogLastText = text
		startupLogText.SetText(text)
		// Move cursor to end so the latest text is visible
		startupLogText.CursorRow = len(strings.Split(startupLogLastText, "\n")) - 1
		startupLogUpdating = false
	}
}

// hideStartupLogArea removes and hides the startup log display
func hideStartupLogArea() {
	startupLogMu.Lock()
	if startupLogStopCh != nil {
		select {
		case <-startupLogStopCh:
			// already closed
		default:
			close(startupLogStopCh)
		}
		startupLogStopCh = nil
	}
	startupLogMu.Unlock()

	if startupLogHost != nil {
		startupLogHost.Objects = nil
		startupLogHost.Hide()
		startupLogHost.Refresh()
	}
}

func owlcmsSupportsStartupLog(version string) bool {
	// Brutal normalization: ensure "-SNAPSHOT" prerelease compares correctly.
	// (ASCII sorting would treat 'S' differently; lowercase makes snapshot sort after rc.)
	normalized := strings.ReplaceAll(version, "-SNAPSHOT", "-snapshot")
	v, err := semver.NewVersion(normalized)
	if err != nil {
		log.Printf("Invalid OWLCMS version for semver comparison (%q): %v", version, err)
		return false
	}

	min, err := semver.NewVersion("64.0.0-rc08")
	if err != nil {
		// This should never happen; if it does, fail safe by disabling startup log.
		log.Printf("Internal error parsing minimum startup.log version: %v", err)
		return false
	}

	return !v.LessThan(min)
}

func monitorStartupLog(appDir, version string) {
	if !owlcmsSupportsStartupLog(version) {
		// Older OWLCMS versions don't generate logs/startup.log; don't poll for it.
		// Keep UI consistent: show an informational startup area while we wait for port readiness.
		showStartupLogArea()
		setStartupLogText(fmt.Sprintf(
			"OWLCMS %s does not generate logs/startup.log.\nWaiting for OWLCMS to respond on port %s...\n",
			version,
			GetPort(),
		))
		return
	}

	startupLogPath := filepath.Join(appDir, "logs", "startup.log")
	log.Printf("Monitoring for startup.log at: %s\n", startupLogPath)

	// Show the startup log area immediately
	showStartupLogArea()
	setStartupLogText("Waiting for startup.log...\n")

	// For testing: simulate dummy data
	// Toggle this to true only when doing UI-only testing.
	const startupLogTestMode = false
	if startupLogTestMode {
		log.Println("Testing mode: generating dummy startup log data")
		setStartupLogText("=== OWLCMS Test Startup Log ===\n")

		// Generate dummy data for up to 60 seconds, every 5 seconds,
		// or stop early if the startup log area is closed.
		startupLogMu.Lock()
		stopCh := startupLogStopCh
		startupLogMu.Unlock()
		go func() {
			for i := 0; i < 12; i++ {
				select {
				case <-stopCh:
					return
				default:
				}
				appendStartupLogText(fmt.Sprintf("[%02d:00] Initializing component %d...\n", i*5, i+1))
				select {
				case <-stopCh:
					return
				case <-time.After(5 * time.Second):
				}
			}
			appendStartupLogText("\n=== Startup dummy output complete (waiting for close/timeout) ===\n")
		}()
		return
	}

	// Real file monitoring
	// Check every 500ms for up to 60 seconds
	for i := 0; i < 120; i++ {
		if _, err := os.Stat(startupLogPath); err == nil {
			log.Printf("Found startup.log, starting to tail it\n")
			// Tail the file for 60 seconds
			go tailStartupLog(startupLogPath)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("startup.log not found after monitoring period\n")
	setStartupLogText("startup.log not found after monitoring period\n")
}

func tailStartupLog(logPath string) {
	log.Printf("Tailing startup log: %s\n", logPath)
	startupLogMu.Lock()
	stopCh := startupLogStopCh
	startupLogMu.Unlock()

	file, err := os.Open(logPath)
	if err != nil {
		log.Printf("Failed to open startup.log: %v\n", err)
		return
	}
	defer file.Close()

	// Read initial content
	content, err := os.ReadFile(logPath)
	if err == nil {
		setStartupLogText(string(content))
	}

	// Monitor for changes for 60 seconds (or until OWLCMS becomes ready)
	startTime := time.Now()
	lastSize := int64(0)
	if stat, err := file.Stat(); err == nil {
		lastSize = stat.Size()
	}

	foundReady := false
	for {
		select {
		case <-stopCh:
			return
		default:
		}

		if time.Since(startTime) > 60*time.Second {
			log.Println("Stopping startup log tail after 60 seconds")
			break
		}

		// Check for new content
		if stat, err := os.Stat(logPath); err == nil {
			if stat.Size() > lastSize {
				// Read the entire file again
				content, err := os.ReadFile(logPath)
				if err == nil {
					contentStr := string(content)
					setStartupLogText(contentStr)
					lastSize = stat.Size()

					// Check if OWLCMS is ready
					if !foundReady && (strings.Contains(contentStr, "OWLCMS Ready.") || strings.Contains(contentStr, "owlcms Ready.")) {
						foundReady = true
						log.Println("Found 'OWLCMS Ready.' in startup log")
					}
				}
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	// If we didn't find "OWLCMS Ready." after timeout, show error dialog
	if !foundReady {
		logsDir := filepath.Dir(logPath)
		go func() {
			dialog.ShowCustomConfirm("OWLCMS Startup Issue", "Open Logs Folder", "Close",
				widget.NewLabel("OWLCMS did not report ready status within 60 seconds.\nPlease check the logs and send them to owlcms@jflamy.dev if the issue persists."),
				func(ok bool) {
					if ok {
						shared.OpenFileExplorer(logsDir)
					}
				}, mainWindow)
		}()
		// Keep the log display visible so user can see what went wrong
	} else {
		// Close the startup log display only if startup was successful
		hideStartupLogArea()
	}
}
