package owlcms

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"owlcms-launcher/owlcms/downloadutils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/Masterminds/semver/v3"
)

// Release represents a GitHub release
type Release struct {
	Name string `json:"name"`
}

var (
	showPrereleases      bool = false
	allReleases          []string
	releaseDropdown      *fyne.Container
	prereleaseCheckbox   *widget.Check
	updateTitle          *widget.RichText
	downloadButtonTitle  *widget.Hyperlink
	updateTitleContainer *fyne.Container
	installAvailableLink *widget.Hyperlink
	releaseNotesLink     *widget.Hyperlink
	availableVersion     string
	availableVersionURL  string
)

func fetchReleases() ([]string, error) {
	urls := []string{
		"https://api.github.com/repos/owlcms/owlcms4-prerelease/releases",
		"https://api.github.com/repos/owlcms/owlcms4/releases",
	}

	var allReleasesList []Release
	client := &http.Client{
		Timeout: 5 * time.Second,
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

		allReleasesList = append(allReleasesList, releases...)
	}

	if len(allReleasesList) == 0 {
		return nil, fmt.Errorf("no releases found")
	}

	releaseNames := make([]string, 0, len(allReleasesList))
	for _, release := range allReleasesList {
		releaseNames = append(releaseNames, release.Name)
	}

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

	owlcmsDir := installDir
	if _, err := os.Stat(owlcmsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(owlcmsDir, 0755); err != nil {
			dialog.ShowError(fmt.Errorf("creating owlcms directory: %w", err), w)
			return
		}
	}

	zipPath := filepath.Join(owlcmsDir, fileName)
	extractPath := filepath.Join(owlcmsDir, version)

	progressBar := widget.NewProgressBar()
	messageLabel := widget.NewLabel(fmt.Sprintf("Downloading OWLCMS %s...", version))
	progressContent := container.NewVBox(messageLabel, progressBar)
	progressDialog := dialog.NewCustom(
		"Installing OWLCMS",
		"Please wait...",
		progressContent,
		w)
	progressDialog.Show()

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

		progressCallback := func(downloaded, total int64) {
			if total > 0 {
				progress := float64(downloaded) / float64(total)
				progressBar.SetValue(progress)
				messageLabel.SetText(fmt.Sprintf("Downloading OWLCMS %s... %.1f%%", version, progress*100))
				messageLabel.Refresh()
			}
		}

		err := downloadutils.DownloadArchive(zipURL, zipPath, progressCallback, nil)
		if err != nil {
			dialog.ShowError(fmt.Errorf("download failed: %w", err), w)
			done <- false
			return
		}

		messageLabel.SetText("Extracting files...")
		messageLabel.Refresh()

		err = downloadutils.ExtractZip(zipPath, extractPath)
		if err != nil {
			dialog.ShowError(fmt.Errorf("extraction failed: %w", err), w)
			done <- false
			return
		}

		if err := os.Remove(zipPath); err != nil {
			log.Printf("Warning: Could not delete downloaded zip file: %v", err)
		}

		message := fmt.Sprintf(
			"Successfully installed OWLCMS version %s\n\n"+
				"Location: %s\n\n"+
				"The program files have been extracted to the above directory.",
			version, extractPath)

		dialog.ShowInformation("Installation Complete", message, w)

		if isInitialDownload {
			recomputeVersionList(w)
			setupReleaseDropdown(w)
			checkForNewerVersion()

			stopContainer.Hide()
			versionContainer.Show()
			downloadContainer.Show()
			statusLabel.Hide()

			downloadButtonTitle.Show()
			updateTitleContainer.Show()
		} else {
			HideDownloadables()
			recomputeVersionList(w)
			checkForNewerVersion()
		}

		w.Content().Refresh()
		done <- true
	}()

	<-done
}

func confirmAndDownloadVersion(version string, w fyne.Window) {
	dialog.ShowConfirm("Confirm Download",
		fmt.Sprintf("Do you want to download and install OWLCMS version %s?", version),
		func(ok bool) {
			if ok {
				downloadReleaseWithProgress(version, w, false)
			}
		},
		w)
}

func createReleaseDropdown(w fyne.Window) (*widget.Select, *fyne.Container) {
	selectWidget := widget.NewSelect([]string{}, func(selected string) {
		confirmAndDownloadVersion(selected, w)
	})
	selectWidget.PlaceHolder = "Choose a release to download"

	// Create prerelease checkbox if not already created
	if prereleaseCheckbox == nil {
		prereleaseCheckbox = widget.NewCheck("Show Prereleases", func(checked bool) {
			showPrereleases = checked
			populateReleaseSelect(selectWidget)
		})
	}
	prereleaseCheckbox.Hide()

	populateReleaseSelect(selectWidget)
	horiz := container.New(layout.NewHBoxLayout(), selectWidget, prereleaseCheckbox)
	releaseDropdown = horiz
	releaseDropdown.Resize(fyne.NewSize(300, 200))

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
			updateTitleContainer,
			singleOrMultiVersionLabel,
			downloadButtonTitle,
			dropdownContainer,
		}
	} else {
		downloadContainer.Objects = []fyne.CanvasObject{
			widget.NewLabel("You are not connected to the Internet. Available updates cannot be shown."),
		}
	}
	populateReleaseSelect(selectWidget)
	downloadContainer.Refresh()
}
