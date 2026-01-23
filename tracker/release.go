package tracker

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"time"

	"owlcms-launcher/shared"
	"owlcms-launcher/tracker/downloadutils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/Masterminds/semver/v3"
)

// Release represents a GitHub release
type Release struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

var (
	showPrereleases      bool = false
	allReleases          []string
	releaseDropdown      *fyne.Container
	prereleaseCheckbox   *widget.Check
	updateTitle          *widget.RichText
	updateTitleContainer *fyne.Container
	downloadButtonTitle  *widget.Hyperlink
)

func fetchReleases() ([]string, error) {
	url := "https://api.github.com/repos/owlcms/owlcms-tracker/releases"

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var releases []Release
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, fmt.Errorf("invalid response format: %w", err)
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}

	releaseNames := make([]string, 0, len(releases))
	for _, release := range releases {
		// Use tag_name as it's more reliable for versioning
		releaseNames = append(releaseNames, release.TagName)
	}

	// Sort the release names in semver order, most recent at the top
	// Uses shared.CompareVersions which considers SNAPSHOT more recent than other prereleases
	sort.Slice(releaseNames, func(i, j int) bool {
		return shared.CompareVersions(releaseNames[i], releaseNames[j])
	})

	// Log the latest versions
	var latestStable, latestPrerelease string
	for _, release := range releaseNames {
		if !containsPreReleaseTag(release) {
			latestStable = release
			break
		}
	}
	for _, release := range releaseNames {
		if containsPreReleaseTag(release) {
			latestPrerelease = release
			break
		}
	}
	log.Printf("Tracker - Available from GitHub - Latest stable: %s, Latest prerelease: %s\n",
		latestStable, latestPrerelease)

	return releaseNames, nil
}

func populateReleaseSelect(selectWidget *widget.Select) {
	filteredReleases := []string{}
	stableReleases := []string{}
	for _, release := range allReleases {
		if showPrereleases || !containsPreReleaseTag(release) {
			filteredReleases = append(filteredReleases, release)
		}
		if !containsPreReleaseTag(release) {
			stableReleases = append(stableReleases, release)
		}
	}
	if !showPrereleases && len(stableReleases) > 20 {
		filteredReleases = stableReleases[:20]
	}
	selectWidget.Options = filteredReleases
	selectWidget.Refresh()
}

func containsPreReleaseTag(version string) bool {
	return shared.IsPrerelease(version)
}

func createReleaseDropdown(w fyne.Window) (*widget.Select, *fyne.Container) {
	releaseSelect := widget.NewSelect([]string{}, func(selected string) {
		if selected != "" {
			confirmAndDownloadVersion(selected, w)
		}
	})
	releaseSelect.PlaceHolder = "Select a version to install"

	prereleaseCheckbox = widget.NewCheck("Show Prereleases", func(checked bool) {
		showPrereleases = checked
		// Refetch to get updated list when toggling prereleases.
		go func() {
			releases, err := fetchReleases()
			if err != nil {
				log.Printf("failed to fetch releases after prerelease toggle: %v", err)
				return
			}
			allReleases = releases
			populateReleaseSelect(releaseSelect)
			checkForNewerVersion()
			// Keep the dropdown visible while the user is toggling prereleases.
			ShowDownloadables()
			if downloadContainer != nil {
				downloadContainer.Refresh()
			}
		}()
	})
	prereleaseCheckbox.Hide()

	dropdownContainer := container.NewHBox(releaseSelect, prereleaseCheckbox)
	return releaseSelect, dropdownContainer
}

// setupReleaseDropdown initializes the release dropdown and populates it with available releases.
// It fetches releases on-demand if not already loaded.
func setupReleaseDropdown(w fyne.Window) {
	// Fetch releases on-demand if not already loaded
	if len(allReleases) == 0 {
		if r, err := fetchReleases(); err == nil {
			allReleases = r
		} else {
			log.Printf("Tracker setupReleaseDropdown: fetchReleases failed: %v", err)
		}
	}

	selectWidget, dropdownContainer := createReleaseDropdown(w)
	releaseDropdown = dropdownContainer

	if len(allReleases) > 0 {
		downloadContainer.Objects = []fyne.CanvasObject{
			updateTitleContainer,
			singleOrMultiVersionLabel,
			downloadButtonTitle,
			dropdownContainer,
		}
		// Hide dropdown and checkbox initially; show only the link
		dropdownContainer.Hide()
		if prereleaseCheckbox != nil {
			prereleaseCheckbox.Hide()
		}
		downloadsShown = false
	} else {
		downloadContainer.Objects = []fyne.CanvasObject{
			updateTitleContainer,
			singleOrMultiVersionLabel,
			downloadButtonTitle,
		}
	}
	populateReleaseSelect(selectWidget)
	downloadContainer.Refresh()
}

func confirmAndDownloadVersion(version string, w fyne.Window) {
	dialog.ShowConfirm("Confirm Download",
		fmt.Sprintf("Do you want to download and install Tracker version %s?", version),
		func(confirm bool) {
			if confirm {
				shared.PromptForInstallVersionName(installDir, version, w, func(newVersion string) {
					downloadAndInstallVersion(version, newVersion, w)
				})
			}
		}, w)
}

func downloadAndInstallVersion(downloadVersion, installVersion string, w fyne.Window) {
	// Create a progress dialog
	progressBar := widget.NewProgressBar()
	progressBar.SetValue(0.01)
	messageLabel := widget.NewLabel(fmt.Sprintf("Preparing to download Tracker %s...", downloadVersion))
	content := container.NewVBox(messageLabel, progressBar)
	progressDialog := dialog.NewCustom(
		"Installing Tracker",
		"Please wait...",
		content,
		w)
	progressDialog.Show()

	// Try device-independent name first, then old platform-specific names for backward compatibility
	assetNames := getAssetNames(downloadVersion)
	var zipURL, assetName string

	// Check each asset name in order until we find one that exists
	for _, name := range assetNames {
		testURL := fmt.Sprintf("https://github.com/owlcms/owlcms-tracker/releases/download/%s/%s", downloadVersion, name)
		if checkAssetExists(testURL) {
			zipURL = testURL
			assetName = name
			break
		}
	}

	if zipURL == "" {
		// None of the expected assets exist - fail immediately
		progressDialog.Hide()
		dialog.ShowError(fmt.Errorf("no tracker release asset found for version %s (tried: %v)", downloadVersion, assetNames), w)
		return
	}

	// Ensure the tracker directory exists
	trackerDir := installDir
	if _, err := os.Stat(trackerDir); os.IsNotExist(err) {
		if err := shared.EnsureDir0755(trackerDir); err != nil {
			progressDialog.Hide()
			dialog.ShowError(fmt.Errorf("creating tracker directory: %w", err), w)
			return
		}
	}

	zipPath := filepath.Join(trackerDir, assetName)
	extractPath := filepath.Join(trackerDir, installVersion)

	go func() {
		log.Printf("Starting download from URL: %s\n", zipURL)
		messageLabel.SetText(fmt.Sprintf("Downloading Tracker %s...", downloadVersion))
		messageLabel.Refresh()

		progressCallback := func(downloaded, total int64) {
			if total > 0 {
				progress := float64(downloaded) / float64(total)
				progressBar.SetValue(progress)
			}
		}

		err := downloadutils.DownloadArchive(zipURL, zipPath, progressCallback, nil)
		if err != nil {
			progressDialog.Hide()
			dialog.ShowError(fmt.Errorf("download failed: %w", err), w)
			return
		}

		messageLabel.SetText("Extracting files...")
		messageLabel.Refresh()

		log.Printf("Extracting ZIP file to: %s\n", extractPath)
		extractProgress := func(extracted, total int64) {
			if total > 0 {
				progressBar.SetValue(float64(extracted) / float64(total))
			}
		}
		err = downloadutils.ExtractZip(zipPath, extractPath, extractProgress)
		if err != nil {
			progressDialog.Hide()
			dialog.ShowError(fmt.Errorf("extraction failed: %w", err), w)
			return
		}

		progressBar.SetValue(1.0)
		log.Println("Extraction completed")
		// Ensure the tab UI is initialized so download UI widgets exist
		initializeTab(w)
		updateExplanation()

		progressDialog.Hide()

		message := fmt.Sprintf(
			"Successfully installed Tracker version %s\n\n"+
				"Location: %s\n\n"+
				"The program files have been extracted to the above directory.",
			installVersion, extractPath)

		dialog.ShowInformation("Installation Complete", message, w)
		HideDownloadables()

		recomputeVersionList(w)
		checkForNewerVersion()
	}()
}

// getAssetNames returns possible asset names to try, starting with device-independent
// then falling back to device-dependent names for backward compatibility
func getAssetNames(version string) []string {
	goos := downloadutils.GetGoos()
	goarch := downloadutils.GetGoarch()

	// Try device-independent first
	assetNames := []string{fmt.Sprintf("owlcms-tracker_%s.zip", version)}

	// Fall back to platform-specific names for backward compatibility
	switch goos {
	case "windows":
		assetNames = append(assetNames, fmt.Sprintf("owlcms-tracker-windows_%s.zip", version))
	case "darwin":
		if goarch == "arm64" {
			assetNames = append(assetNames, fmt.Sprintf("owlcms-tracker-macos-arm64_%s.zip", version))
		} else {
			assetNames = append(assetNames, fmt.Sprintf("owlcms-tracker-macos-x64_%s.zip", version))
		}
	case "linux":
		// Linux uses the Raspberry Pi build
		assetNames = append(assetNames, fmt.Sprintf("owlcms-tracker-rpi_%s.zip", version))
	default:
		assetNames = append(assetNames, fmt.Sprintf("owlcms-tracker-rpi_%s.zip", version))
	}

	return assetNames
}

// checkAssetExists performs a HEAD request to check if an asset exists
func checkAssetExists(url string) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Head(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func getMostRecentStableRelease() (string, error) {
	for _, release := range allReleases {
		if !containsPreReleaseTag(release) {
			return release, nil
		}
	}
	return "", fmt.Errorf("no stable release found")
}

func getMostRecentPrerelease() (string, error) {
	for _, release := range allReleases {
		if containsPreReleaseTag(release) {
			return release, nil
		}
	}
	return "", fmt.Errorf("no prerelease found")
}

// InstallDefault performs the default install action for the Tracker package.
// It selects the most recent stable release and starts the confirm+download flow,
// or shows the download UI if no stable release is found.
func InstallDefault(w fyne.Window) {
	// If we haven't fetched releases yet, try to fetch them now so the
	// Install button can auto-select the latest stable release.
	if len(allReleases) == 0 {
		if r, err := fetchReleases(); err == nil {
			allReleases = r
		} else {
			log.Printf("Tracker InstallDefault: background fetchReleases failed: %v", err)
		}
	}

	latest, err := getMostRecentStableRelease()
	log.Printf("Tracker InstallDefault: getMostRecentStableRelease -> latest=%q err=%v", latest, err)
	if err == nil && latest != "" {
		confirmAndDownloadVersion(latest, w)
	} else {
		log.Println("Tracker InstallDefault: no latest stable found, showing download UI")
		ShowDownloadables()
	}
}

func checkForNewerVersion() {
	latestInstalled := findLatestInstalled()
	updateExplanation()

	if latestInstalled != "" {
		latestInstalledVersion, err := shared.NewVersionForComparison(latestInstalled)
		if err == nil {
			log.Printf("Tracker - Latest installed version: %s\n", latestInstalled)

			// Check for newer versions (both stable and prerelease)
			for _, release := range allReleases {
				releaseVersion, err := shared.NewVersionForComparison(release)
				if err == nil && releaseVersion.GreaterThan(latestInstalledVersion) {
					log.Printf("Tracker - Found newer version: %s\n", release)
					releaseURL := fmt.Sprintf("https://github.com/owlcms/owlcms-tracker/releases/tag/%s", release)

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
						versionToInstall := release
						if versionToInstall == "" {
							return
						}
						confirmAndDownloadVersion(versionToInstall, mainWindow)
					}

					messageBox := shared.CreateUpdateNotification(versionType, releaseVersion.String(), installLink, releaseNotesLink)
					updateTitleContainer.Objects = []fyne.CanvasObject{messageBox}
					updateTitleContainer.Refresh()
					updateTitleContainer.Show()
					return
				}
			}

			// Show message with release notes link (no newer version found)
			releaseURL := fmt.Sprintf("https://github.com/owlcms/owlcms-tracker/releases/tag/%s", latestInstalled)
			parsedURL, _ := url.Parse(releaseURL)
			releaseNotesLink := widget.NewHyperlink("Release Notes", parsedURL)
			// Ensure hyperlink visible for installed prerelease/stable
			releaseNotesLink.Show()
			// Log what we think is installed for debugging
			log.Printf("Tracker:updateTitle - latestInstalled=%q installedVersions=%v", latestInstalled, getAllInstalledVersions())
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
		}
	} else {
		messageBox := container.NewHBox(
			widget.NewLabel("No version installed. Select a version to download below."),
		)
		updateTitleContainer.Objects = []fyne.CanvasObject{messageBox}
		updateTitleContainer.Refresh()
		updateTitleContainer.Show()
	}
}

func findLatestInstalled() string {
	trackerDir := installDir
	entries, err := os.ReadDir(trackerDir)
	if err != nil {
		return ""
	}

	var versions []*semver.Version
	for _, entry := range entries {
		if entry.IsDir() {
			v, err := semver.NewVersion(entry.Name())
			if err == nil {
				versions = append(versions, v)
			}
		}
	}

	if len(versions) == 0 {
		return ""
	}

	sort.Sort(sort.Reverse(semver.Collection(versions)))
	return versions[0].String()
}

func updateExplanation() {
	if len(allReleases) == 0 {
		if !downloadsShown {
			return
		}
		downloadContainer.Objects = []fyne.CanvasObject{
			widget.NewLabel("You are not connected to the Internet. Available updates cannot be shown."),
		}
		downloadContainer.Show()
		downloadContainer.Refresh()
		return
	}
	log.Printf("len(allReleases) = %d\n", len(allReleases))
	x := getAllInstalledVersions()
	log.Printf("Tracker - Updating explanation %d\n", len(x))
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
				singleOrMultiVersionLabel.SetText("Use the Update button above to install the latest version.")
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
				singleOrMultiVersionLabel.SetText("Use the Update button above to install the latest version.")
			}
		}
	} else {
		singleOrMultiVersionLabel.SetText("You have several versions installed.")
	}
	singleOrMultiVersionLabel.Wrapping = fyne.TextWrapWord
	singleOrMultiVersionLabel.Show()
	singleOrMultiVersionLabel.Refresh()
}

func stripVersionMetadata(version string) string {
	v, err := semver.NewVersion(version)
	if err != nil {
		return version
	}
	if v.Prerelease() != "" {
		return fmt.Sprintf("%d.%d.%d-%s", v.Major(), v.Minor(), v.Patch(), v.Prerelease())
	}
	return fmt.Sprintf("%d.%d.%d", v.Major(), v.Minor(), v.Patch())
}

func setupDownloadContainer() {
	if len(allReleases) > 0 {
		downloadContainer.Objects = []fyne.CanvasObject{
			updateTitle,
			singleOrMultiVersionLabel,
			downloadButtonTitle,
			releaseDropdown,
		}
	} else {
		downloadContainer.Objects = []fyne.CanvasObject{
			widget.NewLabel("You are not connected to the Internet. Available updates cannot be shown."),
		}
	}

	updateTitle.Show()
	releaseDropdown.Hide()
	prereleaseCheckbox.Hide()
	if downloadContainer != nil {
		downloadContainer.Refresh()
	}
}
