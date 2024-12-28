package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"owlcms-launcher/downloadUtils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/Masterminds/semver/v3"
)

type Release struct {
	Name string `json:"name"`
}

var (
	showPrereleases bool = false
	allReleases     []string
	downloadTitle   *widget.Label
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

func createReleaseDropdown(w fyne.Window, downloadGroup *fyne.Container) *widget.Select {
	releaseDropdown := widget.NewSelect([]string{}, func(selected string) {
		var urlPrefix string
		if containsPreReleaseTag(selected) {
			urlPrefix = "https://github.com/owlcms/owlcms4-prerelease/releases/download"
		} else {
			urlPrefix = "https://github.com/owlcms/owlcms4/releases/download"
		}
		fileName := fmt.Sprintf("owlcms_%s.zip", selected)
		zipURL := fmt.Sprintf("%s/%s/%s", urlPrefix, selected, fileName)
		zipPath := fileName
		extractPath := selected

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

					// Reinitialize the version list
					fmt.Println("Reinitializing version list")
					versionContainer.Objects = nil // Clear the container
					newVersionList := createVersionList(w, stopButton, downloadGroup, versionContainer)

					// Update the scroll container's size
					numVersions := len(getAllInstalledVersions())
					minHeight := 50 // minimum height
					rowHeight := 40 // approximate height per row
					height := minHeight + (rowHeight * min(numVersions, 4))
					versionScroll := container.NewVScroll(newVersionList)
					versionScroll.SetMinSize(fyne.NewSize(400, float32(height)))
					versionContainer.Add(versionScroll)

					fmt.Println("Version list reinitialized")

					// Recompute the downloadTitle
					checkForNewerVersion()
				}()
			},
			w)
	})
	releaseDropdown.PlaceHolder = "Choose a release to download"

	downloadGroup.Objects = []fyne.CanvasObject{
		downloadTitle,
		widget.NewLabel("Download and install a new version of OWLCMS from GitHub:"),
		releaseDropdown,
	}

	populateReleaseDropdown(releaseDropdown)

	return releaseDropdown
}

func containsPreReleaseTag(version string) bool {
	return strings.Contains(version, "-rc") || strings.Contains(version, "-alpha") || strings.Contains(version, "-beta")
}
