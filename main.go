package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"owlcms-launcher/downloadUtils"
	"owlcms-launcher/javacheck"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type Release struct {
	Name string `json:"name"`
}

type myTheme struct {
	fyne.Theme
}

func newMyTheme() *myTheme {
	return &myTheme{Theme: theme.LightTheme()}
}

func (m myTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNameBackground {
		return color.White
	}
	if name == theme.ColorNameForeground {
		return color.Black
	}
	if name == theme.ColorNameShadow {
		return color.NRGBA{R: 0, G: 0, B: 0, A: 40}
	}
	return m.Theme.Color(name, variant)
}

func fetchReleases() ([]string, error) {
	url := "https://api.github.com/repos/owlcms/owlcms4-prerelease/releases"
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var releases []Release
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, fmt.Errorf("invalid response format: %w", err)
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}

	releaseNames := make([]string, 0, 10)
	for i, release := range releases {
		if i >= 10 {
			break
		}
		releaseNames = append(releaseNames, release.Name)
	}

	return releaseNames, nil
}

func findLatestInstalled() string {
	entries, err := os.ReadDir(".")
	if err != nil {
		return ""
	}

	// Version pattern that matches x.x.x and optional -rc/-alpha/-beta suffix
	versionPattern := regexp.MustCompile(`^\d+\.\d+\.\d+(?:-(?:rc|alpha|beta)(?:\d+)?)?$`)
	var versions []string
	for _, entry := range entries {
		if entry.IsDir() && versionPattern.MatchString(entry.Name()) {
			versions = append(versions, entry.Name())
		}
	}

	if len(versions) == 0 {
		return ""
	}

	sort.Sort(sort.Reverse(sort.StringSlice(versions)))
	return versions[0]
}

func checkJava() error {
	return javacheck.CheckJava()
}

// checkPort tries to connect to localhost:8080 and returns nil if successful
func checkPort() error {
	resp, err := http.Get("http://localhost:8080")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func monitorProcess(cmd *exec.Cmd) chan error {
	result := make(chan error, 1)
	go func() {
		// Start a goroutine to wait for process exit
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		// Try connecting to port 8080 for up to 60 seconds
		timeout := time.After(60 * time.Second)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case err := <-done:
				// Process exited before port was available
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

var (
	currentProcess   *exec.Cmd
	globalStopButton *widget.Button
	statusLabel      *widget.Label // Add status label
	killedByUs       bool          // Add flag to track if we initiated the kill
)

func stopOwlcms() error {
	if currentProcess != nil && currentProcess.Process != nil {
		pid := currentProcess.Process.Pid
		killedByUs = true // Set flag before killing
		err := currentProcess.Process.Kill()
		if err != nil {
			killedByUs = false // Reset flag if kill failed
			return fmt.Errorf("failed to stop OWLCMS (PID: %d): %w", pid, err)
		}
		fmt.Printf("Stopped OWLCMS (PID: %d)\n", pid)
		// Add death confirmation
		fmt.Printf("OWLCMS process %d has been terminated\n", pid)
		statusLabel.SetText(fmt.Sprintf("OWLCMS process %d has been terminated", pid))
		currentProcess = nil
		return nil
	}
	return nil
}

// isKilled checks if the process was killed (either by us or externally)
func isKilled(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "signal: killed" ||
		err.Error() == "os: process already finished" ||
		strings.Contains(err.Error(), "process was killed")
}

func launchOwlcms(version string, launchButton *widget.Button) error {
	statusLabel.SetText("Starting OWLCMS...")
	// Store current directory to restore it later
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	// Look for owlcms.jar in the version directory
	jarPath := filepath.Join(version, "owlcms.jar")
	if _, err := os.Stat(jarPath); os.IsNotExist(err) {
		launchButton.Show() // Show launch button again if start fails
		return fmt.Errorf("owlcms.jar not found in %s directory", version)
	}

	// Change to version directory
	if err := os.Chdir(version); err != nil {
		launchButton.Show() // Show launch button again if start fails
		return fmt.Errorf("changing to version directory: %w", err)
	}
	defer os.Chdir(originalDir)

	javaCmd := "java"
	localJava := filepath.Join(originalDir, "java17", "bin", "java")
	if runtime.GOOS == "windows" {
		localJava += ".exe"
	}
	if _, err := os.Stat(localJava); err == nil {
		javaCmd = localJava
	}

	cmd := exec.Command(javaCmd, "-jar", "owlcms.jar")
	if err := cmd.Start(); err != nil {
		statusLabel.SetText("Failed to start OWLCMS")
		launchButton.Show() // Show launch button again if start fails
		return fmt.Errorf("failed to start OWLCMS: %w", err)
	}

	fmt.Printf("Launching OWLCMS (PID: %d), waiting for port 8080...\n", cmd.Process.Pid)
	statusLabel.SetText(fmt.Sprintf("Starting OWLCMS (PID: %d), please wait...", cmd.Process.Pid))
	currentProcess = cmd
	globalStopButton.Show()

	killedByUs = false // Reset flag when starting new process

	// Monitor the process in background
	monitorChan := monitorProcess(cmd)

	// Wait for monitoring result in background
	go func() {
		if err := <-monitorChan; err != nil {
			fmt.Printf("OWLCMS process %d failed to start properly: %v\n", cmd.Process.Pid, err)
			statusLabel.SetText(fmt.Sprintf("OWLCMS process %d failed to start properly", cmd.Process.Pid))
			globalStopButton.Hide()
			launchButton.Show()
			currentProcess = nil
			return
		}

		fmt.Printf("OWLCMS process %d is ready (port 8080 responding)\n", cmd.Process.Pid)
		statusLabel.SetText(fmt.Sprintf("OWLCMS running (PID: %d)", cmd.Process.Pid))

		// Process is stable, wait for it to end
		err := cmd.Wait()
		pid := cmd.Process.Pid

		if killedByUs {
			// If we killed it, just report normal termination
			fmt.Printf("OWLCMS process %d was stopped by user\n", pid)
			statusLabel.SetText(fmt.Sprintf("OWLCMS process %d was stopped by user", pid))
		} else if err != nil {
			// Only report error if it wasn't killed by us
			fmt.Printf("OWLCMS process %d terminated with error: %v\n", pid, err)
			statusLabel.SetText(fmt.Sprintf("OWLCMS process %d terminated with error", pid))
		} else {
			fmt.Printf("OWLCMS process %d exited normally\n", pid)
			statusLabel.SetText(fmt.Sprintf("OWLCMS process %d exited normally", pid))
		}

		currentProcess = nil
		killedByUs = false // Reset flag
		globalStopButton.Hide()
		launchButton.Show()
	}()

	return nil
}

func createButtons(w fyne.Window) (*widget.Button, *widget.Button) {
	var stopButton *widget.Button
	var launchButton *widget.Button

	stopCallback := func() {
		fmt.Println("Stopping OWLCMS...")         // Log immediately when button is clicked
		statusLabel.SetText("Stopping OWLCMS...") // Update GUI immediately

		// Now do the actual stopping in background
		go func() {
			if currentProcess == nil || currentProcess.Process == nil {
				return
			}
			pid := currentProcess.Process.Pid
			fmt.Printf("Attempting to stop OWLCMS process (PID: %d)...\n", pid)
			statusLabel.SetText(fmt.Sprintf("Stopping OWLCMS process %d...", pid))

			if err := stopOwlcms(); err != nil {
				dialog.ShowError(err, w)
				statusLabel.SetText(fmt.Sprintf("Failed to stop OWLCMS process %d", pid))
				return
			}
			globalStopButton.Hide()
			launchButton.Show()
		}()
	}

	stopButton = widget.NewButton("Stop OWLCMS", stopCallback)
	stopButton.Hide()
	globalStopButton = stopButton

	launchButton = widget.NewButton("Launch OWLCMS", func() {
		version := findLatestInstalled()
		if version == "" {
			dialog.ShowError(fmt.Errorf("no OWLCMS version installed"), w)
			return
		}

		fmt.Println("Launching OWLCMS...")
		if err := checkJava(); err != nil {
			dialog.ShowError(fmt.Errorf("java check/installation failed: %w", err), w)
			return
		}

		launchButton.Hide() // Hide launch button immediately when clicked

		if err := launchOwlcms(version, launchButton); err != nil {
			dialog.ShowError(err, w)
			return
		}
	})

	return launchButton, stopButton
}

func main() {
	a := app.NewWithID("app.owlcms.owlcms-launcher")
	a.Settings().SetTheme(newMyTheme())
	w := a.NewWindow("OWLCMS Launcher")
	w.Resize(fyne.NewSize(600, 300)) // Larger initial window size

	progress := widget.NewProgressBarInfinite()
	loadingText := canvas.NewText("Fetching releases...", color.Black)
	loadingContainer := container.NewVBox(loadingText, progress)

	releaseLabel := widget.NewLabel("Select OWLCMS Release:")
	releaseDropdown := widget.NewSelect([]string{}, func(selected string) {
		urlPrefix := "https://github.com/owlcms/owlcms4-prerelease/releases/download"
		fileName := fmt.Sprintf("owlcms_%s.zip", selected)
		zipURL := fmt.Sprintf("%s/%s/%s", urlPrefix, selected, fileName)
		zipPath := fileName
		extractPath := selected // Use the release version as subdirectory

		dialog.ShowConfirm("Confirm Download",
			fmt.Sprintf("Do you want to download and install OWLCMS version %s?", selected),
			func(ok bool) {
				if !ok {
					return
				}

				// Show progress dialog
				progressDialog := dialog.NewCustom(
					"Installing OWLCMS",
					"Please wait...",
					widget.NewTextGridFromString("Downloading and extracting files..."),
					w)
				progressDialog.Show()

				// Download the ZIP file using downloadUtils
				err := downloadUtils.DownloadZip(zipURL, zipPath)
				if err != nil {
					progressDialog.Hide()
					dialog.ShowError(fmt.Errorf("download failed: %w", err), w)
					return
				}

				// Extract the ZIP file to version-specific subdirectory
				err = downloadUtils.ExtractZip(zipPath, extractPath)
				if err != nil {
					progressDialog.Hide()
					dialog.ShowError(fmt.Errorf("extraction failed: %w", err), w)
					return
				}

				// Hide progress dialog
				progressDialog.Hide()

				// Show success panel with installation details
				message := fmt.Sprintf(
					"Successfully installed OWLCMS version %s\n\n"+
						"Location: %s\n\n"+
						"The program files have been extracted to the above directory.",
					selected, extractPath)

				dialog.ShowInformation("Installation Complete", message, w)
			},
			w)
	})
	releaseDropdown.PlaceHolder = "Choose a release version"

	launchButton, stopButton := createButtons(w)

	statusLabel = widget.NewLabel("") // Create status label

	mainContent := container.NewVBox(
		widget.NewLabel("OWLCMS Launcher"),
		releaseLabel,
		releaseDropdown,
		container.NewHBox(launchButton, stopButton),
		statusLabel, // Add status label to UI
	)

	w.SetContent(loadingContainer)
	w.Resize(fyne.NewSize(800, 600))

	go func() {
		releases, err := fetchReleases()
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		releaseDropdown.Options = releases
		w.SetContent(mainContent)
		w.Canvas().Refresh(mainContent)
	}()

	w.ShowAndRun()
}
