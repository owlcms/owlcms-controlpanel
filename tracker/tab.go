package tracker

import (
	"log"
	"net/http"
	"os/exec"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var (
	installDir                = getInstallDir()
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

	mainContent := container.NewBorder(
		stopContainer,     // Top
		downloadContainer, // Bottom
		nil,               // Left
		nil,               // Right
		versionContainer,  // Center (expands to fill space)
	)
	statusLabel.SetText("Checking installation status...")
	statusLabel.Refresh()
	statusLabel.Show()
	stopContainer.Show()

	// Start initialization in a goroutine
	go initializeTab(w)

	return mainContent
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

	// If no version is installed, get the latest stable version
	if len(getAllInstalledVersions()) == 0 && len(allReleases) > 0 {
		statusLabel.SetText("No versions installed. Getting latest stable version...")
		for _, release := range allReleases {
			if !containsPreReleaseTag(release) {
				downloadAndInstallVersion(release, w)
				break
			}
		}
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
