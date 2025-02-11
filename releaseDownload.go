package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"firmata-launcher/downloadUtils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/Masterminds/semver/v3"
)

type Release struct {
	Name string `json:"name"`
}

var (
	showPrereleases     bool = false
	allReleases         []string
	releaseDropdown     *fyne.Container
	prereleaseCheckbox  *widget.Check
	updateTitle         *widget.RichText  // Change to RichText for Markdown support
	downloadButtonTitle *widget.Hyperlink // New title for download button
)

func fetchReleases() ([]string, error) {
	urls := []string{
		"https://api.github.com/repos/jflamy/owlcms-firmata/releases",
	}

	var allReleases []Release
	client := &http.Client{
		Timeout: 5 * time.Second, // Set a timeout for the HTTP request
	}

	for _, url := range urls {
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

		allReleases = append(allReleases, releases...)
	}

	if len(allReleases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}

	releaseNames := make([]string, 0, len(allReleases))
	for _, release := range allReleases {
		releaseNames = append(releaseNames, release.Name)
	}

	// Sort the release names in semver order, most recent at the top
	sort.Slice(releaseNames, func(i, j int) bool {
		v1, err1 := semver.NewVersion(releaseNames[i])
		v2, err2 := semver.NewVersion(releaseNames[j])
		if err1 != nil || err2 != nil {
			return releaseNames[i] > releaseNames[j]
		}
		return v1.GreaterThan(v2)
	})

	// After sorting releases but before returning, log the latest versions
	var latestStable, latestPrerelease string
	for _, release := range releaseNames {
		version := extractSemverTag(release)
		if !containsPreReleaseTag(version) {
			latestStable = version
			break // First one is latest since they're sorted
		}
	}
	for _, release := range releaseNames {
		version := extractSemverTag(release)
		if containsPreReleaseTag(version) {
			latestPrerelease = version
			break // First one is latest since they're sorted
		}
	}
	log.Printf("Available from GitHub - Latest stable: %s, Latest prerelease: %s\n",
		latestStable, latestPrerelease)

	return releaseNames, nil
}

func openFileExplorer(path string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin": // macOS
		cmd = exec.Command("open", path)
	case "linux":
		cmd = exec.Command("xdg-open", path)
	default:
		return fmt.Errorf("unsupported operating system: %s", downloadUtils.GetGoos())
	}

	if err := cmd.Start(); err != nil {
		log.Printf("Failed to open file explorer: %v\n", err)
		return fmt.Errorf("failed to open file explorer: %w", err)
	}

	return nil
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

func createReleaseDropdown(w fyne.Window) (*widget.Select, *fyne.Container) {
	selectWidget := widget.NewSelect([]string{}, func(selected string) {
		// Extract clean version from selected string
		version := extractSemverTag(selected)
		var urlPrefix string
		if containsPreReleaseTag(version) {
			urlPrefix = "https://github.com/jflamy/owlcms-firmata/releases/download"
		} else {
			urlPrefix = "https://github.com/jflamy/owlcms-firmata/releases/download"
		}
		fileName := "owlcms-firmata.jar"
		zipURL := fmt.Sprintf("%s/%s/%s", urlPrefix, version, fileName)

		// Ensure the firmata directory exists
		owlcmsDir := owlcmsInstallDir
		if _, err := os.Stat(owlcmsDir); os.IsNotExist(err) {
			if err := os.MkdirAll(owlcmsDir, 0755); err != nil {
				dialog.ShowError(fmt.Errorf("creating firmata directory: %w", err), w)
				return
			}
		}

		// zipPath := filepath.Join(owlcmsDir, fileName)

		dialog.ShowConfirm("Confirm Download",
			fmt.Sprintf("Do you want to download and install owlcms-firmata version %s?", selected),
			func(ok bool) {
				if !ok {
					return
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

					// Download the ZIP file using downloadUtils
					log.Printf("Starting download from URL: %s\n", zipURL)
					err := downloadUtils.DownloadArchive(zipURL, extractPath)
					if err != nil {
						progressDialog.Hide()
						dialog.ShowError(fmt.Errorf("download failed: %w", err), w)
						return
					}

					// // Extract the ZIP file to version-specific subdirectory
					// log.Printf("Extracting ZIP file to: %s\n", extractPath)
					// err = downloadUtils.ExtractZip(zipPath, extractPath)
					// if err != nil {
					// 	progressDialog.Hide()
					// 	dialog.ShowError(fmt.Errorf("extraction failed: %w", err), w)
					// 	return
					// }

					// Log when extraction is done
					log.Println("Extraction completed")

					// Log before closing the dialog
					log.Println("Closing progress dialog")

					// Hide progress dialog
					progressDialog.Hide()

					// Show success panel with installation details
					message := fmt.Sprintf(
						"Successfully installed owlcms-firmata version %s\n\n"+
							"Location: %s\n\n"+
							"The program files have been extracted to the above directory.",
						selected, extractPath)

					dialog.ShowInformation("Installation Complete", message, w)
					HideDownloadables()

					// Recompute the version list
					recomputeVersionList(w)

					// Recompute the downloadTitle
					latestInstalled = findLatestInstalled()
					checkForNewerVersion()
				}()
			},
			w)
	})
	selectWidget.PlaceHolder = "Choose a release to download"
	populateReleaseSelect(selectWidget)
	if prereleaseCheckbox != nil {
		prereleaseCheckbox.Hide()
	}
	releaseDropdown = container.New(layout.NewHBoxLayout(), selectWidget)
	releaseDropdown.Resize(fyne.NewSize(200, 200))

	return selectWidget, releaseDropdown
}

func containsPreReleaseTag(version string) bool {
	return strings.Contains(version, "-rc") || strings.Contains(version, "-alpha") || strings.Contains(version, "-beta")
}

func getMostRecentStableRelease() (string, error) {
	var mostRecentStable *semver.Version
	for _, release := range allReleases {
		version := extractSemverTag(release) // Clean the version string first
		releaseVersion, err := semver.NewVersion(version)
		if err != nil {
			continue
		}
		if !containsPreReleaseTag(version) {
			if mostRecentStable == nil || releaseVersion.GreaterThan(mostRecentStable) {
				mostRecentStable = releaseVersion
			}
		}
	}
	if mostRecentStable == nil {
		return "", fmt.Errorf("no stable release found")
	}
	return mostRecentStable.String(), nil
}

func getMostRecentPrerelease() (string, error) {
	var mostRecentPrerelease *semver.Version
	for _, release := range allReleases {
		version := extractSemverTag(release) // Clean the version string first
		releaseVersion, err := semver.NewVersion(version)
		if err != nil {
			continue
		}
		if containsPreReleaseTag(version) {
			if mostRecentPrerelease == nil || releaseVersion.GreaterThan(mostRecentPrerelease) {
				mostRecentPrerelease = releaseVersion
			}
		}
	}
	if mostRecentPrerelease == nil {
		return "", fmt.Errorf("no prerelease found")
	}
	return mostRecentPrerelease.String(), nil
}

func extractSemverTag(s string) string {
	re := regexp.MustCompile(`(v?\d+\.\d+\.\d+(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+)?)$`)
	if match := re.FindString(s); match != "" {
		if match[0] == 'v' {
			return match[1:]
		}
		return match
	}
	return s
}

// ...existing code...
