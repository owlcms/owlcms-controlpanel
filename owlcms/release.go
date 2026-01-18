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

	"owlcms-launcher/shared"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/Masterminds/semver/v3"
)

// Release represents a GitHub release
type Release struct {
	// GitHub Releases API fields
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
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
	fetchFromURL := func(url string) ([]Release, error) {
		client := &http.Client{Timeout: 5 * time.Second}
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
		return releases, nil
	}

	// Always fetch stable releases. Fetch prereleases when the checkbox is selected
	// OR when a prerelease is already installed (so update checks keep working).
	needPrereleases := showPrereleases
	for _, v := range getAllInstalledVersions() {
		if containsPreReleaseTag(v) {
			needPrereleases = true
			break
		}
	}

	stableURL := "https://api.github.com/repos/owlcms/owlcms4/releases"
	preURL := "https://api.github.com/repos/owlcms/owlcms4-prerelease/releases"

	stable, err := fetchFromURL(stableURL)
	if err != nil {
		return nil, err
	}
	all := append([]Release{}, stable...)
	if needPrereleases {
		pre, err := fetchFromURL(preURL)
		if err != nil {
			return nil, err
		}
		all = append(all, pre...)
	}

	if len(all) == 0 {
		return nil, fmt.Errorf("no releases found")
	}

	// Prefer tag_name (always machine-readable), fall back to name.
	seen := map[string]struct{}{}
	releaseNames := make([]string, 0, len(all))
	for _, r := range all {
		v := strings.TrimSpace(r.TagName)
		if v == "" {
			v = strings.TrimSpace(r.Name)
		}
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		releaseNames = append(releaseNames, v)
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

func downloadReleaseWithProgress(downloadVersion, installVersion string, w fyne.Window, isInitialDownload bool) {
	var urlPrefix string
	if containsPreReleaseTag(downloadVersion) {
		urlPrefix = "https://github.com/owlcms/owlcms4-prerelease/releases/download"
	} else {
		urlPrefix = "https://github.com/owlcms/owlcms4/releases/download"
	}
	fileName := fmt.Sprintf("owlcms_%s.zip", downloadVersion)
	zipURL := fmt.Sprintf("%s/%s/%s", urlPrefix, downloadVersion, fileName)

	owlcmsDir := installDir
	if _, err := os.Stat(owlcmsDir); os.IsNotExist(err) {
		if err := shared.EnsureDir0755(owlcmsDir); err != nil {
			dialog.ShowError(fmt.Errorf("creating owlcms directory: %w", err), w)
			return
		}
	}

	zipPath := filepath.Join(owlcmsDir, fileName)
	extractPath := filepath.Join(owlcmsDir, installVersion)

	progressBar := widget.NewProgressBar()
	messageLabel := widget.NewLabel(fmt.Sprintf("Downloading OWLCMS %s...", downloadVersion))
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
				messageLabel.SetText(fmt.Sprintf("Downloading OWLCMS %s... %.1f%%", downloadVersion, progress*100))
				messageLabel.Refresh()
			}
		}

		err := shared.DownloadArchive(zipURL, zipPath, progressCallback, nil)
		if err != nil {
			log.Printf("Download failed: %v", err)
			dialog.ShowError(fmt.Errorf("download failed: %w", err), w)
			done <- false
			return
		}

		log.Printf("Download complete, starting extraction to %s", extractPath)
		messageLabel.SetText("Extracting files...")
		messageLabel.Refresh()

		err = shared.ExtractZip(zipPath, extractPath)
		log.Printf("ExtractZip returned, err=%v", err)
		if err != nil {
			dialog.ShowError(fmt.Errorf("extraction failed: %w", err), w)
			done <- false
			return
		}

		if err := normalizeExtractedDir(extractPath); err != nil {
			dialog.ShowError(fmt.Errorf("failed to finalize install directory: %w", err), w)
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
			installVersion, extractPath)

		log.Printf("Download and extraction complete for version %s, showing dialog", installVersion)
		dialog.ShowInformation("Installation Complete", message, w)

		_ = isInitialDownload // keep parameter stable even if caller distinguishes paths
		HideDownloadables()
		setOwlcmsTabMode(w)

		log.Printf("Refreshing window content")
		w.Content().Refresh()
		done <- true
	}()

	<-done
}

// normalizeExtractedDir flattens a single top-level directory if the archive contained one.
// This keeps the install directory consistent even when installing under a renamed folder.
func normalizeExtractedDir(extractPath string) error {
	entries, err := os.ReadDir(extractPath)
	if err != nil {
		return err
	}

	var subdirs []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			subdirs = append(subdirs, entry)
			continue
		}
		// If any file exists at the top level, assume layout is already correct.
		return nil
	}

	if len(subdirs) != 1 {
		return nil
	}

	childPath := filepath.Join(extractPath, subdirs[0].Name())
	childEntries, err := os.ReadDir(childPath)
	if err != nil {
		return err
	}

	for _, entry := range childEntries {
		src := filepath.Join(childPath, entry.Name())
		dst := filepath.Join(extractPath, entry.Name())
		if _, statErr := os.Stat(dst); statErr == nil {
			return fmt.Errorf("destination already exists: %s", dst)
		}
		if err := os.Rename(src, dst); err != nil {
			return err
		}
	}

	return os.Remove(childPath)
}

// GetReleaseTagURL returns the GitHub releases/tag URL for the given version string,
// using the prerelease repository when the version indicates a prerelease.
func GetReleaseTagURL(version string) string {
	if containsPreReleaseTag(version) {
		return fmt.Sprintf("https://github.com/owlcms/owlcms4-prerelease/releases/tag/%s", version)
	}
	return fmt.Sprintf("https://github.com/owlcms/owlcms4/releases/tag/%s", version)
}

func confirmAndDownloadVersion(version string, w fyne.Window) {
	dialog.ShowConfirm("Confirm Download",
		fmt.Sprintf("Do you want to download and install OWLCMS version %s?", version),
		func(ok bool) {
			if ok {
				shared.PromptForInstallVersionName(installDir, version, w, func(newVersion string) {
					downloadReleaseWithProgress(version, newVersion, w, false)
				})
			}
		},
		w)
}

func createReleaseDropdown(w fyne.Window) (*widget.Select, *fyne.Container) {
	selectWidget := widget.NewSelect([]string{}, func(selected string) {
		confirmAndDownloadVersion(selected, w)
	})
	selectWidget.PlaceHolder = "Choose a release to download"

	// IMPORTANT: The release dropdown is rebuilt at runtime (e.g. via setOwlcmsTabModeInstalled).
	// If the checkbox were a singleton, its callback could capture an old select widget.
	// Always create a new checkbox bound to THIS select widget.
	prereleaseCheckbox = widget.NewCheck("Show Prereleases", func(checked bool) {
		showPrereleases = checked
		// Refetch so OWLCMS can include the separate prerelease repository when enabled.
		go func() {
			releases, err := fetchReleases()
			if err != nil {
				log.Printf("failed to fetch releases after prerelease toggle: %v", err)
				return
			}
			// Fyne widget methods are thread-safe for basic updates.
			allReleases = releases
			populateReleaseSelect(selectWidget)
			checkForNewerVersion()
			// Keep the dropdown visible while the user is toggling prereleases.
			ShowDownloadables()
			if downloadContainer != nil {
				downloadContainer.Refresh()
			}
		}()
	})
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

// InstallDefault performs the default install action for the OWLCMS package.
// It selects the most recent stable release and begins the confirm+download flow.
func InstallDefault(w fyne.Window) {
	// Before installing, ensure Java runtime is available. Delegate to shared helper
	// which will call the package-local `checkJava` implementation.
	ver := GetTemurinVersion()
	if err := shared.CheckAndInstallJava(ver, statusLabel, w, checkJava); err != nil {
		return
	}

	// Fetch releases on-demand if not already loaded
	if len(allReleases) == 0 {
		if r, err := fetchReleases(); err == nil {
			allReleases = r
		} else {
			log.Printf("OWLCMS InstallDefault: background fetchReleases failed: %v", err)
		}
	}

	latest, err := getMostRecentStableRelease()
	log.Printf("OWLCMS InstallDefault: getMostRecentStableRelease -> latest=%q err=%v", latest, err)
	if err == nil && latest != "" {
		confirmAndDownloadVersion(latest, w)
	} else {
		log.Println("OWLCMS InstallDefault: no latest stable found, showing download UI")
		ShowDownloadables()
	}
}

func setupReleaseDropdown(w fyne.Window) {
	// Fetch releases on-demand if not already loaded
	if len(allReleases) == 0 {
		if r, err := fetchReleases(); err == nil {
			allReleases = r
		} else {
			log.Printf("OWLCMS setupReleaseDropdown: fetchReleases failed: %v", err)
		}
	}

	selectWidget, dropdownContainer := createReleaseDropdown(w)
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
