package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"owlcms-launcher/downloadUtils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/Masterminds/semver/v3"
)

type Release struct {
	Name string `json:"name"`
}

var (
	showPrereleases     bool = false
	allReleases         []string
	downloadButton      *widget.Button
	releaseDropdown     *widget.Select
	prereleaseCheckbox  *widget.Check
	updateButton        *widget.Button // New update button
	updateTitle         *widget.Label
	downloadButtonTitle *widget.Label // New title for download button
)

func fetchReleases() ([]string, error) {
	urls := []string{
		"https://api.github.com/repos/owlcms/owlcms4-prerelease/releases",
		"https://api.github.com/repos/owlcms/owlcms4/releases",
	}

	var allReleases []Release
	for _, url := range urls {
		resp, err := http.Get(url)
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
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	if err := cmd.Start(); err != nil {
		fmt.Printf("Failed to open file explorer: %v\n", err)
		return fmt.Errorf("failed to open file explorer: %w", err)
	}

	return nil
}

func populateReleaseDropdown(releaseDropdown *widget.Select) {
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
	releaseDropdown.Options = filteredReleases
	releaseDropdown.Refresh()
}

func createReleaseDropdown(w fyne.Window) *widget.Select {
	releaseDropdown = widget.NewSelect([]string{}, func(selected string) {
		var urlPrefix string
		if containsPreReleaseTag(selected) {
			urlPrefix = "https://github.com/owlcms/owlcms4-prerelease/releases/download"
		} else {
			urlPrefix = "https://github.com/owlcms/owlcms4/releases/download"
		}
		fileName := fmt.Sprintf("owlcms_%s.zip", selected)
		zipURL := fmt.Sprintf("%s/%s/%s", urlPrefix, selected, fileName)

		// Ensure the owlcms directory exists
		owlcmsDir := owlcmsInstallDir
		if _, err := os.Stat(owlcmsDir); os.IsNotExist(err) {
			if err := os.Mkdir(owlcmsDir, 0755); err != nil {
				dialog.ShowError(fmt.Errorf("creating owlcms directory: %w", err), w)
				return
			}
		}

		zipPath := filepath.Join(owlcmsDir, fileName)
		extractPath := filepath.Join(owlcmsDir, selected)

		dialog.ShowConfirm("Confirm Download",
			fmt.Sprintf("Do you want to download and install OWLCMS version %s?", selected),
			func(ok bool) {
				if !ok {
					return
				}

				// Show progress dialog
				progressDialog := dialog.NewCustom(
					"Installing OWLCMS",
					"Please wait...",
					widget.NewTextGridFromString("Downloading and extracting files..."),
					w)
				progressDialog.Show()

				go func() {
					// Download the ZIP file using downloadUtils
					fmt.Printf("Starting download from URL: %s\n", zipURL)
					err := downloadUtils.DownloadZip(zipURL, zipPath)
					if err != nil {
						progressDialog.Hide()
						dialog.ShowError(fmt.Errorf("download failed: %w", err), w)
						return
					}

					// Extract the ZIP file to version-specific subdirectory
					fmt.Printf("Extracting ZIP file to: %s\n", extractPath)
					err = downloadUtils.ExtractZip(zipPath, extractPath)
					if err != nil {
						progressDialog.Hide()
						dialog.ShowError(fmt.Errorf("extraction failed: %w", err), w)
						return
					}

					// Log when extraction is done
					fmt.Println("Extraction completed")

					// Log before closing the dialog
					fmt.Println("Closing progress dialog")

					// Hide progress dialog
					progressDialog.Hide()

					// Show success panel with installation details
					message := fmt.Sprintf(
						"Successfully installed OWLCMS version %s\n\n"+
							"Location: %s\n\n"+
							"The program files have been extracted to the above directory.",
						selected, extractPath)

					dialog.ShowInformation("Installation Complete", message, w)

					// Recompute the version list
					recomputeVersionList(w)

					// Recompute the downloadTitle
					checkForNewerVersion()
				}()
			},
			w)
	})
	releaseDropdown.PlaceHolder = "Choose a release to download"
	releaseDropdown.Hide() // Hide the dropdown initially
	populateReleaseDropdown(releaseDropdown)
	if prereleaseCheckbox != nil {
		prereleaseCheckbox.Hide()
	}

	return releaseDropdown
}

func containsPreReleaseTag(version string) bool {
	return strings.Contains(version, "-rc") || strings.Contains(version, "-alpha") || strings.Contains(version, "-beta")
}

func checkForNewerVersion() {
	latestInstalled := findLatestInstalled()
	downloadButtonTitle.SetText("You may install additional versions if you wish.")
	if latestInstalled != "" {
		latestStable, _ := semver.NewVersion("0.0.0")
		latestInstalledVersion, err := semver.NewVersion(latestInstalled)
		if err == nil {
			fmt.Printf("Latest installed version: %s\n", latestInstalledVersion)
			for _, release := range allReleases {
				releaseVersion, err := semver.NewVersion(release)
				if err == nil {
					if releaseVersion.GreaterThan(latestInstalledVersion) {
						fmt.Printf("Found newer version: %s\n", releaseVersion)
						if containsPreReleaseTag(release) {
							fmt.Printf("Newer version is a pre-release: %s\n", release)
							if containsPreReleaseTag(latestInstalled) {
								updateTitle.SetText(fmt.Sprintf("A more recent prerelease version (%s) is available", releaseVersion))
								updateTitle.TextStyle = fyne.TextStyle{Bold: true}
								updateTitle.Refresh()
								updateTitle.Show()
								return
							} else {
								fmt.Printf("Skipping pre-release version: %s\n", release)
							}
						} else {
							updateTitle.SetText(fmt.Sprintf("A more recent stable version (%s) is available", releaseVersion))
							updateTitle.TextStyle = fyne.TextStyle{Bold: true}
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
			updateButton.Show()
			downloadButtonTitle.Show()
			downloadButton.Show()

			if containsPreReleaseTag(latestInstalled) {
				updateTitle.SetText(fmt.Sprintf("The latest installed version is a pre-release; the latest stable version is %s", latestStable))
			} else {
				updateTitle.SetText("The latest stable version is installed.")
				updateButton.Hide()
			}
			updateTitle.TextStyle = fyne.TextStyle{Bold: false}
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
		updateTitle.SetText("No version installed. Select a version to download below.")
		updateTitle.TextStyle = fyne.TextStyle{Bold: true}
		updateTitle.Refresh()
		updateTitle.Show()
	}
}
