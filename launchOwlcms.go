package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"owlcms-launcher/downloadUtils"
	"owlcms-launcher/javacheck"

	"fyne.io/fyne/v2/widget"
	"github.com/gofrs/flock"
	"github.com/shirou/gopsutil/process"
)

var (
	lockFilePath = filepath.Join(owlcmsInstallDir, "java.lock")
	pidFilePath  = filepath.Join(owlcmsInstallDir, "java.pid")
	javaPID      int          // Add a global variable to store the Java process PID
	lock         *flock.Flock // Add a global variable to store the lock
)

func acquireJavaLock() (*flock.Flock, error) {
	// lock := flock.New(lockFilePath)
	// locked, err := lock.TryLock()
	// if err != nil {
	// 	log.Printf("Failed to acquire lock %s: %v", lockFilePath, err)
	// 	return nil, fmt.Errorf("failed to acquire lock: %w", err)
	// }
	// if !locked {
	// 	// we could not lock, so the lock owner should have written a PID to the file
	data, err := os.ReadFile(pidFilePath)
	if err == nil && len(data) > 0 {
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err == nil && pid != 0 {
			log.Printf("Another instance of OWLCMS is already running with PID %d", pid)
			return nil, fmt.Errorf("another instance of OWLCMS is already running with PID %d", pid)
		} else {
			log.Printf("Failed to parse PID from PID file: %v", err)
			os.Remove(pidFilePath)
		}
	} else {
		return nil, nil
	}

	// }

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

	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		releaseJavaLock()
		return fmt.Errorf("failed to find process with PID %d: %w", pid, err)

	}

	if downloadUtils.GetGoos() == "windows" && !downloadUtils.IsWSL() {
		if err := proc.Terminate(); err != nil {
			releaseJavaLock()
			return fmt.Errorf("failed to terminate process with PID %d: %w", pid, err)
		}
	} else {
		if err := proc.SendSignal(syscall.SIGKILL); err != nil {
			releaseJavaLock()
			return fmt.Errorf("failed to kill process with PID %d: %w", pid, err)
		}
	}
	releaseJavaLock()
	log.Printf("Killed process with PID %d\n", pid)
	return nil
}

func launchOwlcms(version string, launchButton, stopButton *widget.Button) error {
	currentVersion = version // Store current version

	// Acquire lock file
	var err error
	lock, err = acquireJavaLock()
	if err != nil {
		// failure to lock has already shown the error message
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

	statusLabel.SetText(fmt.Sprintf("Starting OWLCMS %s...", version))
	statusLabel.Refresh()
	statusLabel.Show() // Show the status label when starting Java
	// Store current directory to restore it later
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	// Ensure the owlcms directory exists
	owlcmsDir := owlcmsInstallDir
	if _, err := os.Stat(owlcmsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(owlcmsDir, 0755); err != nil {
			return fmt.Errorf("creating owlcms directory: %w", err)
		}
	}

	// Look for owlcms.jar in the version directory
	jarPath := filepath.Join(owlcmsDir, version, "owlcms.jar")
	if _, err := os.Stat(jarPath); os.IsNotExist(err) {
		launchButton.Show() // Show launch button again if start fails
		return fmt.Errorf("owlcms.jar not found in %s directory", jarPath)
	}

	// Change to version directory
	if err := os.Chdir(filepath.Join(owlcmsDir, version)); err != nil {
		launchButton.Show() // Show launch button again if start fails
		return fmt.Errorf("changing to version directory: %w", err)
	}
	defer os.Chdir(originalDir)

	// find the java runtime binary
	localJava, err := javacheck.FindLocalJava()
	if err != nil {
		statusLabel.SetText(fmt.Sprintf("Failed to find local Java: %v", err))
		launchButton.Show()
		goBackToMainScreen()
		return fmt.Errorf("failed to find local Java: %w", err)
	}

	env := os.Environ()
	env = append(env, fmt.Sprintf("OWLCMS_LAUNCHER=%s", version))

	// Add all properties from environment to process env
	for _, key := range environment.Keys() {
		value, _ := environment.Get(key)
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	// Log all environment variables before starting Java
	log.Printf("Environment variables for OWLCMS %s:", version)
	for _, envVar := range env {
		log.Printf("  %s", envVar)
	}

	// Start the Java process
	cmd := exec.Command(localJava, "-jar", "owlcms.jar")
	cmd.Env = env
	log.Printf("Starting OWLCMS %s with command: %v\n", version, cmd.Args)
	if err := cmd.Start(); err != nil {
		statusLabel.SetText(fmt.Sprintf("Failed to start OWLCMS %s", version))
		releaseJavaLock()
		launchButton.Show() // Show launch button again if start fails
		goBackToMainScreen()
		log.Printf("Failed to start OWLCMS %s: %v\n", version, err)
		return fmt.Errorf("failed to start OWLCMS %s: %w", version, err)
	}

	// Store the PID in the PID file and globally
	javaPID = cmd.Process.Pid
	if err := os.WriteFile(pidFilePath, []byte(fmt.Sprintf("%d\n", javaPID)), 0644); err != nil {
		log.Printf("Failed to write PID to PID file: %v\n", err)
	} else {
		log.Printf("Wrote PID %d to PID file %s\n", javaPID, pidFilePath)
	}

	log.Printf("Launching OWLCMS %s (PID: %d), waiting for port %s...\n", version, javaPID, GetPort())
	statusLabel.SetText(fmt.Sprintf("Starting OWLCMS %s (PID: %d), please wait. Full startup can take up to 30 seconds.", version, javaPID))
	currentProcess = cmd
	stopButton.SetText(fmt.Sprintf("Stop OWLCMS %s", version))
	stopButton.Show()
	stopContainer.Show()
	downloadContainer.Hide()
	versionContainer.Hide()

	// Monitor the process in background
	monitorChan := monitorProcess(cmd)

	// Wait for monitoring result in background
	go func() {
		if err := <-monitorChan; err != nil {
			log.Printf("OWLCMS process %d failed to start properly: %v\n", javaPID, err)
			statusLabel.SetText(fmt.Sprintf("OWLCMS process %d failed to start properly", javaPID))
			stopButton.Hide()
			stopContainer.Hide()
			launchButton.Show()
			currentProcess = nil
			downloadContainer.Show()
			versionContainer.Show()
			releaseJavaLock()
			return
		}

		log.Printf("OWLCMS process %d is ready (port 8080 responding)\n", javaPID)
		statusLabel.SetText(fmt.Sprintf("OWLCMS running (PID: %d)", javaPID))

		// Process is stable, wait for it to end
		err := cmd.Wait()
		pid := cmd.Process.Pid

		if killedByUs {
			// If we killed it, just report normal termination
			log.Printf("OWLCMS %s (PID: %d) was stopped by user\n", version, pid)
			statusLabel.SetText(fmt.Sprintf("OWLCMS %s (PID: %d) was stopped by user", version, pid))
		} else if err != nil {
			// Only report error if it wasn't killed by us
			log.Printf("OWLCMS %s (PID: %d) terminated with error: %v\n", version, pid, err)
			statusLabel.SetText(fmt.Sprintf("OWLCMS %s (PID: %d) terminated with error", version, pid))
		} else {
			log.Printf("OWLCMS %s (PID: %d) exited normally\n", version, pid)
			statusLabel.SetText(fmt.Sprintf("OWLCMS %s (PID: %d) exited normally", version, pid))
		}

		currentProcess = nil
		killedByUs = false // Reset flag
		stopButton.Hide()
		stopContainer.Hide()
		launchButton.Show()
		downloadContainer.Show()
		versionContainer.Show()
		releaseJavaLock()
	}()

	return nil
}
