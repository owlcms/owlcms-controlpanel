package main

import (
	"fmt"
	"image/color"
	"log"
	"os"
	"os/exec"
	"path/filepath"

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
	switch downloadUtils.GetGoos() {
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
		return err
	}

	statusLabel.Hide() // Hide the status label if Java check is successful
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

func removeAllVersions() {
	entries, err := os.ReadDir(owlcmsInstallDir)
	if err != nil {
		log.Printf("Failed to read owlcms directory: %v\n", err)
		dialog.ShowError(fmt.Errorf("failed to read owlcms directory: %w", err), fyne.CurrentApp().Driver().AllWindows()[0])
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			_, err := semver.NewVersion(entry.Name())
			if err == nil {
				dirPath := filepath.Join(owlcmsInstallDir, entry.Name())
				if err := os.RemoveAll(dirPath); err != nil {
					log.Printf("Failed to remove directory %s: %v\n", dirPath, err)
					dialog.ShowError(fmt.Errorf("failed to remove directory %s: %w", dirPath, err), fyne.CurrentApp().Driver().AllWindows()[0])
					return
				}
			}
		}
	}

	log.Println("All versions removed successfully")
	dialog.ShowInformation("Success", "All versions removed successfully", fyne.CurrentApp().Driver().AllWindows()[0])
	getAllInstalledVersions()
	updateTitle.SetText("All Versions Removed.")
	downloadButtonTitle.SetText("Click here to install a version.")
	downloadButtonTitle.Refresh()
	updateTitle.Refresh()
	recomputeVersionList(fyne.CurrentApp().Driver().AllWindows()[0])
}

func uninstallAll() {
	dialog.ShowConfirm("Confirm Uninstall", "This will remove all the data and configurations currently stored and exit the program.\nIf you proceed, this cannot be undone. Restarting the program will create new data.", func(confirm bool) {
		if confirm {
			err := os.RemoveAll(owlcmsInstallDir)
			if err != nil {
				log.Printf("Failed to remove all data: %v\n", err)
				dialog.ShowError(fmt.Errorf("failed to remove all data: %w", err), fyne.CurrentApp().Driver().AllWindows()[0])
			} else {
				log.Println("All data removed successfully")
				dialog.ShowInformation("Success", "All data removed successfully", fyne.CurrentApp().Driver().AllWindows()[0])
				fyne.CurrentApp().Quit()
			}
		}
	}, fyne.CurrentApp().Driver().AllWindows()[0])
}

func removeJava() {
	javaDir := filepath.Join(owlcmsInstallDir, "java17")
	err := os.RemoveAll(javaDir)
	if err != nil {
		log.Printf("Failed to remove Java: %v\n", err)
		dialog.ShowError(fmt.Errorf("failed to remove Java: %w", err), fyne.CurrentApp().Driver().AllWindows()[0])
	} else {
		log.Println("Java removed successfully")
		dialog.ShowInformation("Success", "Java removed successfully", fyne.CurrentApp().Driver().AllWindows()[0])
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	a := app.NewWithID("app.owlcms.owlcms-launcher")
	a.Settings().SetTheme(newMyTheme())
	w := a.NewWindow("OWLCMS Control Panel")
	w.Resize(fyne.NewSize(800, 400)) // Larger initial window size

	// Create stop button and status label
	stopButton = widget.NewButton("Stop", nil)
	stopButton.Importance = widget.HighImportance // Make the stop button important
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

	mainContent := container.NewVBox(
		// widget.NewLabelWithStyle("OWLCMS Launcher", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		stopContainer,
		versionContainer,
		downloadContainer, // Use downloadGroup here
	)

	var javaAvailable bool
	javaLoc, err := javacheck.FindLocalJava()
	javaAvailable = err == nil && javaLoc != ""

	// Check for internet connection before anything else
	internetAvailable := CheckForInternet() //&& false
	log.Printf("javaloc %s err %v javaAvailable %t internetAvailable %t", javaLoc, err, javaAvailable, internetAvailable)
	if internetAvailable && !javaAvailable {
		// Check for Java before anything else
		if err := checkJava(statusLabel); err != nil {
			dialog.ShowError(fmt.Errorf("failed to fetch Java: %w", err), w)
		}
	}

	var releases []string
	if internetAvailable {
		releases, err = fetchReleases()
		if err == nil {
			allReleases = releases
		}
	} else {
		allReleases = []string{}
	}

	numVersions := len(getAllInstalledVersions())
	if numVersions == 0 && !internetAvailable {
		w.Resize(fyne.NewSize(800, 400))
		d := dialog.NewInformation("No Internet Connection", "You must be connected to the internet to fetch a version of the program.\nPlease connect and restart the program", w)
		d.Resize(fyne.NewSize(400, 200))
		d.SetDismissText("Exit")
		d.Show()
		d.SetOnClosed(func() {
			a.Driver().Quit()
		})
		w.ShowAndRun()
		return
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

	// Create menu items
	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("Remove All Versions", func() {
			removeAllVersions()
		}),
		fyne.NewMenuItem("Remove Java", func() {
			removeJava()
		}),
		fyne.NewMenuItem("Remove All Stored Data and Configurations", func() {
			uninstallAll()
		}),
		fyne.NewMenuItem("Open Installation Directory", func() {
			if err := openFileExplorer(owlcmsInstallDir); err != nil {
				dialog.ShowError(fmt.Errorf("failed to open installation directory: %w", err), w)
			}
		}),
	)
	killMenu := fyne.NewMenu("Processes",
		fyne.NewMenuItem("Kill Already Running Process", func() {
			if err := killLockingProcess(); err != nil {
				dialog.ShowError(fmt.Errorf("failed to kill already running process: %w", err), w)
			} else {
				dialog.ShowInformation("Success", "Successfully killed the already running process", w)
			}
		}),
	)
	menu := fyne.NewMainMenu(fileMenu, killMenu)
	w.SetMainMenu(menu)
	mainContent.Resize(fyne.NewSize(800, 400))
	w.SetContent(mainContent)
	w.Resize(fyne.NewSize(800, 400))

	populateReleaseSelect(releaseSelect) // Populate the dropdown with the releases
	updateTitle.Show()
	releaseDropdown.Hide()
	prereleaseCheckbox.Hide() // Show the checkbox once releases are fetched
	log.Printf("Fetched %d releases\n", len(releases))

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

	// Check if a more recent version is available
	checkForNewerVersion()
	downloadContainer.Refresh()
	downloadContainer.Show()
	mainContent.Refresh()

	w.SetContent(mainContent)
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
		if err := os.MkdirAll(owlcmsDir, 0755); err != nil {
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
		err := downloadUtils.DownloadArchive(zipURL, zipPath)
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
