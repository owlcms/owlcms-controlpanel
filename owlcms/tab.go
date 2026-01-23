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
	"fyne.io/fyne/v2/widget"
	"github.com/Masterminds/semver/v3"
)

var (
	statusLabel      *widget.Label
	stopButton       *widget.Button
	versionContainer *fyne.Container
	stopContainer    *fyne.Container
	// TEMPORARY TEST FLAG: when true, treat OWLCMS as not installed.
	// Keep variable for testing; default to false to use real detection.
	forceUninstalledOwlcms    = false
	singleOrMultiVersionLabel *widget.Label
	downloadContainer         *fyne.Container
	downloadsShown            bool
	urlLink                   *widget.Hyperlink
	appDirLink                *widget.Hyperlink
	tailLogLink               *widget.Hyperlink
	startupLogText            *widget.Entry
	startupLogContainer       *fyne.Container
	startupLogHost            *fyne.Container
	selectionContent          *fyne.Container
	runningContent            *fyne.Container
	modeStack                 *fyne.Container
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

	// Create Tail logs hyperlink (only shown when running on Windows)
	tailLogLink = widget.NewHyperlink("", nil)
	tailLogLink.Hide()

	// Initialize containers
	downloadContainer = container.NewVBox()
	versionContainer = container.NewStack()
	stopContainer = container.NewVBox(widget.NewSeparator(), stopButton, statusLabel, urlLink, appDirLink, tailLogLink)

	// Initialize download titles
	updateTitle = widget.NewRichTextFromMarkdown("")

	// Initialize release notes hyperlink
	releaseNotesLink = widget.NewHyperlink("Release Notes", nil)
	releaseNotesLink.Hide()

	// Initialize install hyperlink for available version
	installAvailableLink = widget.NewHyperlink("Install as new version", nil)
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

	// Configure stop button behavior (confirm before stopping)
	stopButton.OnTapped = func() {
		log.Println("Stop button tapped")
		confirmDialog := dialog.NewConfirm(
			"Confirm Stop",
			"Stopping OWLCMS will stop the current competition on all platforms. Make sure this is a correct time to stop.",
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

	// Set a fixed height for the bottom container
	downloadContainer.Resize(fyne.NewSize(800, 180))

	// Create menu bar
	menuBar := createMenuBar(w)

	// Add small spacer under the menu bar to increase top area
	topSpacer := canvas.NewRectangle(color.Transparent)
	topSpacer.SetMinSize(fyne.NewSize(1, 8))

	// Two different layouts:
	// - Selection mode: version list (center) + download section (bottom)
	// - Running mode: startup log host (center), no bottom section
	startupLogHost = container.NewStack()
	startupLogHost.Hide()

	selectionContent = container.NewBorder(
		nil,
		downloadContainer,
		nil,
		nil,
		versionContainer,
	)

	runningContent = container.NewMax(startupLogHost)
	runningContent.Hide()

	modeStack = container.NewStack(selectionContent, runningContent)

	mainContent := container.NewBorder(
		container.NewVBox(menuBar, topSpacer, stopContainer), // Top (menu bar, spacer and stop container)
		nil,       // Bottom (handled by selectionContent)
		nil,       // Left
		nil,       // Right
		modeStack, // Center switches between selection/running layouts
	)

	// If OWLCMS install directory does not exist, reset tab to explanation mode
	if forceUninstalledOwlcms || func() bool { _, err := os.Stat(installDir); return os.IsNotExist(err) }() {
		setOwlcmsTabModeUninstalled(w)
		return mainContent
	} else {
		// Start initialization in a goroutine
		go initializeOwlcmsTab(w)
	}

	return mainContent
}

func showSelectionLayout() {
	if selectionContent != nil {
		selectionContent.Show()
	}
	if runningContent != nil {
		runningContent.Hide()
	}
	if startupLogHost != nil {
		startupLogHost.Hide()
	}
}

func showRunningLayout() {
	if selectionContent != nil {
		selectionContent.Hide()
	}
	if runningContent != nil {
		runningContent.Show()
	}
}

// setOwlcmsTabModeRunning switches the tab into the running layout (no version picker).
func setOwlcmsTabModeRunning() {
	showRunningLayout()
	if versionContainer != nil {
		versionContainer.Hide()
	}
	if downloadContainer != nil {
		downloadContainer.Hide()
	}
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
		fyne.NewMenuItem("Refresh Available Versions", func() {
			refreshAvailableVersions(w)
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
		// Commented out: remove all versions via Files menu (use Uninstall instead)
		// fyne.NewMenuItem("Remove All OWLCMS Versions", func() {
		// 	removeAllVersions()
		// }),
		// Commented out: remove bundled Java via Files menu
		// fyne.NewMenuItem("Remove OWLCMS Java", func() {
		// 	removeJava()
		// }),
		fyne.NewMenuItem("Uninstall OWLCMS", func() {
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
		menuItemText = "Disable connection to local tracker"
	} else {
		menuItemText = "Enable connection to local tracker"
	}

	trackerToggleItem := fyne.NewMenuItem(menuItemText, nil)
	trackerToggleItem.Action = func() {
		// Get the tracker port
		trackerPort := tracker.GetPort()
		trackerURL := fmt.Sprintf("ws://127.0.0.1:%s/ws", trackerPort)

		// Toggle the connection
		if GetTrackerConnectionEnabled() {
			// Disable
			if err := DeleteProperty("OWLCMS_VIDEODATA"); err != nil {
				dialog.ShowError(fmt.Errorf("failed to disable tracker connection: %w", err), w)
				return
			}
			dialog.ShowInformation("Tracker Connection", "Tracker connection disabled. Restart OWLCMS for changes to take effect.", w)
			trackerToggleItem.Label = "Enable connection to local tracker"
			return
		}

		// Enable
		if err := SaveProperty("OWLCMS_VIDEODATA", trackerURL); err != nil {
			dialog.ShowError(fmt.Errorf("failed to enable tracker connection: %w", err), w)
			return
		}
		dialog.ShowInformation("Tracker Connection", fmt.Sprintf("Tracker connection enabled on port %s. OWLCMS will connect to Tracker once it is started. Restart OWLCMS for changes to take effect.", trackerPort), w)
		trackerToggleItem.Label = "Disable connection to local tracker"
	}

	optionsMenuItems := []*fyne.MenuItem{trackerToggleItem}

	optionsMenu := shared.CreateMenuButton("Options", optionsMenuItems)

	// Add small vertical padding
	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(fyne.NewSize(1, 5))

	return container.NewVBox(
		spacer,
		container.NewHBox(fileMenu, processMenu, optionsMenu),
	)
}

func refreshAvailableVersions(w fyne.Window) {
	go func() {
		// Reset release-related state to mirror a fresh app start.
		showPrereleases = false
		allReleases = nil
		if prereleaseCheckbox != nil {
			prereleaseCheckbox.SetChecked(false)
		}

		releases, err := fetchReleases()
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to refresh available versions: %w", err), w)
			return
		}
		allReleases = releases

		// Rebuild the download UI as if the app just started.
		setupReleaseDropdown(w)
		recomputeVersionList(w)
		checkForNewerVersion()
		if downloadContainer != nil {
			downloadContainer.Refresh()
		}
	}()
}

// initializeOwlcmsTab handles the async initialization of the OWLCMS tab
func initializeOwlcmsTab(w fyne.Window) {
	// Set the appropriate mode based on installed versions
	if len(getAllInstalledVersions()) == 0 {
		setOwlcmsTabModeUninstalled(w)
	} else {
		setOwlcmsTabModeInstalled(w)
	}
	log.Println("OWLCMS tab setup done.")
}

// HideDownloadables hides the download dropdown
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

// ShowDownloadables shows the download dropdown
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

// setOwlcmsTabModeUninstalled is the ONLY way to show the "0 installed versions" UI.
// It must NOT show the download section.
func setOwlcmsTabModeUninstalled(w fyne.Window) {
	showSelectionLayout()
	resetToExplainMode(w)
	log.Printf("UI Mode: Uninstalled (0 versions)")
}

// setOwlcmsTabModeInstalled is the ONLY way to show the "≥1 installed versions" UI.
// It MUST show BOTH the version list and the download section.
func setOwlcmsTabModeInstalled(w fyne.Window) {
	showSelectionLayout()
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

// setOwlcmsTabMode is the single switch deciding which of the two modes to show.
// This prevents any code path from leaving the version list visible without the
// matching download section.
func setOwlcmsTabMode(w fyne.Window) {
	if len(getAllInstalledVersions()) == 0 {
		setOwlcmsTabModeUninstalled(w)
		return
	}
	setOwlcmsTabModeInstalled(w)
}

func goBackToMainScreen() {
	setOwlcmsTabMode(mainWindow)
}

// checkJava checks for Java and downloads it if not found
func checkJava(statusLabel *widget.Label) error {
	statusLabel.SetText("Checking for the Java language runtime.")
	statusLabel.Refresh()
	statusLabel.Show()
	if stopButton != nil {
		stopButton.Hide()
	}
	if stopContainer != nil {
		stopContainer.Show()
	}
	if versionContainer != nil {
		versionContainer.Hide()
	}
	// Don't hide downloadContainer during Java check - it will be restored after

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

	latestInstalledVersion, err := shared.NewVersionForComparison(latestInstalled)
	if err != nil {
		return
	}

	log.Printf("Latest installed version: %s\n", latestInstalled)

	// Check for newer versions (both stable and prerelease)
	for _, release := range allReleases {
		releaseVersion, err := shared.NewVersionForComparison(release)
		if err != nil {
			continue
		}

		if releaseVersion.GreaterThan(latestInstalledVersion) {
			log.Printf("Found newer version: %s\n", release)
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

			messageBox := shared.CreateUpdateNotification(versionType, releaseVersion.String(), installAvailableLink, releaseNotesLink)
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

	// Log what we think is installed for debugging
	log.Printf("OWLCMS:updateTitle - latestInstalled=%q installedVersions=%v", latestInstalled, getAllInstalledVersions())

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
		// Only auto-hide the dropdown when downloads are not expanded.
		// If the user is interacting with the prerelease toggle, keep it visible.
		if !downloadsShown {
			releaseDropdown.Hide()
		}
	}
	if downloadContainer != nil {
		downloadContainer.Refresh()
	}
}

func updateExplanation() {
	if len(allReleases) == 0 {
		if !downloadsShown {
			return
		}
		downloadContainer.Objects = []fyne.CanvasObject{
			widget.NewLabel("You are not connected to the Internet. Available updates cannot be shown."),
		}
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
	setOwlcmsTabMode(mainWindow)
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
				// Do not quit the control panel. Refresh the UI so the uninstalled explanation appears.
				setOwlcmsTabMode(mainWindow)
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
