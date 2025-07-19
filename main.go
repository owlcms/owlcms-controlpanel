package main

import (
	"fmt"
	"image/color"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"owlcms-launcher/downloadUtils"
	"owlcms-launcher/javacheck"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout" // Corrected import with v2
	"fyne.io/fyne/v2/storage"
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
	singleOrMultiVersionLabel *widget.Label     // New label for single or multi version update
	downloadContainer         *fyne.Container   // New global to track the same container
	downloadsShown            bool              // New global to track whether downloads are shown
	urlLink                   *widget.Hyperlink // Add this new variable
	appDirLink                *widget.Hyperlink // New: hyperlink for application directory
)

func init() {
	javacheck.InitJavaCheck(owlcmsInstallDir, GetTemurinVersion)
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
	updateTitle.ParseMarkdown("All Versions Removed.")
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
	log.Println("Starting OWLCMS Launcher")
	a := app.NewWithID("app.owlcms.owlcms-launcher")
	a.Settings().SetTheme(newMyTheme())
	w := a.NewWindow("OWLCMS Control Panel")
	w.Resize(fyne.NewSize(800, 430))
	w.Show()

	// Initialize environment early
	InitEnv()

	// Check for updates immediately after showing the window
	// Don't show "you're ok"" dialog when starting up
	go checkForUpdates(w, false)

	// Create stop button and status label
	stopButton = widget.NewButton("Stop", nil)
	stopButton.Importance = widget.HighImportance // Make the stop button important
	statusLabel = widget.NewLabel("Initializing OWLCMS Control Panel...")
	statusLabel.Wrapping = fyne.TextWrapWord // Allow status messages to wrap

	// Create URL hyperlink
	urlLink = widget.NewHyperlink("", nil)
	urlLink.Hide()

	// Create application directory hyperlink
	appDirLink = widget.NewHyperlink("", nil)
	appDirLink.Hide()

	// Create containers with initial loading state
	initialLoadingContent := container.NewVBox(
		widget.NewLabelWithStyle("OWLCMS Control Panel", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		statusLabel,
	)

	// Initialize hidden containers that will be shown later
	downloadContainer = container.NewVBox()
	versionContainer = container.NewMax()                                           // Use NewMax so it expands in the center
	stopContainer = container.NewVBox(stopButton, statusLabel, urlLink, appDirLink) // Add appDirLink here

	// Set initial content to show loading state immediately
	w.SetContent(initialLoadingContent)
	w.Show() // Show window immediately with loading indicator

	// Initialize download titles (these won't be visible yet)
	updateTitle = widget.NewRichTextFromMarkdown("")                                             // Initialize as RichText for Markdown
	downloadButtonTitle = widget.NewHyperlink("Click here to install additional versions.", nil) // New title for download button
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

	// Create the real main content that will replace the loading screen
	// Only put versionContainer in the center, and nothing below the border container.
	downloadContainer.Resize(fyne.NewSize(800, 180)) // Set a fixed height for the bottom container

	mainContent := container.NewBorder(
		stopContainer,     // Top (hidden except when stopping)
		downloadContainer, // Bottom (fixed height, always visible)
		nil,               // Left
		nil,               // Right
		versionContainer,  // Center (now a NewMax, expands to fill space)
	)

	// Start initialization in a goroutine
	go func() {
		// Check Java first
		statusLabel.SetText("Checking for Java runtime...")
		javaLoc, err := javacheck.FindLocalJava()
		internetAvailable := CheckForInternet()

		// Update status
		if err != nil || javaLoc == "" {
			if !internetAvailable {
				content := container.New(layout.NewCenterLayout(),
					widget.NewLabel("Java is not installed and there is no internet connection.\nPlease connect and restart the program."))

				dialog := dialog.NewCustom("No Internet Connection", "Exit", content, w)
				dialog.SetOnClosed(func() {
					a.Quit()
				})
				dialog.Show()
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
			d.Show()
			d.SetOnClosed(func() {
				a.Driver().Quit()
			})
			return
		}

		// Initialize version list and UI components
		recomputeVersionList(w)
		releaseSelect, releaseDropdown := createReleaseDropdown(w)
		updateTitle.Hide()
		releaseDropdown.Hide()

		prereleaseCheckbox = widget.NewCheck("Show Prereleases", func(checked bool) {
			showPrereleases = checked
			populateReleaseSelect(releaseSelect)
		})
		prereleaseCheckbox.Hide()

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

		// Setup menus
		setupMenus(w)

		// Update UI and switch to main content
		populateReleaseSelect(releaseSelect)
		updateTitle.Show()
		releaseDropdown.Hide()
		prereleaseCheckbox.Hide()

		// If no version is installed, get the latest stable version
		if len(getAllInstalledVersions()) == 0 && len(allReleases) > 0 {
			statusLabel.SetText("No versions installed. Getting latest stable version...")
			for _, release := range allReleases {
				if !containsPreReleaseTag(release) {
					// Download the latest stable version
					downloadAndInstallVersion(release, w)
					break
				}
			}
		}

		// Check if a more recent version is available
		checkForNewerVersion()

		// Switch from loading view to main view
		w.SetContent(mainContent)
		statusLabel.SetText("Ready")
		statusLabel.Hide()

		// Show the appropriate containers
		stopContainer.Hide()
		versionContainer.Show()
		downloadContainer.Show()
		mainContent.Refresh()

		setupCleanupOnExit(w)
		log.Println("Setup complete")
	}()

	// Set up signal handling - remove all parameters
	setupSignalHandling()

	// Run the application
	w.ShowAndRun()
}

// New helper function to setup menus
func setupMenus(w fyne.Window) {
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
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Open Installation Directory", func() {
			if err := openFileExplorer(owlcmsInstallDir); err != nil {
				dialog.ShowError(fmt.Errorf("failed to open installation directory: %w", err), w)
			}
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Install from Local ZIP", func() {
			// Create and show file dialog to select ZIP file
			fileDialog := dialog.NewFileOpen(
				func(reader fyne.URIReadCloser, err error) {
					if err != nil {
						dialog.ShowError(err, w)
						return
					}
					if reader == nil {
						// User canceled the operation
						return
					}

					// Process the selected ZIP file
					processLocalZipFile(reader.URI().Path(), w)
					reader.Close()
				},
				w,
			)

			// Set file filter to only show ZIP files
			fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".zip"}))
			fileDialog.Show()
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
	helpMenu := fyne.NewMenu("Help",
		fyne.NewMenuItem("Documentation", func() {
			linkURL, _ := url.Parse("https://owlcms.github.io/owlcms4-prerelease/#/LocalControlPanel")
			link := widget.NewHyperlink("Control Panel Documentation", linkURL)
			dialog.ShowCustom("Documentation", "Close", link, w)
		}),
		fyne.NewMenuItem("Check for Updates", func() {
			// Show confirmation dialog when checking from menu
			checkForUpdates(w, true)
		}),
		fyne.NewMenuItem("About", func() {
			dialog.ShowInformation("About", "OWLCMS Launcher version "+launcherVersion, w)
		}),
	)
	menu := fyne.NewMainMenu(fileMenu, killMenu, helpMenu)
	w.SetMainMenu(menu)
}

// processLocalZipFile handles a ZIP file selected from the file system
func processLocalZipFile(zipPath string, w fyne.Window) {
	// Extract version number from filename if possible
	fileName := filepath.Base(zipPath)
	version := ""

	// Try to extract version from filename (assuming format like "owlcms_4.24.1.zip" or any prefix with semver)
	if strings.HasSuffix(fileName, ".zip") {
		// Remove .zip extension
		nameWithoutExt := strings.TrimSuffix(fileName, ".zip")

		// Find the first digit which might be the start of a semantic version
		for i, char := range nameWithoutExt {
			if char >= '0' && char <= '9' {
				// Found first digit, extract from here to end as potential version
				version = nameWithoutExt[i:]
				break
			}
		}
	}

	// If version couldn't be determined or is invalid, ask the user
	if version == "" || !isValidSemVer(version) {
		content := widget.NewEntry()
		content.SetPlaceHolder("e.g., 4.24.1")

		versionDialog := dialog.NewCustomConfirm(
			"Enter Version",
			"Install",
			"Cancel",
			content,
			func(confirmed bool) {
				if !confirmed || content.Text == "" {
					return
				}

				if isValidSemVer(content.Text) {
					installLocalZipFile(zipPath, content.Text, w)
				} else {
					dialog.ShowError(fmt.Errorf("invalid version format, please use semantic versioning (e.g., 4.24.1)"), w)
				}
			},
			w,
		)
		versionDialog.Show()
	} else {
		// We have a valid version, proceed with installation
		installLocalZipFile(zipPath, version, w)
	}
}

// isValidSemVer checks if a string is a valid semantic version
func isValidSemVer(version string) bool {
	_, err := semver.NewVersion(version)
	return err == nil
}

// installLocalZipFile installs from a local ZIP file
func installLocalZipFile(zipPath, version string, w fyne.Window) {
	// Create a custom progress dialog
	progressBar := widget.NewProgressBar()
	progressBar.SetValue(0.1)
	messageLabel := widget.NewLabel(fmt.Sprintf("Installing OWLCMS %s from local file...", version))
	content := container.NewVBox(messageLabel, progressBar)
	progressDialog := dialog.NewCustom(
		"Installing OWLCMS",
		"Please wait...",
		content,
		w)
	progressDialog.Show()

	// Ensure the owlcms directory exists
	owlcmsDir := owlcmsInstallDir
	if _, err := os.Stat(owlcmsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(owlcmsDir, 0755); err != nil {
			progressDialog.Hide()
			dialog.ShowError(fmt.Errorf("creating owlcms directory: %w", err), w)
			return
		}
	}

	// Copy the ZIP with original filename to preserve it as-is
	originalFileName := filepath.Base(zipPath)
	destOriginalPath := filepath.Join(owlcmsDir, originalFileName)

	messageLabel.SetText("Copying ZIP file...")
	messageLabel.Refresh()

	progressBar.SetValue(0.3)

	// Copy the file with original name to the installation directory
	if zipPath != destOriginalPath {
		err := copyFile(zipPath, destOriginalPath)
		if err != nil {
			progressDialog.Hide()
			dialog.ShowError(fmt.Errorf("failed to copy ZIP file: %w", err), w)
			return
		}
	}

	extractPath := filepath.Join(owlcmsDir, version)

	go func() {
		progressBar.SetValue(0.5)
		messageLabel.SetText("Extracting files...")
		messageLabel.Refresh()

		// Use the original copied file for extraction
		log.Printf("Extracting ZIP file to: %s\n", extractPath)
		err := downloadUtils.ExtractZip(destOriginalPath, extractPath)
		if err != nil {
			progressDialog.Hide()
			dialog.ShowError(fmt.Errorf("extraction failed: %w", err), w)
			return
		}

		// Set to complete
		progressBar.SetValue(1.0)

		// Log when extraction is done
		log.Println("Extraction completed")
		updateExplanation()

		// Hide progress dialog
		progressDialog.Hide()

		// Show success panel with installation details
		message := fmt.Sprintf(
			"Successfully installed OWLCMS version %s\n\n"+
				"Location: %s\n\n"+
				"The program files have been extracted to the above directory.\n\n",
			version, extractPath)

		dialog.ShowInformation("Installation Complete", message, w)

		// Recompute the version list
		recomputeVersionList(w)

		// Recompute the downloadTitle
		checkForNewerVersion()
	}()
}

// New helper function to setup signal handling
func setupSignalHandling() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Signal %v caught, cleaning up.\n", sig)

		// Make sure we properly cleanup before exiting
		if currentProcess != nil && currentProcess.Process != nil {
			pid := currentProcess.Process.Pid
			log.Printf("Stopping OWLCMS %s (PID: %d)...\n", currentVersion, pid)

			// Set the killedByUs flag first so the monitoring goroutine knows this was intentional
			killedByUs = true

			// Use forceful termination since we need to exit quickly
			// the death is processed by launchOwlcms.go that is waiting on the process
			ForcefullyKillProcess(pid)
		}

		log.Println("Exiting Control Panel...")

		// Force exit with a slight delay to allow logs to be written
		time.AfterFunc(100*time.Millisecond, func() {
			os.Exit(0)
		})
	}()
}

// New helper function to setup cleanup on window close
func setupCleanupOnExit(w fyne.Window) {
	w.SetCloseIntercept(func() {
		if currentProcess != nil {
			confirmDialog := dialog.NewConfirm(
				"Confirm Exit",
				"The server is running. This will stop the owlcms server for all the users. Are you sure you want to exit?",
				func(confirm bool) {
					if !confirm {
						log.Println("Closing OWLCMS Launcher")
						// Use the same graceful termination method as in stopProcess
						if currentProcess != nil && currentProcess.Process != nil {
							stopProcess(currentProcess, currentVersion, stopButton, downloadContainer, versionContainer, statusLabel, w)
						}
						w.Close()
					}
				},
				w,
			)
			confirmDialog.SetConfirmText("Don't Stop owlcms")
			confirmDialog.SetDismissText("Stop owlcms and Exit")
			confirmDialog.Show()
		} else {
			w.Close()
		}
	})
}

func HideDownloadables() {
	downloadsShown = false
	releaseDropdown.Hide()
	prereleaseCheckbox.Hide()
	downloadContainer.Refresh()
}

func ShowDownloadables() {
	downloadsShown = true
	releaseDropdown.Show()
	prereleaseCheckbox.Show()
	downloadContainer.Refresh()
}

func downloadAndInstallVersion(version string, w fyne.Window) {
	// Create a custom progress dialog with both progress bar and message
	// Show it immediately before any network operations
	progressBar := widget.NewProgressBar()
	progressBar.SetValue(0.01) // Set a small initial value to show activity
	messageLabel := widget.NewLabel(fmt.Sprintf("Preparing to download OWLCMS %s...", version))
	content := container.NewVBox(messageLabel, progressBar)
	progressDialog := dialog.NewCustom(
		"Installing OWLCMS",
		"Please wait...",
		content,
		w)
	progressDialog.Show()

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
			progressDialog.Hide()
			dialog.ShowError(fmt.Errorf("creating owlcms directory: %w", err), w)
			return
		}
	}

	zipPath := filepath.Join(owlcmsDir, fileName)
	extractPath := filepath.Join(owlcmsDir, version)

	go func() {
		// Download the ZIP file using downloadUtils
		log.Printf("Starting download from URL: %s\n", zipURL)
		messageLabel.SetText(fmt.Sprintf("Downloading OWLCMS %s...", version))
		messageLabel.Refresh()

		progressCallback := func(downloaded, total int64) {
			if total > 0 {
				progress := float64(downloaded) / float64(total)
				progressBar.SetValue(progress)
			}
		}

		err := downloadUtils.DownloadArchive(zipURL, zipPath, progressCallback, nil)
		if err != nil {
			progressDialog.Hide()
			dialog.ShowError(fmt.Errorf("download failed: %w", err), w)
			return
		}

		progressBar.SetValue(0.9)
		messageLabel.SetText("Extracting files...")
		messageLabel.Refresh()

		// Extract the ZIP file to version-specific subdirectory
		log.Printf("Extracting ZIP file to: %s\n", extractPath)
		err = downloadUtils.ExtractZip(zipPath, extractPath)
		if err != nil {
			progressDialog.Hide()
			dialog.ShowError(fmt.Errorf("extraction failed: %w", err), w)
			return
		}

		// Set to complete
		progressBar.SetValue(1.0)

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
		HideDownloadables()

		// Recompute the version list
		recomputeVersionList(w)

		// Recompute the downloadTitle
		checkForNewerVersion()
	}()
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
						var releaseURL string
						if containsPreReleaseTag(release) {
							releaseURL = fmt.Sprintf("https://github.com/owlcms/owlcms4-prerelease/releases/tag/%s", release)
							if containsPreReleaseTag(latestInstalled) {
								updateTitle.ParseMarkdown(fmt.Sprintf("**A more recent prerelease version %s is available.** [Release Notes](%s)", releaseVersion, releaseURL))
								updateTitle.Refresh()
								updateTitle.Show()
								return
							}
						} else {
							releaseURL = fmt.Sprintf("https://github.com/owlcms/owlcms4/releases/tag/%s", release)
							updateTitle.ParseMarkdown(fmt.Sprintf("**A more recent stable version %s is available.** [Release Notes](%s)", releaseVersion, releaseURL))
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

			var releaseURL string
			if containsPreReleaseTag(latestInstalled) {
				stableURL := fmt.Sprintf("https://github.com/owlcms/owlcms4/releases/tag/%s", latestStable)
				prereleaseURL := fmt.Sprintf("https://github.com/owlcms/owlcms4-prerelease/releases/tag/%s", latestInstalled)
				updateTitle.ParseMarkdown(fmt.Sprintf(
					`**The latest installed version is pre-release %s** [Release Notes](%s)
					
The latest stable version is %s. [Release Notes](%s)`,
					latestInstalled, prereleaseURL, latestStable, stableURL))
			} else {
				releaseURL = fmt.Sprintf("https://github.com/owlcms/owlcms4/releases/tag/%s", latestInstalled)
				updateTitle.ParseMarkdown(fmt.Sprintf("**The latest stable version is installed.** [Release Notes](%s)", releaseURL))
			}
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
		updateTitle.ParseMarkdown("**No version installed. Select a version to download below.**")
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
		downloadContainer.Remove(singleOrMultiVersionLabel)
		downloadContainer.Refresh()
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
