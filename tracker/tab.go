package tracker

import (
	"fmt"
	"image/color"
	"log"
	"os"
	"os/exec"

	"owlcms-launcher/shared"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var (
	installDir = getInstallDir()
	// TEMPORARY TEST FLAG: when true, treat Tracker as not installed.
	// Keep variable for testing; default to false to use real detection.
	forceUninstalledTracker   = false
	tabRoot                   *fyne.Container
	currentProcess            *exec.Cmd
	currentVersion            string
	statusLabel               *widget.Label
	stopButton                *widget.Button
	versionContainer          *fyne.Container
	stopContainer             *fyne.Container
	singleOrMultiVersionLabel *widget.Label
	downloadContainer         *fyne.Container
	downloadsShown            bool
	urlLink                   *widget.Hyperlink
	appDirLink                *widget.Hyperlink
	tailLogLink               *widget.Hyperlink
	mainWindow                fyne.Window
	killedByUs                bool
)

// IsRunning returns true if Tracker is currently running
func IsRunning() bool {
	return currentProcess != nil
}

// OnTabSelected is called when the Tracker tab is selected.
// It refreshes the version list to update the OWLCMS version warning.
func OnTabSelected() {
	if mainWindow != nil && versionContainer != nil && len(getAllInstalledVersions()) > 0 {
		// If tracker is running, don't show the version list
		if IsRunning() {
			return
		}
		recomputeVersionList(mainWindow)
	}
}

// StopRunningProcess stops the running Tracker process
func StopRunningProcess(w fyne.Window) {
	if currentProcess != nil && currentProcess.Process != nil {
		log.Println("Stopping Tracker process")
		stopProcess(currentProcess, currentVersion, stopButton, downloadContainer, versionContainer, statusLabel, w)
	}
}

// HandleSignalCleanup handles cleanup when the application receives a signal
func HandleSignalCleanup() {
	if currentProcess != nil && currentProcess.Process != nil {
		pid := currentProcess.Process.Pid
		log.Printf("Forcefully stopping Tracker (PID: %d)...\n", pid)
		killedByUs = true
		// Use direct kill for fast cleanup
		if err := currentProcess.Process.Kill(); err != nil {
			log.Printf("Failed to kill Tracker process %d: %v\n", pid, err)
		} else {
			log.Printf("Tracker process %d killed\n", pid)
		}
		currentProcess = nil
	}
}

// CreateTab creates and returns the Tracker tab content
func CreateTab(w fyne.Window) *fyne.Container {
	initConfig()

	// Store main window reference
	mainWindow = w

	log.Println("Creating Tracker tab content")

	// Create stop button and status label
	stopButton = widget.NewButtonWithIcon("Stop", theme.CancelIcon(), nil)
	stopButton.Importance = widget.SuccessImportance // Dark green, matching Firmata
	statusLabel = widget.NewLabel("")
	statusLabel.Wrapping = fyne.TextWrapWord

	// Create containers
	downloadContainer = container.NewVBox()
	versionContainer = container.NewStack()

	// Create URL hyperlink
	urlLink = widget.NewHyperlink("", nil)
	urlLink.Hide()

	// App directory + log tail links (shown when running)
	appDirLink = widget.NewHyperlink("", nil)
	appDirLink.Hide()
	tailLogLink = widget.NewHyperlink("", nil)
	tailLogLink.Hide()

	stopContainer = container.NewVBox(widget.NewSeparator(), stopButton, statusLabel, urlLink, appDirLink, tailLogLink)

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
			"Stop the running Tracker process?",
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

	mainContent := container.NewBorder(
		container.NewVBox(menuBar, topSpacer, stopContainer), // Top (menu bar, spacer and stop container)
		downloadContainer, // Bottom
		nil,               // Left
		nil,               // Right
		versionContainer,  // Center (expands to fill space)
	)
	tabRoot = mainContent
	statusLabel.SetText("Checking installation status...")
	statusLabel.Refresh()
	statusLabel.Show()
	stopContainer.Show()

	// If the installation directory does not exist, reset the tab to explanation mode
	if forceUninstalledTracker || func() bool { _, err := os.Stat(installDir); return os.IsNotExist(err) }() {
		resetToExplainMode()
		return mainContent
	}

	// Start initialization in a goroutine for installed case
	go initializeTab(w)

	return mainContent
}

// createMenuBar creates the menu bar with File and Processes menus
func createMenuBar(w fyne.Window) *fyne.Container {
	// Create the File menu button with popup
	fileMenuItems := []*fyne.MenuItem{
		fyne.NewMenuItem("Open Tracker Installation Directory", func() {
			if err := shared.OpenFileExplorer(installDir); err != nil {
				dialog.ShowError(fmt.Errorf("failed to open installation directory: %w", err), w)
			}
		}),
		fyne.NewMenuItem("Refresh Available Versions", func() {
			refreshAvailableVersions(w)
		}),
		fyne.NewMenuItem("Install Tracker version from ZIP", func() {
			selectLocalZip(w, func(path string, err error) {
				if err != nil {
					dialog.ShowError(fmt.Errorf("file selection failed: %w", err), w)
					return
				}
				if path == "" {
					return
				}
				ProcessLocalZipFile(path, w, installDir, updateExplanation, recomputeVersionList, checkForNewerVersion)
			})
		}),
		fyne.NewMenuItemSeparator(),
		// Commented out: remove all versions via Files menu (use Uninstall instead)
		// fyne.NewMenuItem("Remove All Tracker Versions", func() {
		// 	removeAllVersions()
		// }),
		fyne.NewMenuItem("Uninstall Tracker", func() {
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

// initializeTab handles the async initialization of the Tracker tab
func initializeTab(w fyne.Window) {
	// Set the appropriate mode based on installed versions
	if len(getAllInstalledVersions()) == 0 {
		setTrackerTabModeUninstalled(w)
	} else {
		setTrackerTabModeInstalled(w)
	}
	log.Println("Tracker tab setup done.")
}

// setTrackerTabModeUninstalled shows the install prompt for when no versions are installed.
func setTrackerTabModeUninstalled(_ fyne.Window) {
	resetToExplainMode()
	log.Printf("UI Mode: Uninstalled (0 versions)")
}

// setTrackerTabModeInstalled shows the version list and download section.
func setTrackerTabModeInstalled(w fyne.Window) {
	// If tracker is running, don't switch to version list mode
	if IsRunning() {
		log.Printf("UI Mode: Running - not switching to installed mode")
		return
	}

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

// setTrackerTabMode is the single switch deciding which mode to show.
func setTrackerTabMode(w fyne.Window) {
	if len(getAllInstalledVersions()) == 0 {
		setTrackerTabModeUninstalled(w)
		return
	}
	setTrackerTabModeInstalled(w)
}

// computeVersionScrollHeight returns a minimum height for the version list scroll
// so it can display up to 4 rows without being too small.
func computeVersionScrollHeight(numVersions int) float32 {
	minHeight := 140 // minimum height to provide adequate vertical space
	rowHeight := 50  // approximate height per row
	return float32(minHeight + (rowHeight * min(numVersions, 4)))
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// resetToExplainMode updates the tab UI to show the explanation and Install button
func resetToExplainMode() {
	if versionContainer == nil {
		return
	}
	// Clear/hide the update and download UI so leftover messages (like "You are using...")
	if updateTitleContainer != nil {
		updateTitleContainer.Objects = []fyne.CanvasObject{}
		updateTitleContainer.Hide()
		updateTitleContainer.Refresh()
	}
	if downloadContainer != nil {
		downloadContainer.Objects = []fyne.CanvasObject{}
		downloadContainer.Hide()
		downloadContainer.Refresh()
	}
	if releaseDropdown != nil {
		releaseDropdown.Hide()
	}
	if prereleaseCheckbox != nil {
		prereleaseCheckbox.Hide()
	}

	// Also hide status/stop UI
	if statusLabel != nil {
		statusLabel.SetText("")
		statusLabel.Hide()
	}
	if stopContainer != nil {
		stopContainer.Hide()
	}
	if appDirLink != nil {
		appDirLink.Hide()
	}
	if tailLogLink != nil {
		tailLogLink.Hide()
	}

	// Now show the uninstalled explanation and Install button
	shared.ShowUninstalledTabContent(versionContainer, "asset/tracker.md", func() { InstallDefault(fyne.CurrentApp().Driver().AllWindows()[0]) }, nil)
	// Ensure the version container is visible after switching to explanation mode
	versionContainer.Show()
	versionContainer.Refresh()
}
