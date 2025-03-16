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
	"runtime"
	"sort"
	"strings"
	"time"

	"owlcms-launcher/downloadUtils"

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
		"https://api.github.com/repos/owlcms/owlcms4-prerelease/releases",
		"https://api.github.com/repos/owlcms/owlcms4/releases",
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

func downloadReleaseWithProgress(version string, w fyne.Window, isInitialDownload bool) {
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
			dialog.ShowError(fmt.Errorf("creating owlcms directory: %w", err), w)
			return
		}
	}

	zipPath := filepath.Join(owlcmsDir, fileName)
	extractPath := filepath.Join(owlcmsDir, version)

	// Create progress dialog with progress bar
	progressBar := widget.NewProgressBar()
	messageLabel := widget.NewLabel(fmt.Sprintf("Downloading OWLCMS %s...", version))
	progressContent := container.NewVBox(messageLabel, progressBar)
	progressDialog := dialog.NewCustom(
		"Installing OWLCMS",
		"Please wait...",
		progressContent,
		w)
	progressDialog.Show()

	// Create a channel to wait for download completion
	done := make(chan bool)

	go func() {
		var dialogClosed bool
		closeDialog := func() {
			if !dialogClosed {
				progressDialog.Hide()
				dialogClosed = true
			}
		}
		defer closeDialog()

		// Download with progress tracking
		progressCallback := func(downloaded, total int64) {
			if total > 0 {
				progress := float64(downloaded) / float64(total)
				progressBar.SetValue(progress)
				messageLabel.SetText(fmt.Sprintf("Downloading OWLCMS %s... %.1f%%", version, progress*100))
				messageLabel.Refresh()
			}
		}

		// Fix: Add nil as the last argument for the cancel channel
		err := downloadUtils.DownloadArchive(zipURL, zipPath, progressCallback, nil)
		if err != nil {
			dialog.ShowError(fmt.Errorf("download failed: %w", err), w)
			done <- false
			return
		}

		messageLabel.SetText("Extracting files...")
		messageLabel.Refresh()

		err = downloadUtils.ExtractZip(zipPath, extractPath)
		if err != nil {
			dialog.ShowError(fmt.Errorf("extraction failed: %w", err), w)
			done <- false
			return
		}

		message := fmt.Sprintf(
			"Successfully installed OWLCMS version %s\n\n"+
				"Location: %s\n\n"+
				"The program files have been extracted to the above directory.",
			version, extractPath)

		dialog.ShowInformation("Installation Complete", message, w)

		// Initialize UI after installation
		if isInitialDownload {
			// Initialize version list and release dropdown first
			recomputeVersionList(w)
			setupReleaseDropdown(w)
			checkForNewerVersion()

			// Show necessary containers
			stopContainer.Hide()
			versionContainer.Show()
			downloadContainer.Show()
			statusLabel.Hide()

			// Show download button and update title
			downloadButtonTitle.Show()
			updateTitle.Show()
		} else {
			HideDownloadables()
			recomputeVersionList(w)
			checkForNewerVersion()
		}

		w.Content().Refresh()
		done <- true
	}()

	// Wait for download completion
	<-done
}

func createReleaseDropdown(w fyne.Window) (*widget.Select, *fyne.Container) {
	selectWidget := widget.NewSelect([]string{}, func(selected string) {
		dialog.ShowConfirm("Confirm Download",
			fmt.Sprintf("Do you want to download and install OWLCMS version %s?", selected),
			func(ok bool) {
				if ok {
					downloadReleaseWithProgress(selected, w, false)
				}
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
		releaseVersion, err := semver.NewVersion(release)
		if err != nil {
			continue
		}
		if !containsPreReleaseTag(release) {
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
		releaseVersion, err := semver.NewVersion(release)
		if err != nil {
			continue
		}
		if containsPreReleaseTag(release) {
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

func setupReleaseDropdown(w fyne.Window) {
	selectWidget, dropdownContainer := createReleaseDropdown(w)
	if len(allReleases) > 0 {
		downloadContainer.Objects = []fyne.CanvasObject{
			updateTitle,
			singleOrMultiVersionLabel,
			downloadButtonTitle,
			dropdownContainer,
			prereleaseCheckbox,
		}
	} else {
		downloadContainer.Objects = []fyne.CanvasObject{
			widget.NewLabel("You are not connected to the Internet. Available updates cannot be shown."),
		}
	}
	populateReleaseSelect(selectWidget)
	downloadContainer.Refresh()
}
