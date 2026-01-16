package tracker

import (
	"fmt"
	"image/color"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"owlcms-launcher/shared"
	"owlcms-launcher/tracker/downloadutils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/Masterminds/semver/v3"
)

var (
	versionList     *widget.List
	latestInstalled string
)

func getAllInstalledVersions() []string {
	trackerDir := installDir
	entries, err := os.ReadDir(trackerDir)
	if err != nil {
		return nil
	}

	type versionWithMeta struct {
		semver   *semver.Version
		original string
	}

	var versions []versionWithMeta
	for _, entry := range entries {
		if entry.IsDir() {
			// Try to parse the directory name, stripping build metadata for comparison
			baseVersion, _ := shared.ParseVersionWithBuild(entry.Name())
			v, err := semver.NewVersion(baseVersion)
			if err == nil {
				versions = append(versions, versionWithMeta{
					semver:   v,
					original: entry.Name(),
				})
			}
		}
	}

	// Sort by semver (build metadata is ignored in comparison)
	sort.Slice(versions, func(i, j int) bool {
		return versions[j].semver.LessThan(versions[i].semver)
	})

	var versionStrings []string
	for _, v := range versions {
		versionStrings = append(versionStrings, v.original)
	}

	return versionStrings
}

func findLatestStableInstalledVersion() string {
	var latestStable *semver.Version
	for _, dir := range getAllInstalledVersions() {
		version := extractSemverTag(dir)
		v, err := semver.NewVersion(version)
		if err == nil && !containsPreReleaseTag(version) {
			if latestStable == nil || v.GreaterThan(latestStable) {
				latestStable = v
			}
		}
	}
	if latestStable != nil {
		return latestStable.String()
	}
	return ""
}

func extractSemverTag(tag string) string {
	// Already a valid semver, just return it
	return tag
}

func findLatestPrereleaseInstalledVersion() string {
	trackerDir := installDir
	entries, err := os.ReadDir(trackerDir)
	if err != nil {
		return ""
	}

	var versions []*semver.Version
	for _, entry := range entries {
		if entry.IsDir() {
			v, err := semver.NewVersion(entry.Name())
			if err == nil && v.Prerelease() != "" {
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

func createVersionList(w fyne.Window, stopBtn *widget.Button) *widget.List {
	versions := getAllInstalledVersions()

	versionList = widget.NewList(
		func() int { return len(versions) },
		func() fyne.CanvasObject {
			// Template item for the version list
			label := widget.NewLabelWithStyle("LabelTemplate", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			launchButton := widget.NewButton("ButtonTemplate", nil)
			launchButton.Resize(fyne.NewSize(80, 25))
			launchButton.Importance = widget.HighImportance
			buttonContainer := container.NewHBox(
				container.NewPadded(launchButton),
				layout.NewSpacer(),
			)
			grid := container.New(layout.NewHBoxLayout(), container.NewGridWrap(fyne.NewSize(120, 25), label), buttonContainer)
			return grid
		},
		func(index widget.ListItemID, item fyne.CanvasObject) {
			version := versions[index]

			grid := item.(*fyne.Container)

			label := grid.Objects[0].(*fyne.Container).Objects[0].(*widget.Label)
			label.SetText(version)
			label.TextStyle = fyne.TextStyle{Bold: true}
			label.Refresh()

			buttonContainer := grid.Objects[1].(*fyne.Container)
			buttonContainer.RemoveAll()

			createLaunchButton(w, version, stopBtn, buttonContainer)
			createFilesButton(version, w, buttonContainer)
			if len(allReleases) > 0 {
				createUpdateButton(version, w, buttonContainer)
			}
			if len(versions) > 1 {
				createImportButton(versions, version, w, buttonContainer)
			}
			shared.CreateDuplicateButton(installDir, version, w, buttonContainer, func(newVersion string) {
				recomputeVersionList(w)
			})
			shared.CreateRenameButton(installDir, version, w, buttonContainer, func(newVersion string) {
				recomputeVersionList(w)
			})
			createRemoveButton(version, w, buttonContainer)
			buttonContainer.Add(layout.NewSpacer())
			buttonContainer.Refresh()
		},
	)

	versionList.OnSelected = func(id widget.ListItemID) {
		if id < len(versions) {
			log.Printf("Selected version: %s\n", versions[id])
		}
	}

	if len(versions) > 0 {
		versionList.Select(0)
	}

	if latest := findLatestInstalled(); latest != "" {
		for i, v := range versions {
			if v == latest {
				versionList.Select(i)
				break
			}
		}
	}

	return versionList
}

func createImportButton(versions []string, version string, w fyne.Window, buttonContainer *fyne.Container) {
	importButton := widget.NewButton("Import", nil)
	importButton.Show()
	importButton.OnTapped = func() {
		sourceVersions := filterVersions(versions, version)
		sourceVersionDropdown := widget.NewSelect(sourceVersions, func(selected string) {})
		dialog.ShowForm("Import Data and Config",
			"Import",
			"Cancel",
			[]*widget.FormItem{
				widget.NewFormItem("Copy the data and configurations from a previous installation", sourceVersionDropdown),
			},
			func(ok bool) {
				if !ok {
					return
				}

				sourceVersion := sourceVersionDropdown.Selected
				if sourceVersion == "" {
					dialog.ShowError(fmt.Errorf("source version cannot be empty"), w)
					return
				}

				sourceDir := filepath.Join(installDir, sourceVersion)
				destDir := filepath.Join(installDir, version)

				if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
					dialog.ShowError(fmt.Errorf("source version %s does not exist", sourceVersion), w)
					return
				}

				// Copy local files
				if err := copyFiles(filepath.Join(sourceDir, "local"), filepath.Join(destDir, "local"), true); err != nil {
					log.Printf("No local files to copy from %s\n", sourceDir)
				}

				dialog.ShowInformation("Import Complete", fmt.Sprintf("Successfully imported data and config from version %s to version %s", sourceVersion, version), w)
			},
			w)
	}
	buttonContainer.Add(container.NewPadded(importButton))
}

func createUpdateButton(version string, w fyne.Window, buttonContainer *fyne.Container) {
	updateButton := widget.NewButton("Update", nil)
	var mostRecent string
	var err error

	latestStable, stableErr := getMostRecentStableRelease()
	latestPrerelease, preErr := getMostRecentPrerelease()
	latestStableInstalled := findLatestStableInstalledVersion()
	latestPrereleaseInstalled := findLatestPrereleaseInstalledVersion()

	if (!containsPreReleaseTag(latestStableInstalled) && stableErr == nil && latestStableInstalled == latestStable) ||
		(containsPreReleaseTag(latestPrereleaseInstalled) && preErr == nil && latestPrereleaseInstalled == latestPrerelease) {
		return
	}

	if !containsPreReleaseTag(version) {
		mostRecent, err = getMostRecentStableRelease()
		if err == nil {
			adjustUpdateButton(mostRecent, version, updateButton, buttonContainer, w)
		}
	} else {
		mostRecent, err = getMostRecentPrerelease()
		if err == nil {
			adjustUpdateButton(mostRecent, version, updateButton, buttonContainer, w)
		} else {
			log.Printf("failed to get most recent prerelease: %v", err)
		}
	}
	buttonContainer.Add(container.NewPadded(updateButton))
}

func createRemoveButton(version string, w fyne.Window, buttonContainer *fyne.Container) {
	removeButton := widget.NewButton("Remove", nil)
	removeButton.OnTapped = func() {
		dialog.ShowConfirm("Confirm Remove",
			fmt.Sprintf("Do you want to remove owlcms-tracker version %s?", version),
			func(ok bool) {
				if !ok {
					return
				}

				log.Printf("Removing version %s\n", version)
				err := os.RemoveAll(filepath.Join(installDir, version))
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to remove owlcms-tracker %s: %w", version, err), w)
					return
				}

				log.Print("Reinitializing version list")
				recomputeVersionList(w)

				latestInstalled = findLatestInstalled()
				log.Printf("latestInstalled: %s\n", latestInstalled)
				checkForNewerVersion()
				downloadContainer.Refresh()
			},
			w)
	}
	buttonContainer.Add(container.NewPadded(removeButton))
}

func createFilesButton(version string, w fyne.Window, buttonContainer *fyne.Container) *widget.Button {
	filesButton := widget.NewButton("Files", nil)
	filesButton.Resize(fyne.NewSize(80, 25))
	filesButton.SetText("Files")
	filesButton.OnTapped = func() {
		versionDir := filepath.Join(installDir, version)
		if err := shared.OpenFileExplorer(versionDir); err != nil {
			dialog.ShowError(fmt.Errorf("failed to open file explorer for %s: %w", versionDir, err), w)
		}
	}
	buttonContainer.Add(container.NewPadded(filesButton))
	return filesButton
}

// NewGreenButton creates a green-styled button
func NewGreenButton(label string, tapped func()) *widget.Button {
	btn := widget.NewButton(label, tapped)
	btn.Importance = widget.SuccessImportance
	return btn
}

func createLaunchButton(w fyne.Window, version string, stopBtn *widget.Button, buttonContainer *fyne.Container) {
	launchButton := NewGreenButton("Launch", nil)
	launchButton.OnTapped = func() {
		if currentProcess != nil {
			dialog.ShowError(fmt.Errorf("owlcms-tracker is already running"), w)
			return
		}

		log.Printf("Launching version %s\n", version)

		if err := launchTracker(version, launchButton, stopBtn); err != nil {
			dialog.ShowError(err, w)
			return
		}
	}
	buttonContainer.Add(container.NewPadded(launchButton))
}

func adjustUpdateButton(mostRecent string, version string, updateButton *widget.Button, buttonContainer *fyne.Container, w fyne.Window) {
	compare, err := semver.NewVersion(mostRecent)
	x, err2 := semver.NewVersion(version)
	if err == nil && err2 == nil {
		if compare.GreaterThan(x) {
			updateButton.SetText(fmt.Sprintf("Update to %s", mostRecent))
			updateButton.OnTapped = func() {
				updateVersion(version, mostRecent, w)
			}
			updateButton.Refresh()
		} else {
			buttonContainer.Refresh()
		}
	} else {
		log.Printf("failed to compare versions: %v %v", err, err2)
	}
}

func updateVersion(existingVersion string, targetVersion string, w fyne.Window) {
	currentVersionDir := filepath.Join(installDir, existingVersion)

	// Try device-independent name first, then old platform-specific names for backward compatibility
	assetNames := getAssetNames(targetVersion)
	var zipURL, assetName string

	// Check each asset name in order until we find one that exists
	for _, name := range assetNames {
		testURL := fmt.Sprintf("https://github.com/owlcms/owlcms-tracker/releases/download/%s/%s", targetVersion, name)
		if checkAssetExists(testURL) {
			zipURL = testURL
			assetName = name
			break
		}
	}

	if zipURL == "" {
		// None of the expected assets exist - fail immediately
		dialog.ShowError(fmt.Errorf("no tracker release asset found for version %s (tried: %v)", targetVersion, assetNames), w)
		return
	}

	extractDir := filepath.Join(installDir, targetVersion)
	if err := shared.EnsureDir0755(extractDir); err != nil {
		dialog.ShowError(fmt.Errorf("creating install directory: %w", err), w)
		return
	}
	zipPath := filepath.Join(installDir, assetName)

	progressBar := widget.NewProgressBar()
	progressBar.SetValue(0.01)
	messageLabel := widget.NewLabel(fmt.Sprintf("Downloading owlcms-tracker %s...", targetVersion))
	content := container.NewVBox(messageLabel, progressBar)
	progressDialog := dialog.NewCustom(
		"Updating owlcms-tracker",
		"Please wait...",
		content,
		w)
	progressDialog.Show()

	go func() {
		defer progressDialog.Hide()

		progressCallback := func(downloaded, total int64) {
			if total > 0 {
				progress := float64(downloaded) / float64(total)
				progressBar.SetValue(progress)
			}
		}

		err := downloadutils.DownloadArchive(zipURL, zipPath, progressCallback, nil)
		if err != nil {
			dialog.ShowError(fmt.Errorf("download failed: %w", err), w)
			return
		}

		messageLabel.SetText("Extracting files...")
		messageLabel.Refresh()

		stopProgress := startTimedProgress(progressBar, 0.0, 1.0, 40*time.Second)
		err = downloadutils.ExtractZip(zipPath, extractDir)
		stopProgress()
		if err != nil {
			dialog.ShowError(fmt.Errorf("extraction failed: %w", err), w)
			return
		}

		// Copy local files to the new version
		err = copyFiles(filepath.Join(currentVersionDir, "local"), filepath.Join(extractDir, "local"), true)
		if err != nil {
			log.Printf("No local files to copy from %s\n", currentVersionDir)
		}

		progressBar.SetValue(1.0)
		dialog.ShowInformation("Update Complete", fmt.Sprintf("Successfully updated to version %s", targetVersion), w)

		recomputeVersionList(w)

		latestInstalled = findLatestInstalled()
		checkForNewerVersion()
	}()
}

func filterVersions(versions []string, currentVer string) []string {
	var filtered []string
	for _, version := range versions {
		if version != currentVer {
			filtered = append(filtered, version)
		}
	}
	return filtered
}

func copyFiles(srcDir, destDir string, alwaysCopy bool) error {
	var localDirModTime time.Time
	if !alwaysCopy {
		srcLocalDir := srcDir
		info, err := os.Stat(srcLocalDir)
		if err != nil {
			return err
		}
		localDirModTime = info.ModTime()
	}

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(destDir, relPath)

		if info.IsDir() {
			return shared.EnsureDir0755(destPath)
		}

		if !alwaysCopy {
			log.Printf("Comparing file timestamps: %s %s %s %s\n", path, info.ModTime(), destPath, localDirModTime)
			if info.ModTime().Before(localDirModTime) {
				return nil
			}
		}

		log.Printf("Copying file: %s to %s\n", path, destPath)

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		if err := shared.EnsureDir0755(filepath.Dir(destPath)); err != nil {
			return err
		}
		destFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer destFile.Close()

		_, err = io.Copy(destFile, srcFile)
		return err
	})
}

// RefreshVersionList refreshes the version list display
func RefreshVersionList(w fyne.Window) {
	recomputeVersionList(w)
}

func recomputeVersionList(w fyne.Window) {
	log.Println("Reinitializing version list")
	versionContainer.Objects = nil
	newVersionList := createVersionList(w, stopButton)

	numVersions := len(getAllInstalledVersions())
	versionScroll := container.NewVScroll(newVersionList)
	// Ensure the version scroll has enough height to display up to 4 rows
	versionScroll.SetMinSize(fyne.NewSize(0, computeVersionScrollHeight(numVersions)))
	center := container.NewStack(versionScroll)

	if numVersions == 0 {
		// Reset the tab to explanation mode (clears download UI then shows explanation)
		resetToExplainMode()
		// No version list to add — return early
		return
	} else {
		versionScroll.Show()
		versionContainer.Show()
	}

	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(fyne.NewSize(1, 8))
	content := container.NewVBox(spacer, center)

	versionContainer.Objects = nil
	versionContainer.Add(content)

	// Update the explanation/status label
	updateExplanation()

	log.Println("Version list reinitialized")
}

func removeAllVersions() {
	entries, err := os.ReadDir(installDir)
	if err != nil {
		log.Printf("Failed to read tracker directory: %v\n", err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			if _, err := semver.NewVersion(entry.Name()); err == nil {
				versionDir := filepath.Join(installDir, entry.Name())
				log.Printf("Removing version directory: %s\n", versionDir)
				os.RemoveAll(versionDir)
			}
		}
	}
	// After removing all version directories, update UI in case no versions remain
	resetToExplainMode()
}

func uninstallAll() {
	dialog.ShowConfirm("Confirm Uninstall", "This will remove all the data and configurations currently stored.\nIf you proceed, this cannot be undone.", func(confirm bool) {
		if !confirm {
			return
		}
		log.Printf("Removing all owlcms-tracker data from: %s\n", installDir)
		err := os.RemoveAll(installDir)
		if err != nil {
			log.Printf("Failed to remove all data: %v\n", err)
			dialog.ShowError(fmt.Errorf("failed to remove all data: %w", err), mainWindow)
			return
		}
		log.Println("All data removed successfully")
		dialog.ShowInformation("Success", "All data removed successfully", mainWindow)
		// Refresh the tab so the uninstalled explanation appears
		recomputeVersionList(mainWindow)
	}, mainWindow)
}
