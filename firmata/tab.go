package firmata

import (
	"fmt"
	"image/color"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"

	customdialog "owlcms-launcher/firmata/dialog"
	"owlcms-launcher/firmata/javacheck"
	"owlcms-launcher/shared"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/Masterminds/semver/v3"
)

var (
	installDir = getInstallDir()
	// TEMPORARY TEST FLAG: when true, treat Firmata as not installed.
	// Keep variable for testing; default to false to use real detection.
	forceUninstalledFirmata   = false
	currentProcess            *exec.Cmd
	currentVersion            string // Add to track current version
	statusLabel               *widget.Label
	stopButton                *widget.Button
	versionContainer          *fyne.Container
	stopContainer             *fyne.Container
	singleOrMultiVersionLabel *widget.Label     // New label for single or multi version update
	downloadContainer         *fyne.Container   // New global to track the same container
	downloadsShown            bool              // New global to track whether downloads are shown
	urlLink                   *widget.Hyperlink // Add this new variable
	mainWindow                fyne.Window       // Reference to the main window
)

func initMain() {
	javacheck.InitJavaCheck(installDir, GetTemurinVersion)
}

type myTheme struct {
	fyne.Theme
}

func newMyTheme() *myTheme {
	return &myTheme{Theme: theme.LightTheme()}
}

func (m myTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameSuccess:
		// Much darker green color
		return color.RGBA{R: 15, G: 80, B: 15, A: 255}
	case theme.ColorNameBackground:
		return color.White
	case theme.ColorNameForeground:
		return color.Black
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 40}
	default:
		return m.Theme.Color(name, variant)
	}
}

func getInstallDir() string {
	switch shared.GetGoos() {
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "firmata")
	case "darwin":
		return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "firmata")
	case "linux":
		return filepath.Join(os.Getenv("HOME"), ".local", "share", "firmata")
	default:
		return "./firmata"
	}
}

// GetInstallDir returns the installation directory used by the firmata package
func GetInstallDir() string {
	return getInstallDir()
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
	setFirmataTabMode(fyne.CurrentApp().Driver().AllWindows()[0])
}

func computeVersionScrollHeight(numVersions int) float32 {
	minHeight := 140 // minimum height to provide adequate vertical space
	rowHeight := 50  // approximate height per row
	return float32(minHeight + (rowHeight * min(numVersions, 4)))
}

func removeAllVersions() {
	entries, err := os.ReadDir(installDir)
	if err != nil {
		log.Printf("Failed to read firmata directory: %v\n", err)
		dialog.ShowError(fmt.Errorf("failed to read firmata directory: %w", err), fyne.CurrentApp().Driver().AllWindows()[0])
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			_, err := semver.NewVersion(entry.Name())
			if err == nil {
				dirPath := filepath.Join(installDir, entry.Name())
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
	updateTitle.ParseMarkdown("All Versions Removed.")
	downloadButtonTitle.SetText("Click here to install a version.")
	downloadButtonTitle.Refresh()
	updateTitle.Refresh()
	recomputeVersionList(mainWindow)
}

func uninstallAll() {
	dialog.ShowConfirm("Confirm Uninstall", "This will remove all the data and configurations currently stored.\nIf you proceed, this cannot be undone. Restarting the program will create new data.", func(confirm bool) {
		if confirm {
			err := os.RemoveAll(installDir)
			if err != nil {
				log.Printf("Failed to remove all data: %v\n", err)
				dialog.ShowError(fmt.Errorf("failed to remove all data: %w", err), fyne.CurrentApp().Driver().AllWindows()[0])
			} else {
				log.Println("All data removed successfully")
				dialog.ShowInformation("Success", "All data removed successfully", fyne.CurrentApp().Driver().AllWindows()[0])
				// Do not quit the control panel; refresh UI to show uninstalled explanation
				recomputeVersionList(mainWindow)
			}
		}
	}, fyne.CurrentApp().Driver().AllWindows()[0])
}

func removeJava() {
	javaDir := filepath.Join(installDir, "java17")
	err := os.RemoveAll(javaDir)
	if err != nil {
		log.Printf("Failed to remove Java: %v\n", err)
		dialog.ShowError(fmt.Errorf("failed to remove Java: %w", err), fyne.CurrentApp().Driver().AllWindows()[0])
	} else {
		log.Println("Java removed successfully")
		dialog.ShowInformation("Success", "Java removed successfully", fyne.CurrentApp().Driver().AllWindows()[0])
	}
}

// IsRunning returns true if Firmata is currently running
func IsRunning() bool {
	return currentProcess != nil
}

// StopRunningProcess stops the running Firmata process
func StopRunningProcess(w fyne.Window) {
	if currentProcess != nil && currentProcess.Process != nil {
		log.Println("Stopping Firmata process")
		stopProcess(currentProcess, currentVersion, stopButton, downloadContainer, versionContainer, statusLabel, w)
	}
}

// HandleSignalCleanup handles cleanup when the application receives a signal
func HandleSignalCleanup() {
	if currentProcess != nil && currentProcess.Process != nil {
		pid := currentProcess.Process.Pid
		log.Printf("Forcefully stopping Firmata (PID: %d)...\n", pid)
		killedByUs = true
		// Use direct kill for fast cleanup
		if err := currentProcess.Process.Kill(); err != nil {
			log.Printf("Failed to kill Firmata process %d: %v\n", pid, err)
		} else {
			log.Printf("Firmata process %d killed\n", pid)
		}
		currentProcess = nil
	}
}

// CreateTab creates and returns the Firmata tab content
// This should be called from the main application after the window is created
func CreateTab(w fyne.Window) *fyne.Container {
	// Initialize the firmata-specific components
	initMain()
	initConfig()

	// Store main window reference
	mainWindow = w

	log.Println("Creating Firmata tab content")

	// Create stop button and status label
	stopButton = widget.NewButtonWithIcon("Stop", theme.CancelIcon(), nil)
	stopButton.Importance = widget.DangerImportance // Dark red for stop action
	statusLabel = widget.NewLabel("")
	statusLabel.Wrapping = fyne.TextWrapWord // Allow status messages to wrap

	// Create containers
	downloadContainer = container.NewVBox()
	versionContainer = container.NewStack() // Use Stack so it expands in the center (replaces deprecated NewMax)

	// Create URL hyperlink
	urlLink = widget.NewHyperlink("", nil)
	urlLink.Hide()

	stopContainer = container.NewVBox(widget.NewSeparator(), stopButton, statusLabel, urlLink)

	// Initialize download titles
	updateTitle = widget.NewRichTextFromMarkdown("")
	updateTitleContainer = container.NewHBox(updateTitle)
	downloadButtonTitle = widget.NewHyperlink("Click here to install additional versions.", nil)
	downloadButtonTitle.OnTapped = func() {
		if !downloadsShown {
			ShowDownloadables()
		} else {
			HideDownloadables()
		}
	}
	singleOrMultiVersionLabel = widget.NewLabel("")

	// Configure stop button behavior (confirm before stopping)
	stopButton.OnTapped = func() {
		log.Println("Stop button tapped")
		confirmDialog := dialog.NewConfirm(
			"Confirm Stop",
			"Stop the running Firmata process?",
			func(confirm bool) {
				if confirm {
					stopProcess(currentProcess, currentVersion, stopButton, downloadContainer, versionContainer, statusLabel, w)
				}
			},
			w,
		)
		confirmDialog.SetConfirmText("Stop")
		confirmDialog.SetDismissText("Cancel")
		confirmDialog.Show()
	}
	stopButton.Hide()
	stopContainer.Hide()

	// Set a fixed height for the bottom container to match OWLCMS tab
	downloadContainer.Resize(fyne.NewSize(800, 180))

	// Create menu bar
	menuBar := createMenuBar(w)

	mainContent := container.NewBorder(
		container.NewVBox(menuBar, stopContainer), // Top (menu bar and stop container)
		downloadContainer,                         // Bottom
		nil,                                       // Left
		nil,                                       // Right
		versionContainer,                          // Center (now a NewStack, expands to fill space)
	)
	statusLabel.SetText("Checking installation status...")
	statusLabel.Refresh()
	statusLabel.Show()
	stopContainer.Show()

	// If Firmata install directory does not exist, reset tab to explanation mode
	if forceUninstalledFirmata || func() bool { _, err := os.Stat(installDir); return os.IsNotExist(err) }() {
		resetToExplainMode(w)
		return mainContent
	} else {
		// Start initialization in a goroutine
		go initializeFirmataTab(w)
	}

	return mainContent
}

// createMenuBar creates the menu bar with File and Processes menus
func createMenuBar(w fyne.Window) *fyne.Container {
	// Create the File menu button with popup
	fileMenuItems := []*fyne.MenuItem{
		fyne.NewMenuItem("Open Firmata Installation Directory", func() {
			if err := shared.OpenFileExplorer(installDir); err != nil {
				dialog.ShowError(fmt.Errorf("failed to open installation directory: %w", err), w)
			}
		}),
		fyne.NewMenuItem("Refresh Available Versions", func() {
			refreshAvailableVersions(w)
		}),
		fyne.NewMenuItemSeparator(),
		// Commented out: remove all versions via Files menu (use Uninstall instead)
		// fyne.NewMenuItem("Remove All Firmata Versions", func() {
		// 	removeAllVersions()
		// }),
		// Commented out: remove bundled Java via Files menu
		// fyne.NewMenuItem("Remove Firmata Java", func() {
		// 	removeJava()
		// }),
		fyne.NewMenuItem("Uninstall Firmata", func() {
			uninstallAll()
		}),
	}
	fileMenu := shared.CreateMenuButton("Files", fileMenuItems)

	// Create the Processes menu button with popup
	processMenuItems := []*fyne.MenuItem{
		fyne.NewMenuItem("Kill Already Running Process", func() {
			if err := killLockingProcess(); err != nil {
				dialog.ShowError(fmt.Errorf("failed to kill already running process: %w", err), w)
			} else {
				dialog.ShowInformation("Success", "Successfully killed the already running process", w)
			}
		}),
	}
	processMenu := shared.CreateMenuButton("Processes", processMenuItems)

	// Add small vertical padding
	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(fyne.NewSize(1, 5))

	return container.NewVBox(
		spacer,
		container.NewHBox(fileMenu, processMenu),
	)
}

func refreshAvailableVersions(w fyne.Window) {
	go func() {
		releases, err := fetchReleases()
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to refresh available versions: %w", err), w)
			return
		}
		allReleases = releases

		// If the dropdown exists, repopulate it.
		if releaseDropdown != nil {
			for _, obj := range releaseDropdown.Objects {
				if selectWidget, ok := obj.(*widget.Select); ok {
					populateReleaseSelect(selectWidget)
					break
				}
			}
		}

		recomputeVersionList(w)
		checkForNewerVersion()
		if downloadContainer != nil {
			downloadContainer.Refresh()
		}
	}()
}

// initializeFirmataTab handles the async initialization of the Firmata tab
func initializeFirmataTab(w fyne.Window) {
	// Set the appropriate mode based on installed versions
	if len(getAllInstalledVersions()) == 0 {
		setFirmataTabModeUninstalled(w)
	} else {
		setFirmataTabModeInstalled(w)
	}
	log.Println("Firmata tab setup done.")
}

// setFirmataTabModeUninstalled shows the install prompt for when no versions are installed.
func setFirmataTabModeUninstalled(w fyne.Window) {
	resetToExplainMode(w)
	log.Printf("UI Mode: Uninstalled (0 versions)")
}

// setFirmataTabModeInstalled shows the version list and download section.
func setFirmataTabModeInstalled(w fyne.Window) {
	// Fetch releases first so update buttons can be computed
	setupReleaseDropdown(w)
	// Now recompute list with release info available
	recomputeVersionList(w)
	checkForNewerVersion()

	if stopButton != nil {
		stopButton.Hide()
	}
	if stopContainer != nil {
		stopContainer.Hide()
	}
	if statusLabel != nil {
		statusLabel.Hide()
	}
	if versionContainer != nil {
		versionContainer.Show()
		versionContainer.Refresh()
	}
	if downloadContainer != nil {
		downloadContainer.Show()
		downloadContainer.Refresh()
	}

	log.Printf("UI Mode: Installed (versions=%d; list+download visible)", len(getAllInstalledVersions()))
}

// setFirmataTabMode is the single switch deciding which mode to show.
func setFirmataTabMode(w fyne.Window) {
	if len(getAllInstalledVersions()) == 0 {
		setFirmataTabModeUninstalled(w)
		return
	}
	setFirmataTabModeInstalled(w)
}

// HideDownloadables hides the download dropdown and prerelease checkbox
func HideDownloadables() {
	downloadsShown = false
	if releaseDropdown != nil {
		releaseDropdown.Hide()
	}
	if prereleaseCheckbox != nil {
		prereleaseCheckbox.Hide()
	}
	if downloadButtonTitle != nil {
		downloadButtonTitle.Show()
	}
	if downloadContainer != nil {
		downloadContainer.Refresh()
	}
}

// ShowDownloadables shows the download dropdown and prerelease checkbox
func ShowDownloadables() {
	downloadsShown = true
	if len(allReleases) == 0 {
		if downloadContainer != nil {
			downloadContainer.Objects = []fyne.CanvasObject{
				widget.NewLabel("You are not connected to the Internet. Available updates cannot be shown."),
			}
			downloadContainer.Refresh()
		}
		return
	}
	if releaseDropdown != nil {
		releaseDropdown.Show()
	}
	if prereleaseCheckbox != nil {
		prereleaseCheckbox.Show()
	}
	if downloadButtonTitle != nil {
		downloadButtonTitle.Hide()
	}
	if downloadContainer != nil {
		downloadContainer.Refresh()
	}
}

func downloadAndInstallVersion(version string, w fyne.Window) {
	var urlPrefix string
	if containsPreReleaseTag(version) {
		urlPrefix = "https://github.com/jflamy/owlcms-firmata/releases/download"
	} else {
		urlPrefix = "https://github.com/jflamy/owlcms-firmata/releases/download"
	}
	fileName := "owlcms-firmata.jar"
	zipURL := fmt.Sprintf("%s/%s/%s", urlPrefix, version, fileName)

	// Ensure the firmata directory exists
	owlcmsDir := installDir
	if _, err := os.Stat(owlcmsDir); os.IsNotExist(err) {
		if err := shared.EnsureDir0755(owlcmsDir); err != nil {
			dialog.ShowError(fmt.Errorf("creating firmata directory: %w", err), w)
			return
		}
	}

	// Show progress dialog with progress bar
	cancel := make(chan bool)
	progressDialog, progressBar := customdialog.NewDownloadDialog(
		"Installing owlcms-firmata",
		w,
		cancel)
	progressDialog.Show()

	go func() {
		extractPath := filepath.Join(owlcmsDir, version)
		if err := shared.EnsureDir0755(extractPath); err != nil {
			progressDialog.Hide()
			dialog.ShowError(fmt.Errorf("creating firmata version directory: %w", err), w)
			return
		}
		extractPath = filepath.Join(extractPath, fileName)

		// Download the file using downloadutils with progress tracking
		log.Printf("Starting download from URL: %s\n", zipURL)
		progressCallback := func(downloaded, total int64) {
			if total > 0 {
				percentage := float64(downloaded) / float64(total)
				progressBar.SetValue(percentage)
			}
		}
		err := shared.DownloadArchive(zipURL, extractPath, progressCallback, cancel)
		if err != nil {
			progressDialog.Hide()
			if err.Error() == "download cancelled" {
				return
			}
			dialog.ShowError(fmt.Errorf("download failed: %w", err), w)
			return
		}

		// Log when extraction is done
		log.Println("Extraction completed")

		// Hide progress dialog
		progressDialog.Hide()

		// Show success panel with installation details
		message := fmt.Sprintf(
			"Successfully installed owlcms-firmata version %s\n\n"+
				"Location: %s\n\n"+
				"The program files have been extracted to the above directory.",
			version, extractPath)

		dialog.ShowInformation("Installation Complete", message, w)
		HideDownloadables()

		// Refresh the tab mode to show the download section properly
		setFirmataTabMode(w)
	}()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func checkForNewerVersion() {
	latestInstalled = findLatestInstalled()
	updateExplanation()

	if latestInstalled != "" {
		latestInstalledVersion, err := semver.NewVersion(latestInstalled)
		if err == nil {
			log.Printf("Latest installed version: %s\n", latestInstalledVersion)

			// Check for newer versions (both stable and prerelease)
			for _, release := range allReleases {
				releaseVersion, err := semver.NewVersion(release)
				if err == nil && releaseVersion.GreaterThan(latestInstalledVersion) {
					log.Printf("Found newer version: %s\n", releaseVersion)
					releaseURL := fmt.Sprintf("https://github.com/jflamy/owlcms-firmata/releases/tag/%s", releaseVersion)
					versionToInstall := extractSemverTag(release)

					var versionType string
					if containsPreReleaseTag(release) {
						versionType = "prerelease"
						// Only offer prerelease if one is already installed
						if !containsPreReleaseTag(latestInstalled) {
							continue // Skip prerelease if user has stable installed
						}
					} else {
						versionType = "stable"
					}

					// Create hyperlinks for Release Notes and install option
					parsedURL, _ := url.Parse(releaseURL)
					releaseNotesLink := widget.NewHyperlink("Release Notes", parsedURL)
					// Ensure hyperlink visible for prerelease/stable announcement
					releaseNotesLink.Show()
					installLink := widget.NewHyperlink("Install as additional version", nil)
					installLink.OnTapped = func() {
						if versionToInstall == "" {
							return
						}
						dialog.ShowConfirm(
							"Confirm Download",
							fmt.Sprintf("Do you want to download and install owlcms-firmata version %s?", versionToInstall),
							func(ok bool) {
								if !ok {
									return
								}
								downloadAndInstallVersion(versionToInstall, mainWindow)
							},
							mainWindow,
						)
					}

					messageBox := shared.CreateUpdateNotification(versionType, releaseVersion.String(), installLink, releaseNotesLink)
					updateTitleContainer.Objects = []fyne.CanvasObject{messageBox}
					updateTitleContainer.Refresh()
					updateTitleContainer.Show()
					return
				}
			}

			// If we get here, no newer version was found
			releaseURL := fmt.Sprintf("https://github.com/jflamy/owlcms-firmata/releases/tag/%s", latestInstalled)
			parsedURL, _ := url.Parse(releaseURL)
			releaseNotesLink := widget.NewHyperlink("Release Notes", parsedURL)
			// Ensure hyperlink visible for installed prerelease/stable
			releaseNotesLink.Show()
			// Log what we think is installed for debugging
			log.Printf("Firmata:updateTitle - latestInstalled=%q installedVersions=%v", latestInstalled, getAllInstalledVersions())
			messageBox := container.NewHBox(
				widget.NewLabel(fmt.Sprintf("The latest %s version %s is installed.", func() string {
					if containsPreReleaseTag(latestInstalled) {
						return "prerelease"
					}
					return "stable"
				}(), latestInstalled)),
				releaseNotesLink,
			)
			updateTitleContainer.Objects = []fyne.CanvasObject{messageBox}
			updateTitleContainer.Refresh()
			updateTitleContainer.Show()
			downloadButtonTitle.Show()
			if releaseDropdown != nil {
				releaseDropdown.Hide()
			}
			if downloadContainer != nil {
				downloadContainer.Refresh()
			}
		}
	} else {
		messageBox := container.NewHBox(
			widget.NewLabel("No version is installed."),
		)
		updateTitleContainer.Objects = []fyne.CanvasObject{messageBox}
		updateTitleContainer.Refresh()
		updateTitleContainer.Show()
	}
}
