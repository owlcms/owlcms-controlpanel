package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"owlcms-launcher/javacheck"

	"fyne.io/fyne/v2/widget"
	"github.com/gofrs/flock"
)

var (
	lockFilePath = filepath.Join(owlcmsInstallDir, "java.lock")
	javaPID      int          // Add a global variable to store the Java process PID
	lock         *flock.Flock // Add a global variable to store the lock
)

func acquireJavaLock() (*flock.Flock, error) {
	lock := flock.New(lockFilePath)
	locked, err := lock.TryLock()
	if err != nil {
		log.Print("Failed to acquire lock: ", err)
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}
	if !locked {
		data, err := os.ReadFile(lockFilePath)
		if err == nil && len(data) > 0 {
			pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err == nil && pid != 0 {
				log.Printf("Another instance of OWLCMS is already running with PID %d", pid)
				return nil, fmt.Errorf("another instance of OWLCMS is already running with PID %d", pid)
			}
		}
		log.Println("Another instance of OWLCMS is already running")
		return nil, fmt.Errorf("another instance of OWLCMS is already running")
	}

	// we acquired the lock but someone has written a PID to the file
	data, err := os.ReadFile(lockFilePath)
	if err == nil && len(data) > 0 {
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err == nil && pid != 0 {
			log.Printf("Another instance of OWLCMS is already running with PID %d", pid)
			return nil, fmt.Errorf("another instance of OWLCMS is already running with PID %d", pid)
		}
	}

	return lock, nil
}

func releaseJavaLock() {
	// write empty content to the lock file
	if err := os.WriteFile(lockFilePath, []byte{}, 0644); err != nil {
		log.Printf("Failed to clear lock file content: %v\n", err)
	}
	log.Println("Released Java lock")
	if lock != nil {
		lock.Unlock()
		lock = nil
	}

}

func launchOwlcms(version string, launchButton, stopButton *widget.Button) error {
	currentVersion = version // Store current version

	// Acquire lock file
	var err error
	lock, err = acquireJavaLock()
	if err != nil {
		// acquring lock already shows an error message
		goBackToMainScreen()
		return err
	}

	// Check if port 8080 is already in use
	if err := checkPort(); err == nil {
		statusLabel.SetText("Another program is running on port 8080")
		statusLabel.Refresh()
		goBackToMainScreen()
		log.Println("Another program is running on port 8080")
		return fmt.Errorf("another program is running on port 8080")
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
		if err := os.Mkdir(owlcmsDir, 0755); err != nil {
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

	// Start OWLCMS in the background
	localJava, err := javacheck.FindLocalJava()
	if err != nil {
		statusLabel.SetText(fmt.Sprintf("Failed to find local Java: %v", err))
		launchButton.Show()
		goBackToMainScreen()
		return fmt.Errorf("failed to find local Java: %w", err)
	}

	env := os.Environ()
	env = append(env, "OWLCMS_LAUNCHER=true")
	cmd := exec.Command(localJava, "-jar", "owlcms.jar")
	log.Printf("Starting OWLCMS %s with command: %v\n", version, cmd.Args)
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		statusLabel.SetText(fmt.Sprintf("Failed to start OWLCMS %s", version))
		launchButton.Show() // Show launch button again if start fails
		goBackToMainScreen()
		log.Printf("Failed to start OWLCMS %s: %v\n", version, err)
		return fmt.Errorf("failed to start OWLCMS %s: %w", version, err)
	}

	// Store the PID in the lock file and globally
	javaPID = cmd.Process.Pid
	if err := os.WriteFile(lockFilePath, []byte(fmt.Sprintf("%d\n", javaPID)), 0644); err != nil {
		log.Printf("Failed to write PID to lock file: %v\n", err)
	} else {
		log.Printf("Wrote PID %d to lock file %s\n", javaPID, lockFilePath)
	}

	log.Printf("Launching OWLCMS %s (PID: %d), waiting for port 8080...\n", version, javaPID)
	statusLabel.SetText(fmt.Sprintf("Starting OWLCMS %s (PID: %d), please wait.  Full startup can take up to 30 seconds.", version, javaPID))
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
