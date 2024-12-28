package main

import (
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"owlcms-launcher/downloadUtils"
	"owlcms-launcher/javacheck"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/Masterminds/semver/v3"
)

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

func checkJava() error {
	return javacheck.CheckJava()
}

var (
	currentProcess   *exec.Cmd
	currentVersion   string // Add to track current version
	statusLabel      *widget.Label
	stopButton       *widget.Button
	versionContainer *fyne.Container
	stopContainer    *fyne.Container
)

func launchOwlcms(version string, launchButton, stopButton *widget.Button, downloadGroup, versionContainer *fyne.Container) error {
	currentVersion = version // Store current version

	// Check if port 8080 is already in use
	if err := checkPort(); err == nil {
		return fmt.Errorf("OWLCMS is already running on port 8080")
	}

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
	if runtime.GOOS == "windows" && !downloadUtils.IsWSL() {
		localJava = filepath.Join(originalDir, "java17", "bin", "javaw.exe")
		javaCmd = "javaw"
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
	stopContainer.Show()
	downloadGroup.Hide()
	versionContainer.Hide()

	// Monitor the process in background
	monitorChan := monitorProcess(cmd)

	// Wait for monitoring result in background
	go func() {
		if err := <-monitorChan; err != nil {
			fmt.Printf("OWLCMS process %d failed to start properly: %v\n", cmd.Process.Pid, err)
			statusLabel.SetText(fmt.Sprintf("OWLCMS process %d failed to start properly", cmd.Process.Pid))
			stopButton.Hide()
			stopContainer.Hide()
			launchButton.Show()
			currentProcess = nil
			downloadGroup.Show()
			versionContainer.Show()
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
		stopContainer.Hide()
		launchButton.Show()
		downloadGroup.Show()
		versionContainer.Show()
	}()

	return nil
}

func fetchReleasesInBackground(releasesChan chan<- []string, errChan chan<- error) {
	time.Sleep(1 * time.Second) // Wait for 1 second before attempting to retrieve the releases
	releases, err := fetchReleases()
	if err != nil {
		errChan <- err
		return
	}
	releasesChan <- releases
}

func main() {
	a := app.NewWithID("app.owlcms.owlcms-launcher")
	a.Settings().SetTheme(newMyTheme())
	w := a.NewWindow("OWLCMS Launcher")
	w.Resize(fyne.NewSize(600, 300)) // Larger initial window size

	// Create stop button and status label
	stopButton = widget.NewButton("Stop", nil)
	statusLabel = widget.NewLabel("")
	statusLabel.Wrapping = fyne.TextWrapWord // Allow status messages to wrap

	// Create containers
	downloadGroup := container.NewVBox()
	versionContainer = container.NewVBox()
	stopContainer = container.NewVBox(stopButton, statusLabel)

	// Configure stop button behavior
	stopButton.OnTapped = func() {
		stopProcess(currentProcess, currentVersion, stopButton, downloadGroup, versionContainer, statusLabel, w)
	}
	stopButton.Hide()
	stopContainer.Hide()

	// Initialize version list
	versionList := createVersionList(w, stopButton, downloadGroup, versionContainer)

	// Create scroll container for version list with dynamic size
	versionScroll := container.NewVScroll(versionList)
	minHeight := 50 // minimum height
	rowHeight := 40 // approximate height per row
	numVersions := len(getAllInstalledVersions())
	if numVersions > 0 {
		// Set height based on number of versions, but cap at 4 rows
		height := minHeight + (rowHeight * min(numVersions, 4))
		versionScroll.SetMinSize(fyne.NewSize(400, float32(height)))
	} else {
		versionScroll.SetMinSize(fyne.NewSize(400, float32(minHeight)))
	}

	// Create more compact layout without padding
	versionContainer.Objects = []fyne.CanvasObject{
		widget.NewLabel("Installed Versions:"),
		versionScroll,
	}

	// Create release dropdown for downloads
	releaseTitle := widget.NewLabelWithStyle("Download New Version", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	releaseMessage := widget.NewLabel("Download and install a new version")
	releaseDropdown := createReleaseDropdown(w, downloadGroup)
	releaseTitle.Hide()
	releaseMessage.Hide()
	releaseDropdown.Hide() // Hide the dropdown initially

	// Create checkbox for showing prereleases
	prereleaseCheckbox := widget.NewCheck("Show Prereleases", func(checked bool) {
		showPrereleases = checked
		populateReleaseDropdown(releaseDropdown) // Repopulate the dropdown when the checkbox is changed
	})
	prereleaseCheckbox.Hide() // Hide the checkbox initially

	downloadGroup.Objects = []fyne.CanvasObject{
		releaseTitle,
		releaseMessage,
		releaseDropdown,
		prereleaseCheckbox,
	}

	mainContent := container.NewVBox(
		widget.NewLabelWithStyle("OWLCMS Launcher", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		stopContainer,
		versionContainer,
		container.NewHBox(
			downloadGroup,
			widget.NewLabel(""),
		),
	)

	w.SetContent(mainContent)
	w.Resize(fyne.NewSize(800, 600))

	// Show installed versions first
	w.SetContent(mainContent)
	w.Canvas().Refresh(mainContent)

	releasesChan := make(chan []string)
	errChan := make(chan error)

	// Show retrieving releases label
	retrievingLabel := widget.NewLabel("Checking for updates...")
	downloadGroup.Objects = append(downloadGroup.Objects, retrievingLabel)
	w.Canvas().Refresh(mainContent)

	go fetchReleasesInBackground(releasesChan, errChan)

	go func() {
		select {
		case releases := <-releasesChan:
			allReleases = releases                   // Store all releases
			populateReleaseDropdown(releaseDropdown) // Populate the dropdown with the releases
			releaseTitle.Show()
			releaseMessage.Show()
			releaseDropdown.Show()
			prereleaseCheckbox.Show()                                                    // Show the checkbox once releases are fetched
			downloadGroup.Objects = downloadGroup.Objects[:len(downloadGroup.Objects)-1] // Remove retrieving label

			// Check if a more recent version is available
			latestInstalled := findLatestInstalled()
			if latestInstalled != "" {
				latestInstalledVersion, err := semver.NewVersion(latestInstalled)
				if err == nil {
					for _, release := range allReleases {
						releaseVersion, err := semver.NewVersion(release)
						// fmt.Printf("Comparing with release version: %s\n", releaseVersion)
						if err == nil && releaseVersion.GreaterThan(latestInstalledVersion) {
							fmt.Printf("A more recent stable version is available %s > %s\n", releaseVersion, latestInstalledVersion)
							releaseTitle.SetText("A more recent stable version is available (" + release + ")")
							break
						}
					}
				}
			}

			w.Canvas().Refresh(mainContent)
		case err := <-errChan:
			fmt.Printf("Error fetching releases: %v\n", err)
			downloadGroup.Objects = []fyne.CanvasObject{
				widget.NewLabelWithStyle("Internet access not available, cannot show the available versions", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			}
			w.Canvas().Refresh(mainContent)
		case <-time.After(10 * time.Second):
			downloadGroup.Objects = []fyne.CanvasObject{
				widget.NewLabelWithStyle("Internet access not available, cannot show the available versions", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			}
			w.Canvas().Refresh(mainContent)
		}
		// Ensure the retrieving label is hidden in all cases
		retrievingLabel.Hide()
		w.Canvas().Refresh(mainContent)
	}()

	w.SetCloseIntercept(func() {
		if currentProcess != nil {
			stopProcess(currentProcess, currentVersion, stopButton, downloadGroup, versionContainer, statusLabel, w)
		}
		w.Close()
	})

	w.ShowAndRun()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
