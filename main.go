package main

import (
	"fmt"
	"image/color"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"owlcms-launcher/downloadUtils"
	"owlcms-launcher/javacheck"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/Masterminds/semver/v3"
)

var (
	owlcmsInstallDir          = getInstallDir()
	currentProcess            *exec.Cmd
	currentVersion            string // Add to track current version
	statusLabel               *widget.Label
	stopButton                *widget.Button
	versionContainer          *fyne.Container
	stopContainer             *fyne.Container
	singleOrMultiVersionLabel *widget.Label   // New label for single or multi version update
	downloadContainer         *fyne.Container // New global to track the same container
	downloadsShown            bool            // New global to track whether downloads are shown

)

func init() {
	javacheck.InitJavaCheck(owlcmsInstallDir)
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

func getInstallDir() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "owlcms")
	case "darwin":
		return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "owlcms")
	case "linux":
		return filepath.Join(os.Getenv("HOME"), ".local", "share", "owlcms")
	default:
		return "./owlcms"
	}
}

func checkJava(statusLabel *widget.Label) error {
	statusLabel.SetText("Checking for the Java language runtime.")
	statusLabel.Refresh()
	statusLabel.Show()
	stopButton.Hide()
	stopContainer.Show()
	versionContainer.Hide()
	downloadContainer.Hide()

	err := javacheck.CheckJava(statusLabel)
	if err != nil {
		statusLabel.SetText("Could not install a Java runtime.")
		statusLabel.Refresh()
	}
	return err
}

func launchOwlcms(version string, launchButton, stopButton *widget.Button) error {
	currentVersion = version // Store current version

	// Check if port 8080 is already in use
	if err := checkPort(); err == nil {
		statusLabel.Hide()
		return fmt.Errorf("OWLCMS is already running on port 8080")
	}

	statusLabel.SetText(fmt.Sprintf("Starting OWLCMS %s...", version))
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

	log.Printf("Launching OWLCMS %s (PID: %d), waiting for port 8080...\n", version, cmd.Process.Pid)
	statusLabel.SetText(fmt.Sprintf("Starting OWLCMS %s (PID: %d), please wait.  Full startup can take up to 30 seconds.", version, cmd.Process.Pid))
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
			log.Printf("OWLCMS process %d failed to start properly: %v\n", cmd.Process.Pid, err)
			statusLabel.SetText(fmt.Sprintf("OWLCMS process %d failed to start properly", cmd.Process.Pid))
			stopButton.Hide()
			stopContainer.Hide()
			launchButton.Show()
			currentProcess = nil
			downloadContainer.Show()
			versionContainer.Show()
			return
		}

		log.Printf("OWLCMS process %d is ready (port 8080 responding)\n", cmd.Process.Pid)
		statusLabel.SetText(fmt.Sprintf("OWLCMS running (PID: %d)", cmd.Process.Pid))

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
	}()

	return nil
}

func goBackToMainScreen() {
	// Implement the logic to go back to the main screen
	// This might involve setting the visibility of certain UI elements
	// or navigating to a different screen in your application.
	stopButton.Hide()
	stopContainer.Hide()
	downloadContainer.Show()
	versionContainer.Show()
}

func computeVersionScrollHeight(numVersions int) float32 {
	minHeight := 50 // minimum height
	rowHeight := 45 // approximate height per row
	return float32(minHeight + (rowHeight * min(numVersions, 4)))
}
func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	a := app.NewWithID("app.owlcms.owlcms-launcher")
	a.Settings().SetTheme(newMyTheme())
	w := a.NewWindow("OWLCMS Launcher")
	w.Resize(fyne.NewSize(600, 300)) // Larger initial window size

	// Create stop button and status label
	stopButton = widget.NewButton("Stop", nil)
	statusLabel = widget.NewLabel("")
	statusLabel.Wrapping = fyne.TextWrapWord // Allow status messages to wrap

	// Create containers
	downloadContainer = container.NewVBox()
	versionContainer = container.NewVBox()
	stopContainer = container.NewVBox(stopButton, statusLabel)

	// Initialize download titles
	updateTitle = widget.NewLabel("")
	downloadButtonTitle = widget.NewHyperlink("Click here to install additional versions.", nil) // New title for download button
	downloadButtonTitle.OnTapped = func() {
		if !downloadsShown {
			downloadsShown = true
			releaseDropdown.Show()
			prereleaseCheckbox.Show()
			downloadContainer.Refresh()
		} else {
			downloadsShown = false
			releaseDropdown.Hide()
			prereleaseCheckbox.Hide()
			downloadContainer.Refresh()
		}
	}
	singleOrMultiVersionLabel = widget.NewLabel("")

	// Configure stop button behavior
	stopButton.OnTapped = func() {
		stopProcess(currentProcess, currentVersion, stopButton, downloadContainer, versionContainer, statusLabel, w)
	}
	stopButton.Hide()
	stopContainer.Hide()

	releases, err := fetchReleases()
	if err == nil {
		allReleases = releases
	} else {
		allReleases = []string{}
	}

	// Initialize version list
	recomputeVersionList(w)

	// Create release dropdown for downloads
	releaseSelect, releaseDropdown := createReleaseDropdown(w)
	updateTitle.Hide()
	releaseDropdown.Hide() // Hide the dropdown initially

	// Create checkbox for showing prereleases
	prereleaseCheckbox = widget.NewCheck("Show Prereleases", func(checked bool) {
		showPrereleases = checked
		populateReleaseSelect(releaseSelect) // Repopulate the dropdown when the checkbox is changed
	})
	prereleaseCheckbox.Hide() // Hide the checkbox initially

	if len(allReleases) > 0 {
		downloadContainer.Objects = []fyne.CanvasObject{
			updateTitle,
			singleOrMultiVersionLabel,
			downloadButtonTitle,
			releaseDropdown,
			prereleaseCheckbox,
		}
	} else {
		downloadContainer.Objects = []fyne.CanvasObject{
			widget.NewLabel("You are not connected to the Internet. Available updates cannot be shown."),
		}
	}

	mainContent := container.NewVBox(
		// widget.NewLabelWithStyle("OWLCMS Launcher", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		stopContainer,
		versionContainer,
		downloadContainer, // Use downloadGroup here
	)

	w.SetContent(mainContent)
	w.Resize(fyne.NewSize(800, 600))

	// Show installed versions first
	w.SetContent(mainContent)
	w.Canvas().Refresh(mainContent)

	populateReleaseSelect(releaseSelect) // Populate the dropdown with the releases
	updateTitle.Show()
	releaseDropdown.Hide()
	prereleaseCheckbox.Hide() // Show the checkbox once releases are fetched
	log.Printf("Fetched %d releases\n", len(releases))
	if len(allReleases) > 0 {
		// downloadButtonContainer.Show()
	}

	// Check if a more recent version is available
	checkForNewerVersion()

	// If no version is installed, get the latest stable version
	if len(getAllInstalledVersions()) == 0 {
		for _, release := range allReleases {
			if !containsPreReleaseTag(release) {
				// Automatically download and install the latest stable version
				downloadAndInstallVersion(release, w)
				break
			}
		}
	}

	w.Canvas().Refresh(mainContent)

	w.SetCloseIntercept(func() {
		if currentProcess != nil {
			stopProcess(currentProcess, currentVersion, stopButton, downloadContainer, versionContainer, statusLabel, w)
		}
		w.Close()
	})

	log.Println("Starting OWLCMS Launcher")
	w.ShowAndRun()

}

func downloadAndInstallVersion(version string, w fyne.Window) {
	var urlPrefix string
	if containsPreReleaseTag(version) {
		urlPrefix = "https://github.com/owlcms/owlcms4-prerelease/releases/download"
	} else {
		urlPrefix = "https://github.com/owlcms/owlcms4/releases/download"
	}
	fileName := fmt.Sprintf("owlcms_%s.zip", version)
	zipURL := fmt.Sprintf("%s/%s/%s", urlPrefix, version, fileName)

	// Ensure the owlcms directory exists
	owlcmsDir := owlcmsInstallDir
	if _, err := os.Stat(owlcmsDir); os.IsNotExist(err) {
		if err := os.Mkdir(owlcmsDir, 0755); err != nil {
			dialog.ShowError(fmt.Errorf("creating owlcms directory: %w", err), w)
			return
		}
	}

	zipPath := filepath.Join(owlcmsDir, fileName)
	extractPath := filepath.Join(owlcmsDir, version)

	// Show progress dialog
	progressDialog := dialog.NewCustom(
		"Installing OWLCMS",
		"Please wait...",
		widget.NewTextGridFromString("Downloading and extracting files..."),
		w)
	progressDialog.Show()

	go func() {
		// Download the ZIP file using downloadUtils
		log.Printf("Starting download from URL: %s\n", zipURL)
		err := downloadUtils.DownloadZip(zipURL, zipPath)
		if err != nil {
			progressDialog.Hide()
			dialog.ShowError(fmt.Errorf("download failed: %w", err), w)
			return
		}

		// Extract the ZIP file to version-specific subdirectory
		log.Printf("Extracting ZIP file to: %s\n", extractPath)
		err = downloadUtils.ExtractZip(zipPath, extractPath)
		if err != nil {
			progressDialog.Hide()
			dialog.ShowError(fmt.Errorf("extraction failed: %w", err), w)
			return
		}

		// Log when extraction is done
		log.Println("Extraction completed")
		updateExplanation()

		// Hide progress dialog
		progressDialog.Hide()

		// Show success panel with installation details
		message := fmt.Sprintf(
			"Successfully installed OWLCMS version %s\n\n"+
				"Location: %s\n\n"+
				"The program files have been extracted to the above directory.",
			version, extractPath)

		dialog.ShowInformation("Installation Complete", message, w)

		// Recompute the version list
		recomputeVersionList(w)

		// Recompute the downloadTitle
		checkForNewerVersion()
	}()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func checkForNewerVersion() {
	latestInstalled := findLatestInstalled()

	// Set the single or multi version label
	updateExplanation()

	if latestInstalled != "" {
		latestStable, _ := semver.NewVersion("0.0.0")
		latestInstalledVersion, err := semver.NewVersion(latestInstalled)
		if err == nil {
			log.Printf("Latest installed version: %s\n", latestInstalledVersion)
			for _, release := range allReleases {
				releaseVersion, err := semver.NewVersion(release)
				if err == nil {
					if releaseVersion.GreaterThan(latestInstalledVersion) {
						log.Printf("Found newer version: %s\n", releaseVersion)
						if containsPreReleaseTag(release) {
							log.Printf("Newer version is a pre-release: %s\n", release)
							if containsPreReleaseTag(latestInstalled) {
								updateTitle.SetText(fmt.Sprintf("A more recent prerelease version (%s) is available", releaseVersion))
								updateTitle.TextStyle = fyne.TextStyle{Bold: true}
								updateTitle.Refresh()
								updateTitle.Show()
								return
							} else {
								log.Printf("Skipping pre-release version: %s\n", release)
							}
						} else {
							updateTitle.SetText(fmt.Sprintf("A more recent stable version (%s) is available", releaseVersion))
							updateTitle.TextStyle = fyne.TextStyle{Bold: true}
							updateTitle.Refresh()
							updateTitle.Show()
							return
						}
					}
					if (releaseVersion.GreaterThan(latestStable)) && !containsPreReleaseTag(release) {
						latestStable = releaseVersion
					}
				}
			}
			updateTitle.Show()
			downloadButtonTitle.Show()

			if containsPreReleaseTag(latestInstalled) {
				updateTitle.SetText(fmt.Sprintf("The latest installed version is a pre-release; the latest stable version is %s", latestStable))
			} else {
				updateTitle.SetText("The latest stable version is installed.")
			}
			updateTitle.TextStyle = fyne.TextStyle{Bold: true}
			updateTitle.Refresh()

			downloadButtonTitle.Refresh()
			if releaseDropdown != nil {
				releaseDropdown.Hide()
			}
			if prereleaseCheckbox != nil {
				prereleaseCheckbox.Hide()
			}
			updateTitle.Show()
			downloadButtonTitle.Show()
			if downloadContainer != nil {
				downloadContainer.Refresh()
			}
		}
	} else {
		updateTitle.SetText("No version installed. Select a version to download below.")
		updateTitle.TextStyle = fyne.TextStyle{Bold: true}
		updateExplanation()
		updateTitle.Refresh()
		updateTitle.Show()
		if downloadContainer != nil {
			downloadContainer.Refresh()
		}
	}
}

func updateExplanation() {
	if len(allReleases) == 0 {
		downloadContainer.Objects = []fyne.CanvasObject{
			widget.NewLabel("You are not connected to the Internet. Available updates cannot be shown."),
		}
		downloadContainer.Show()
		downloadContainer.Refresh()
		return
	}
	log.Printf("len(allReleases) = %d\n", len(allReleases))
	x := getAllInstalledVersions()
	log.Printf("Updating explanation %d\n", len(x))
	if len(x) == 0 {
		singleOrMultiVersionLabel.Hide()
	} else if len(x) == 1 {
		latestStable, stableErr := getMostRecentStableRelease()
		latestPrerelease, preErr := getMostRecentPrerelease()

		// Remove the label from the container first
		downloadContainer.Remove(singleOrMultiVersionLabel)

		if containsPreReleaseTag(x[0]) {
			if preErr == nil && x[0] == latestPrerelease {
				// It's the latest prerelease; do not re-insert the label
			} else {
				// Not the latest; re-insert singleOrMultiVersionLabel as second item
				downloadContainer.Objects = append(downloadContainer.Objects[:1], append([]fyne.CanvasObject{singleOrMultiVersionLabel}, downloadContainer.Objects[1:]...)...)
				singleOrMultiVersionLabel.SetText("Use the Update button above to install the latest version. The current database will be copied to the new version, as well as local changes made to the configuration since the previous installation.")
			}
		} else {
			if stableErr == nil && x[0] == latestStable {
				// It's the latest stable; do not re-insert the label
			} else {
				downloadContainer.Objects = append(downloadContainer.Objects[:1], append([]fyne.CanvasObject{singleOrMultiVersionLabel}, downloadContainer.Objects[1:]...)...)
				singleOrMultiVersionLabel.SetText("Use the Update button above to install the latest version. The current database will be copied to the new version, as well as local changes made to the configuration since the previous installation.")
			}
		}
	} else {
		singleOrMultiVersionLabel.SetText("You have several versions installed. Use the Import button if you wish to copy the database and local configuration changes from a previous version.")
	}
	singleOrMultiVersionLabel.Wrapping = fyne.TextWrapWord
	singleOrMultiVersionLabel.Show()
	singleOrMultiVersionLabel.Refresh()
}
