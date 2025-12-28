package tracker

import (
	"fmt"
	"image/color"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"owlcms-launcher/shared"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var (
	installDir                = getInstallDir()
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
	mainWindow                fyne.Window
	killedByUs                bool
)

// IsRunning returns true if Tracker is currently running
func IsRunning() bool {
	return currentProcess != nil
}

// StopRunningProcess stops the running Tracker process
func StopRunningProcess(w fyne.Window) {
	if currentProcess != nil && currentProcess.Process != nil {
		log.Println("Stopping Tracker process")
		stopProcess(currentProcess, currentVersion, stopButton, downloadContainer, versionContainer, statusLabel, w)
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

	// If the installation directory does not exist, show the shared explanation/install UI
	if _, err := os.Stat(installDir); os.IsNotExist(err) {
		shared.ShowUninstalledTabContent(versionContainer, "asset/tracker.md", func() {
			latest, err := getMostRecentStableRelease()
			log.Printf("Tracker Install button: getMostRecentStableRelease -> latest=%q err=%v", latest, err)
			if err == nil && latest != "" {
				log.Printf("Tracker Install: starting downloadAndInstallVersion(%s)", latest)
				downloadAndInstallVersion(latest, w)
			} else {
				log.Println("Tracker Install: no latest stable found, showing download UI")
				ShowDownloadables()
			}
		}, func() { initializeTab(w) })
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
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Remove All Tracker Versions", func() {
			removeAllVersions()
		}),
		fyne.NewMenuItem("Remove All Tracker Stored Data and Configurations", func() {
			uninstallAll()
			// Update UI to explanation/install mode
			resetToExplainMode()
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

// initializeTab handles the async initialization of the Tracker tab
func initializeTab(w fyne.Window) {
	// Check for internet connection
	internetAvailable := CheckForInternet()

	var releases []string
	var err error
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
		d := dialog.NewInformation("No Internet Connection", "You must be connected to the internet to fetch a version of Tracker.\nPlease connect and restart the program", w)
		d.Resize(fyne.NewSize(400, 200))
		d.SetDismissText("OK")
		d.Show()
		return
	}

	// Initialize version list
	recomputeVersionList(w)

	// Create prerelease checkbox
	var releaseSelect *widget.Select
	prereleaseCheckbox = widget.NewCheck("Show Prereleases", func(checked bool) {
		showPrereleases = checked
		if releaseSelect != nil {
			populateReleaseSelect(releaseSelect)
		}
	})
	prereleaseCheckbox.Hide()

	// Create release dropdown for downloads
	releaseSelect, releaseDropdownLocal := createReleaseDropdown(w)
	releaseDropdown = releaseDropdownLocal

	if len(allReleases) > 0 {
		downloadContainer.Objects = []fyne.CanvasObject{
			updateTitleContainer,
			singleOrMultiVersionLabel,
			downloadButtonTitle,
			releaseDropdown,
		}
	} else {
		downloadContainer.Objects = []fyne.CanvasObject{
			widget.NewLabel("You are not connected to the Internet. Available updates cannot be shown."),
		}
	}

	populateReleaseSelect(releaseSelect)
	updateTitle.Show()
	releaseDropdown.Hide()
	prereleaseCheckbox.Hide()

	// If no version is installed, do NOT auto-install. Leave downloads available for user.
	if len(getAllInstalledVersions()) == 0 {
		log.Println("No Tracker versions installed; not auto-installing. Waiting for user action.")
	}

	// Check if a more recent version is available
	checkForNewerVersion()

	// Hide the loading indicators
	statusLabel.SetText("")
	statusLabel.Hide()
	stopContainer.Hide()
	versionContainer.Show()
	downloadContainer.Show()
	downloadContainer.Refresh()

	log.Println("Tracker tab setup done.")
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

// CheckForInternet checks if there is internet connectivity
func CheckForInternet() bool {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get("https://api.github.com")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
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
	shared.ShowUninstalledTabContent(versionContainer, "asset/tracker.md", func() { ShowDownloadables() }, nil)
}
