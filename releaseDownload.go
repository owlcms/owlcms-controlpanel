package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"owlcms-launcher/downloadUtils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type Release struct {
	Name string `json:"name"`
}

func fetchReleases() ([]string, error) {
	url := "https://api.github.com/repos/owlcms/owlcms4-prerelease/releases"
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

	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}

	releaseNames := make([]string, 0, 10)
	for i, release := range releases {
		if i >= 10 {
			break
		}
		releaseNames = append(releaseNames, release.Name)
	}

	return releaseNames, nil
}

func createReleaseDropdown(w fyne.Window, downloadGroup *fyne.Container) *widget.Select {
	releaseDropdown := widget.NewSelect([]string{}, func(selected string) {
		urlPrefix := "https://github.com/owlcms/owlcms4-prerelease/releases/download"
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
				}()
			},
			w)
	})
	releaseDropdown.PlaceHolder = "Choose a release to download"

	downloadGroup.Objects = []fyne.CanvasObject{
		widget.NewLabelWithStyle("Download New Version", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Download and install a new version of OWLCMS from GitHub:"),
		releaseDropdown,
	}

	return releaseDropdown
}

func fetchReleasesInBackground(releasesChan chan<- []string, errChan chan<- error) {
	releases, err := fetchReleases()
	if err != nil {
		errChan <- err
		return
	}
	releasesChan <- releases
}
