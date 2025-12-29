package owlcms

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"owlcms-launcher/owlcms/javacheck"
	"owlcms-launcher/shared"

	"fyne.io/fyne/v2/widget"
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
			p := strings.ReplaceAll(logPath, "\\", "\\\\")
			p = strings.ReplaceAll(p, "\"", "\\\"")
			script := fmt.Sprintf(`tell application "Terminal" to do script "tail -n 10 -f \\\"%s\\\""`, p)
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

	localJava, err := javacheck.FindLocalJava()
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
	env = append(env, fmt.Sprintf("OWLCMS_LAUNCHER=%s", version))

	for _, key := range environment.Keys() {
		value, _ := environment.Get(key)
		log.Printf("   %s=%s", key, value)
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	cmd := exec.Command(localJava, "-jar", "owlcms.jar")
	cmd.Env = env
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

	appDir := filepath.Join(installDir, version)
	appDirLink.SetText(fmt.Sprintf("Open OWLCMS %s directory", version))
	appDirLink.SetURL(nil)
	appDirLink.OnTapped = func() {
		shared.OpenFileExplorer(appDir)
	}
	appDirLink.Show()
	configureTailLogLink(version, appDir)

	monitorChan := monitorProcess(cmd)

	go func() {
		if err := <-monitorChan; err != nil {
			log.Printf("OWLCMS process %d failed to start properly: %v\n", javaPID, err)
			statusLabel.SetText(fmt.Sprintf("OWLCMS process %d failed to start properly", javaPID))
			stopBtn.Hide()
			stopContainer.Hide()
			launchButton.Show()
			currentProcess = nil
			downloadContainer.Show()
			versionContainer.Show()
			releaseJavaLock()
			return
		}

		log.Printf("OWLCMS process %d is ready (port %s responding)\n", javaPID, GetPort())
		statusLabel.SetText(fmt.Sprintf("OWLCMS running (PID: %d) on port %s", javaPID, GetPort()))
		url := fmt.Sprintf("http://localhost:%s", GetPort())
		urlLink.SetURLFromString(url)
		urlLink.SetText("Open OWLCMS in a browser")
		urlLink.Show()

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
		downloadContainer.Show()
		versionContainer.Show()
		urlLink.Hide()
		appDirLink.Hide()
		if tailLogLink != nil {
			tailLogLink.Hide()
		}
		releaseJavaLock()
	}()

	return nil
}
