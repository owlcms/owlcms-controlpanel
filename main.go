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

func getAllInstalledVersions() []string {
	entries, err := os.ReadDir(".")
	if err != nil {
		return nil
	}

	versionPattern := regexp.MustCompile(`^\d+\.\d+\.\d+(?:-(?:rc|alpha|beta)(?:\d+)?)?$`)
	var versions []string
	for _, entry := range entries {
		if entry.IsDir() && versionPattern.MatchString(entry.Name()) {
			versions = append(versions, entry.Name())
		}
	}

	sort.Sort(sort.Reverse(sort.StringSlice(versions)))
	return versions
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
	currentProcess *exec.Cmd
	currentVersion string // Add to track current version
	statusLabel    *widget.Label
	killedByUs     bool
)

func createStopButton(w fyne.Window) *widget.Button {
	stopButton := widget.NewButton("Stop", nil)
	stopButton.Hidden = true
	stopButton.OnTapped = func() {
		fmt.Printf("Stopping OWLCMS %s...\n", currentVersion)
		statusLabel.SetText(fmt.Sprintf("Stopping OWLCMS %s...", currentVersion))

		go func() {
			if currentProcess == nil || currentProcess.Process == nil {
				return
			}
			pid := currentProcess.Process.Pid
			killedByUs = true
			err := currentProcess.Process.Kill()
			if err != nil {
				killedByUs = false
				dialog.ShowError(fmt.Errorf("failed to stop OWLCMS %s (PID: %d): %w", currentVersion, pid, err), w)
				return
			}
			fmt.Printf("OWLCMS %s (PID: %d) has been stopped\n", currentVersion, pid)
			statusLabel.SetText(fmt.Sprintf("OWLCMS %s (PID: %d) has been stopped", currentVersion, pid))
			currentProcess = nil
			stopButton.Hide()
		}()
	}
	return stopButton
}

func launchOwlcms(version string, launchButton, stopButton *widget.Button) error {
	currentVersion = version // Store current version
	statusLabel.SetText(fmt.Sprintf("Starting OWLCMS %s...", version))
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
		statusLabel.SetText(fmt.Sprintf("Failed to start OWLCMS %s", version))
		launchButton.Show() // Show launch button again if start fails
		return fmt.Errorf("failed to start OWLCMS %s: %w", version, err)
	}

	fmt.Printf("Launching OWLCMS %s (PID: %d), waiting for port 8080...\n", version, cmd.Process.Pid)
	statusLabel.SetText(fmt.Sprintf("Starting OWLCMS %s (PID: %d), please wait...", version, cmd.Process.Pid))
	currentProcess = cmd
	stopButton.SetText(fmt.Sprintf("Stop OWLCMS %s", version))
	stopButton.Show()

	killedByUs = false // Reset flag when starting new process

	// Monitor the process in background
	monitorChan := monitorProcess(cmd)

	// Wait for monitoring result in background
	go func() {
		if err := <-monitorChan; err != nil {
			fmt.Printf("OWLCMS process %d failed to start properly: %v\n", cmd.Process.Pid, err)
			statusLabel.SetText(fmt.Sprintf("OWLCMS process %d failed to start properly", cmd.Process.Pid))
			stopButton.Hide()
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
			fmt.Printf("OWLCMS %s (PID: %d) was stopped by user\n", version, pid)
			statusLabel.SetText(fmt.Sprintf("OWLCMS %s (PID: %d) was stopped by user", version, pid))
		} else if err != nil {
			// Only report error if it wasn't killed by us
			fmt.Printf("OWLCMS %s (PID: %d) terminated with error: %v\n", version, pid, err)
			statusLabel.SetText(fmt.Sprintf("OWLCMS %s (PID: %d) terminated with error", version, pid))
		} else {
			fmt.Printf("OWLCMS %s (PID: %d) exited normally\n", version, pid)
			statusLabel.SetText(fmt.Sprintf("OWLCMS %s (PID: %d) exited normally", version, pid))
		}

		currentProcess = nil
		killedByUs = false // Reset flag
		stopButton.Hide()
		launchButton.Show()
	}()

	return nil
}

func main() {
	a := app.NewWithID("app.owlcms.owlcms-launcher")
	a.Settings().SetTheme(newMyTheme())
	w := a.NewWindow("OWLCMS Launcher")
	w.Resize(fyne.NewSize(600, 300)) // Larger initial window size

	progress := widget.NewProgressBarInfinite()
	loadingText := canvas.NewText("Fetching releases...", color.Black)
	loadingContainer := container.NewVBox(loadingText, progress)

	// Create release dropdown for downloads
	releaseDropdown := widget.NewSelect([]string{}, func(selected string) {
		urlPrefix := "https://github.com/owlcms/owlcms4-prerelease/releases/download"
		fileName := fmt.Sprintf("owlcms_%s.zip", selected)
		zipURL := fmt.Sprintf("%s/%s/%s", urlPrefix, selected, fileName)
		zipPath := fileName
		extractPath := selected

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
	releaseDropdown.PlaceHolder = "Choose a release to download"

	// Create version list
	versions := getAllInstalledVersions()

	// Create stop button
	stopButton := createStopButton(w)

	// Create version list with launch buttons
	versionList := widget.NewList(
		func() int { return len(versions) },
		func() fyne.CanvasObject {
			// Create template with version label and launch button side by side
			label := widget.NewLabel("Template")
			button := widget.NewButton("Launch", nil)
			return container.NewBorder(nil, nil, nil, button, label)
		},
		func(index widget.ListItemID, item fyne.CanvasObject) {
			// Get the container and its children
			cont := item.(*fyne.Container)
			label := cont.Objects[0].(*widget.Label)
			button := cont.Objects[1].(*widget.Button)

			// Set version text
			if index < len(versions) {
				version := versions[index]
				label.SetText(version)
				button.OnTapped = func() {
					if currentProcess != nil {
						dialog.ShowError(fmt.Errorf("OWLCMS is already running"), w)
						return
					}

					fmt.Printf("Launching version %s\n", version)
					if err := checkJava(); err != nil {
						dialog.ShowError(fmt.Errorf("java check/installation failed: %w", err), w)
						return
					}

					if err := launchOwlcms(version, button, stopButton); err != nil {
						dialog.ShowError(err, w)
						return
					}
				}
			}
		},
	)

	versionList.OnSelected = func(id widget.ListItemID) {
		if id < len(versions) {
			// Only log selection, no need to store index
			fmt.Printf("Selected version: %s\n", versions[id])
		}
	}

	// Select first version by default if available
	if len(versions) > 0 {
		versionList.Select(0)
	}

	// Set initial selection using findLatestInstalled
	if latest := findLatestInstalled(); latest != "" {
		// Find index of latest version
		for i, v := range versions {
			if v == latest {
				versionList.Select(i)
				break
			}
		}
	}

	// Create scroll container for version list with dynamic size
	versionScroll := container.NewVScroll(versionList)
	minHeight := 50 // minimum height
	rowHeight := 40 // approximate height per row
	numVersions := len(versions)
	if numVersions > 0 {
		// Set height based on number of versions, but cap at 4 rows
		height := minHeight + (rowHeight * min(numVersions, 4))
		versionScroll.SetMinSize(fyne.NewSize(400, float32(height)))
	} else {
		versionScroll.SetMinSize(fyne.NewSize(400, float32(minHeight)))
	}

	statusLabel = widget.NewLabel("")
	statusLabel.Wrapping = fyne.TextWrapWord // Allow status messages to wrap

	// Create layout with tighter grouping of controls
	installedGroup := container.NewVBox(
		widget.NewLabel("Installed Versions:"),
		versionScroll,
		container.NewVBox( // Put controls directly under version list
			container.NewHBox(stopButton),
			statusLabel,
		),
	)

	downloadGroup := container.NewVBox(
		widget.NewLabelWithStyle("Download New Version", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Download and install a new version of OWLCMS from GitHub:"),
		releaseDropdown,
	)

	mainContent := container.NewVBox(
		widget.NewLabelWithStyle("OWLCMS Launcher", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		installedGroup,
		container.NewHBox(
			downloadGroup,
			widget.NewLabel(""),
		),
	)

	w.SetContent(loadingContainer)
	w.Resize(fyne.NewSize(800, 600))

	go func() {
		releases, err := fetchReleases()
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		releaseDropdown.Options = releases // Set the available releases in dropdown
		w.SetContent(mainContent)
		w.Canvas().Refresh(mainContent)
	}()

	w.ShowAndRun()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
