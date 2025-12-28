package firmata

import (
	"fmt"
	"image/color"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"

	"owlcms-launcher/firmata/downloadutils"
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
	installDir                = getInstallDir()
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
	javacheck.InitJavaCheck(installDir)
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
	switch downloadutils.GetGoos() {
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
	stopButton.Hide()
	stopContainer.Hide()
	downloadContainer.Show()
	versionContainer.Show()
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

	// Configure stop button behavior
	stopButton.OnTapped = func() {
		log.Println("Stop button tapped")
		stopProcess(currentProcess, currentVersion, stopButton, downloadContainer, versionContainer, statusLabel, w)
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

	// Start initialization in a goroutine
	go initializeFirmataTab(w)

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
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Remove All Firmata Versions", func() {
			removeAllVersions()
		}),
		fyne.NewMenuItem("Remove Firmata Java", func() {
			removeJava()
		}),
		fyne.NewMenuItem("Remove All Firmata Stored Data and Configurations", func() {
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

// initializeFirmataTab handles the async initialization of the Firmata tab
func initializeFirmataTab(w fyne.Window) {
	var javaAvailable bool
	javaLoc, err := javacheck.FindLocalJava()
	javaAvailable = err == nil && javaLoc != ""

	// Check for internet connection before anything else
	internetAvailable := CheckForInternet()
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
		d := dialog.NewInformation("No Internet Connection", "You must be connected to the internet to fetch a version of Firmata.\nPlease connect and restart the program", w)
		d.Resize(fyne.NewSize(400, 200))
		d.SetDismissText("OK")
		d.Show()
		return
	}

	// Initialize version list
	recomputeVersionList(w)

	// Create prerelease checkbox first so the dropdown builder can include it in the same horizontal container
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
	updateTitle.Hide()
	releaseDropdown.Hide() // Hide the dropdown initially

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

	populateReleaseSelect(releaseSelect) // Populate the dropdown with the releases
	updateTitle.Show()
	releaseDropdown.Hide()
	prereleaseCheckbox.Hide()
	log.Printf("Fetched %d releases\n", len(releases))

	// If no version is installed, get the latest stable version
	if len(getAllInstalledVersions()) == 0 {
		for _, release := range allReleases {
			version := extractSemverTag(release)
			if !containsPreReleaseTag(version) {
				// Automatically download and install the latest stable version
				log.Printf("Downloading and installing latest stable version %s\n", version)
				downloadAndInstallVersion(version, w)
				break
			}
		}
	}

	// Check if a more recent version is available
	checkForNewerVersion()
	downloadContainer.Refresh()
	downloadContainer.Show()

	log.Println("Firmata tab setup done.")
	statusLabel.Hide()
	stopContainer.Hide()
}

// HideDownloadables hides the download dropdown and prerelease checkbox
func HideDownloadables() {
	downloadsShown = false
	releaseDropdown.Hide()
	prereleaseCheckbox.Hide()
	downloadContainer.Refresh()
}

// ShowDownloadables shows the download dropdown and prerelease checkbox
func ShowDownloadables() {
	downloadsShown = true
	releaseDropdown.Show()
	prereleaseCheckbox.Show()
	downloadContainer.Refresh()
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
		if err := os.MkdirAll(owlcmsDir, 0755); err != nil {
			dialog.ShowError(fmt.Errorf("creating firmata directory: %w", err), w)
			return
		}
	}

	// Show progress dialog
	progressDialog := dialog.NewCustom(
		"Installing owlcms-firmata",
		"Please wait...",
		widget.NewLabel("Downloading and extracting files..."),
		w)
	progressDialog.Show()

	go func() {
		extractPath := filepath.Join(owlcmsDir, version)
		os.Mkdir(extractPath, 0755)
		extractPath = filepath.Join(extractPath, fileName)

		// Download the file using downloadutils
		log.Printf("Starting download from URL: %s\n", zipURL)
		err := downloadutils.DownloadArchive(zipURL, extractPath)
		if err != nil {
			progressDialog.Hide()
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
					installLink := widget.NewHyperlink("install as additional version", nil)
					installLink.OnTapped = func() {
						ShowDownloadables()
					}

					messageBox := container.NewHBox(
						widget.NewLabel(fmt.Sprintf("A more recent %s version %s is available.", versionType, releaseVersion)),
						releaseNotesLink,
						installLink,
					)
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
			messageBox := container.NewHBox(
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
	} else {
		messageBox := container.NewHBox(
			widget.NewLabel("No version is installed."),
		)
		updateTitleContainer.Objects = []fyne.CanvasObject{messageBox}
		updateTitleContainer.Refresh()
		updateTitleContainer.Show()
	}
}
