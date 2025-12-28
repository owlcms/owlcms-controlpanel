package owlcms

import (
	"fmt"
	"image/color"
	"log"
	"net/url"
	"os"
	"path/filepath"

	"owlcms-launcher/owlcms/installutils"
	"owlcms-launcher/owlcms/javacheck"
	"owlcms-launcher/shared"
	"owlcms-launcher/tracker"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/Masterminds/semver/v3"
)

var (
	statusLabel               *widget.Label
	stopButton                *widget.Button
	versionContainer          *fyne.Container
	stopContainer             *fyne.Container
	singleOrMultiVersionLabel *widget.Label
	downloadContainer         *fyne.Container
	downloadsShown            bool
	urlLink                   *widget.Hyperlink
	appDirLink                *widget.Hyperlink
	mainWindow                fyne.Window
	mainApp                   fyne.App
)

// CreateTab creates and returns the OWLCMS tab content
// This should be called from the main application after the window is created
func CreateTab(w fyne.Window, app fyne.App) *fyne.Container {
	// Store main window reference
	mainWindow = w
	mainApp = app

	log.Println("Creating OWLCMS tab content")

	// Initialize environment early
	if err := InitEnv(); err != nil {
		log.Printf("Failed to initialize environment: %v", err)
		dialog.ShowError(fmt.Errorf("failed to initialize environment: %w", err), w)
	}

	// Check for updates immediately after showing the window
	go CheckForUpdates(w, false)

	// Create stop button and status label
	stopButton = widget.NewButton("Stop", nil)
	stopButton.Importance = widget.HighImportance
	statusLabel = widget.NewLabel("Initializing OWLCMS Control Panel...")
	statusLabel.Wrapping = fyne.TextWrapWord

	// Create URL hyperlink
	urlLink = widget.NewHyperlink("", nil)
	urlLink.Hide()

	// Create application directory hyperlink
	appDirLink = widget.NewHyperlink("", nil)
	appDirLink.Hide()

	// Initialize containers
	downloadContainer = container.NewVBox()
	versionContainer = container.NewStack()
	stopContainer = container.NewVBox(widget.NewSeparator(), stopButton, statusLabel, urlLink, appDirLink)

	// Initialize download titles
	updateTitle = widget.NewRichTextFromMarkdown("")

	// Initialize release notes hyperlink
	releaseNotesLink = widget.NewHyperlink("Release Notes", nil)
	releaseNotesLink.Hide()

	// Initialize install hyperlink for available version
	installAvailableLink = widget.NewHyperlink("install as new version", nil)
	installAvailableLink.OnTapped = func() {
		if availableVersion != "" {
			confirmAndDownloadVersion(availableVersion, mainWindow)
		}
	}
	installAvailableLink.Hide()

	// Create container to hold update title and hyperlinks
	updateTitleContainer = container.NewHBox(updateTitle, releaseNotesLink, installAvailableLink)

	downloadButtonTitle = widget.NewHyperlink("Click here to install additional versions.", nil)
	downloadButtonTitle.OnTapped = func() {
		if !downloadsShown {
			ShowDownloadables()
		} else {
			HideDownloadables()
		}
	}
	singleOrMultiVersionLabel = widget.NewLabel("")

	// Configure stop button behavior
	stopButton.OnTapped = func() {
		log.Println("Stop button tapped")
		stopProcess(currentProcess, currentVersion, stopButton, downloadContainer, versionContainer, statusLabel, w)
	}
	stopButton.Hide()
	stopContainer.Hide()

	// Set a fixed height for the bottom container
	downloadContainer.Resize(fyne.NewSize(800, 180))

	// Create menu bar
	menuBar := createMenuBar(w)

	// Add small spacer under the menu bar to increase top area
	topSpacer := canvas.NewRectangle(color.Transparent)
	topSpacer.SetMinSize(fyne.NewSize(1, 8))

	mainContent := container.NewBorder(
		container.NewVBox(menuBar, topSpacer, stopContainer), // Top (menu bar, spacer and stop container)
		downloadContainer, // Bottom (fixed height, always visible)
		nil,               // Left
		nil,               // Right
		versionContainer,  // Center (expands to fill space)
	)

	// If OWLCMS install directory does not exist, show explanation and Install button
	if _, err := os.Stat(installDir); os.IsNotExist(err) {
		shared.ShowUninstalledTabContent(versionContainer, "asset/owlcms.md", func() {
			mostRecent, err := getMostRecentStableRelease()
			log.Printf("OWLCMS Install button: getMostRecentStableRelease -> mostRecent=%q err=%v", mostRecent, err)
			if err == nil && mostRecent != "" {
				log.Printf("OWLCMS Install: starting downloadReleaseWithProgress(%s)", mostRecent)
				downloadReleaseWithProgress(mostRecent, w, false)
			} else {
				log.Println("OWLCMS Install: no latest stable found, showing download UI")
				ShowDownloadables()
			}
		}, func() { initializeOwlcmsTab(w, app) })
	} else {
		// Start initialization in a goroutine
		go initializeOwlcmsTab(w, app)
	}

	return mainContent
}

// createMenuBar creates the menu bar with File, Processes, and Options menus
func createMenuBar(w fyne.Window) *fyne.Container {
	// Create the File menu button with popup
	fileMenuItems := []*fyne.MenuItem{
		fyne.NewMenuItem("Open OWLCMS Installation Directory", func() {
			if err := shared.OpenFileExplorer(installDir); err != nil {
				dialog.ShowError(fmt.Errorf("failed to open installation directory: %w", err), w)
			}
		}),
		fyne.NewMenuItem("Install OWLCMS version from ZIP", func() {
			selectLocalZip(w, func(path string, err error) {
				if err != nil {
					dialog.ShowError(fmt.Errorf("file selection failed: %w", err), w)
					return
				}
				if path == "" {
					return
				}
				installutils.ProcessLocalZipFile(path, w, installDir, copyFile, updateExplanation, recomputeVersionList, checkForNewerVersion)
			})
		}),
		fyne.NewMenuItem("Save installed OWLCMS version as ZIP", func() {
			installutils.ZipCurrentSetup(w, installDir, getAllInstalledVersions, selectSaveZip)
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Remove All OWLCMS Versions", func() {
			removeAllVersions()
		}),
		fyne.NewMenuItem("Remove OWLCMS Java", func() {
			removeJava()
		}),
		fyne.NewMenuItem("Remove All OWLCMS Stored Data and Configurations", func() {
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

	// Create the Options menu button with popup
	// Set menu item text based on current state
	var menuItemText string
	if GetTrackerConnectionEnabled() {
		menuItemText = "\u2717 Stop connecting to local tracker"
	} else {
		menuItemText = "\u2713 Automatically connect to local tracker"
	}

	optionsMenuItems := []*fyne.MenuItem{
		fyne.NewMenuItem(menuItemText, func() {
			// Get the tracker port
			trackerPort := tracker.GetPort()
			trackerURL := fmt.Sprintf("ws://127.0.0.1:%s/ws", trackerPort)

			// Toggle the connection
			if GetTrackerConnectionEnabled() {
				// Disable
				if err := DeleteProperty("OWLCMS_VIDEODATA"); err != nil {
					dialog.ShowError(fmt.Errorf("failed to disable tracker connection: %w", err), w)
				} else {
					dialog.ShowInformation("Tracker Connection", "Tracker connection disabled. Restart OWLCMS for changes to take effect.", w)
				}
			} else {
				// Enable
				if err := SaveProperty("OWLCMS_VIDEODATA", trackerURL); err != nil {
					dialog.ShowError(fmt.Errorf("failed to enable tracker connection: %w", err), w)
				} else {
					dialog.ShowInformation("Tracker Connection", fmt.Sprintf("Tracker connection enabled on port %s. OWLCMS will connect to Tracker once it is started. Restart OWLCMS for changes to take effect.", trackerPort), w)
				}
			}
		}),
	}

	optionsMenu := shared.CreateMenuButton("Options", optionsMenuItems)

	// Add small vertical padding
	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(fyne.NewSize(1, 5))

	return container.NewVBox(
		spacer,
		container.NewHBox(fileMenu, processMenu, optionsMenu),
	)
}

// initializeOwlcmsTab handles the async initialization of the OWLCMS tab
func initializeOwlcmsTab(w fyne.Window, app fyne.App) {
	// Check Java first
	statusLabel.SetText("Checking for Java runtime...")
	javaLoc, err := javacheck.FindLocalJava()
	internetAvailable := CheckForInternet()

	// Update status
	if err != nil || javaLoc == "" {
		if !internetAvailable {
			content := container.New(layout.NewCenterLayout(),
				widget.NewLabel("Java is not installed and there is no internet connection.\nPlease connect and restart the program."))

			dlg := dialog.NewCustom("No Internet Connection", "Exit", content, w)
			dlg.SetOnClosed(func() {
				app.Quit()
			})
			dlg.Show()
			return
		}

		statusLabel.SetText("Java runtime not found. Starting download...")
		if err := checkJava(statusLabel); err != nil {
			dialog.ShowError(fmt.Errorf("failed to fetch Java: %w", err), w)
			return
		}
	} else {
		statusLabel.SetText("Java runtime found. Loading application...")
	}

	// Get releases if internet is available
	statusLabel.SetText("Checking for available OWLCMS versions...")
	if internetAvailable {
		releases, err := fetchReleases()
		if err == nil {
			allReleases = releases
		}
	} else {
		allReleases = []string{}
	}

	// Check if we need to download a version
	numVersions := len(getAllInstalledVersions())
	if numVersions == 0 && !internetAvailable {
		d := dialog.NewInformation("No Internet Connection",
			"You must be connected to the internet to fetch a version of the program.\nPlease connect and restart the program", w)
		d.Resize(fyne.NewSize(400, 200))
		d.SetDismissText("Exit")
		d.SetOnClosed(func() {
			app.Quit()
		})
		d.Show()
		return
	}

	if numVersions == 0 && internetAvailable {
		// Do not auto-install. Show explanation and allow user to install via UI.
		message := widget.NewLabel("No OWLCMS version installed. Use the Install button to fetch a version.")
		updateTitleContainer.Objects = []fyne.CanvasObject{message}
		updateTitleContainer.Refresh()
		// leave downloads available for user to select
	}

	// Initialize version list
	recomputeVersionList(w)

	// Setup release dropdown
	setupReleaseDropdown(w)

	// Check for newer version
	checkForNewerVersion()

	// Show the version container
	stopContainer.Hide()
	versionContainer.Show()
	downloadContainer.Show()
	statusLabel.Hide()

	log.Println("OWLCMS tab setup done.")
}

// HideDownloadables hides the download dropdown
func HideDownloadables() {
	downloadsShown = false
	releaseDropdown.Hide()
	prereleaseCheckbox.Hide()
	downloadContainer.Refresh()
}

// ShowDownloadables shows the download dropdown
func ShowDownloadables() {
	downloadsShown = true
	releaseDropdown.Show()
	prereleaseCheckbox.Show()
	downloadContainer.Refresh()
}

func goBackToMainScreen() {
	stopButton.Hide()
	stopContainer.Hide()
	downloadContainer.Show()
	versionContainer.Show()
}

// checkJava checks for Java and downloads it if not found
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

	statusLabel.Hide()
	return nil
}

func checkForNewerVersion() {
	latestInstalled := findLatestInstalled()

	if latestInstalled != "" {
		updateExplanation()
	}

	if latestInstalled == "" {
		messageBox := container.NewHBox(
			widget.NewLabel("No version is installed."),
		)
		updateTitleContainer.Objects = []fyne.CanvasObject{messageBox}
		updateTitleContainer.Refresh()
		updateTitleContainer.Show()
		return
	}

	latestInstalledVersion, err := semver.NewVersion(latestInstalled)
	if err != nil {
		return
	}

	log.Printf("Latest installed version: %s\n", latestInstalledVersion)

	// Check for newer versions (both stable and prerelease)
	for _, release := range allReleases {
		releaseVersion, err := semver.NewVersion(release)
		if err != nil {
			continue
		}

		if releaseVersion.GreaterThan(latestInstalledVersion) {
			log.Printf("Found newer version: %s\n", releaseVersion)
			// Use helper to get the correct releases/tag URL (prerelease vs stable)
			releaseURL := GetReleaseTagURL(release)

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

			// Store the available version for the install link
			availableVersion = release
			availableVersionURL = releaseURL

			// Update release notes link
			parsedURL, _ := url.Parse(releaseURL)
			releaseNotesLink.SetURL(parsedURL)
			releaseNotesLink.Show()

			// Show install link
			installAvailableLink.Show()

			messageBox := container.NewHBox(
				widget.NewLabel(fmt.Sprintf("A more recent %s version %s is available.", versionType, releaseVersion)),
				releaseNotesLink,
				installAvailableLink,
			)
			updateTitleContainer.Objects = []fyne.CanvasObject{messageBox}
			updateTitleContainer.Refresh()
			updateTitleContainer.Show()
			return
		}
	}

	// If we get here, no newer version was found
	// Choose the correct releases repo for prerelease installs
	releaseURL := GetReleaseTagURL(latestInstalled)
	parsedURL, _ := url.Parse(releaseURL)
	releaseNotesLink.SetURL(parsedURL)
	// Ensure the release notes hyperlink is visible for both stable and prerelease cases
	releaseNotesLink.Show()

	messageBox := container.NewHBox(
		// Log what we think is installed for debugging
		log.Printf("OWLCMS:updateTitle - latestInstalled=%q installedVersions=%v", latestInstalled, getAllInstalledVersions())
		widget.NewLabel(fmt.Sprintf("You are using %s version %s", func() string {
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
		downloadContainer.Remove(singleOrMultiVersionLabel)
		downloadContainer.Refresh()
	} else if len(x) == 1 {
		latestStable, stableErr := getMostRecentStableRelease()
		latestPrerelease, preErr := getMostRecentPrerelease()

		downloadContainer.Remove(singleOrMultiVersionLabel)

		if containsPreReleaseTag(x[0]) {
			if preErr == nil && x[0] == latestPrerelease {
				// Latest prerelease installed
			} else {
				if len(downloadContainer.Objects) > 0 {
					downloadContainer.Objects = append(downloadContainer.Objects[:1], append([]fyne.CanvasObject{singleOrMultiVersionLabel}, downloadContainer.Objects[1:]...)...)
				} else {
					downloadContainer.Objects = append([]fyne.CanvasObject{singleOrMultiVersionLabel}, downloadContainer.Objects...)
				}
				singleOrMultiVersionLabel.SetText("Use the Update button above to install the latest version. The current database will be copied to the new version, as well as local changes made to the configuration since the previous installation.")
			}
		} else {
			if stableErr == nil && x[0] == latestStable {
				// Latest stable installed
			} else {
				if len(downloadContainer.Objects) > 0 {
					downloadContainer.Objects = append(downloadContainer.Objects[:1], append([]fyne.CanvasObject{singleOrMultiVersionLabel}, downloadContainer.Objects[1:]...)...)
				} else {
					downloadContainer.Objects = append([]fyne.CanvasObject{singleOrMultiVersionLabel}, downloadContainer.Objects...)
				}
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

func computeVersionScrollHeight(numVersions int) float32 {
	minHeight := 140 // ensure a reasonable minimum vertical space for the version list
	rowHeight := 50
	return float32(minHeight + (rowHeight * min(numVersions, 4)))
}

func removeAllVersions() {
	entries, err := os.ReadDir(installDir)
	if err != nil {
		log.Printf("Failed to read owlcms directory: %v\n", err)
		dialog.ShowError(fmt.Errorf("failed to read owlcms directory: %w", err), fyne.CurrentApp().Driver().AllWindows()[0])
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
	recomputeVersionList(fyne.CurrentApp().Driver().AllWindows()[0])
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
				fyne.CurrentApp().Quit()
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// HandleSignalCleanup handles cleanup when the application receives a signal
func HandleSignalCleanup() {
	if currentProcess != nil && currentProcess.Process != nil {
		pid := currentProcess.Process.Pid
		log.Printf("Stopping OWLCMS %s (PID: %d)...\n", currentVersion, pid)

		// Set the killedByUs flag first so the monitoring goroutine knows this was intentional
		killedByUs = true

		// Use forceful termination since we need to exit quickly
		ForcefullyKillProcess(pid)
	}
}

// IsRunning returns true if OWLCMS is currently running
func IsRunning() bool {
	return currentProcess != nil
}

// StopRunningProcess stops the running OWLCMS process
func StopRunningProcess(w fyne.Window) {
	if currentProcess != nil && currentProcess.Process != nil {
		log.Println("Stopping OWLCMS process")
		stopProcess(currentProcess, currentVersion, stopButton, downloadContainer, versionContainer, statusLabel, w)
	}
}
