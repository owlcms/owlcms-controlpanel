package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"owlcms-launcher/downloadUtils"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/Masterminds/semver/v3"
)

var versionList *widget.List

func getAllInstalledVersions() []string {
	owlcmsDir := owlcmsInstallDir
	entries, err := os.ReadDir(owlcmsDir)
	if err != nil {
		return nil
	}

	versionPattern := regexp.MustCompile(`^\d+\.\d+\.\d+(?:-(?:rc|alpha|beta)(?:\d+)?)?$`)
	var versions []*semver.Version
	for _, entry := range entries {
		if entry.IsDir() && versionPattern.MatchString(entry.Name()) {
			v, err := semver.NewVersion(entry.Name())
			if err == nil {
				versions = append(versions, v)
			}
		}
	}

	sort.Sort(sort.Reverse(semver.Collection(versions)))

	var versionStrings []string
	for _, v := range versions {
		versionStrings = append(versionStrings, v.String())
	}

	return versionStrings
}

func findLatestInstalled() string {
	owlcmsDir := owlcmsInstallDir
	entries, err := os.ReadDir(owlcmsDir)
	if err != nil {
		return ""
	}

	versionPattern := regexp.MustCompile(`^\d+\.\d+\.\d+(?:-(?:rc|alpha|beta)(?:\d+)?)?$`)
	var versions []*semver.Version
	for _, entry := range entries {
		if entry.IsDir() && versionPattern.MatchString(entry.Name()) {
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

func findLatestStableInstalled() string {
	owlcmsDir := owlcmsInstallDir
	entries, err := os.ReadDir(owlcmsDir)
	if err != nil {
		return ""
	}

	versionPattern := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	var versions []*semver.Version
	for _, entry := range entries {
		if entry.IsDir() && versionPattern.MatchString(entry.Name()) {
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

func findLatestPrereleaseInstalled() string {
	owlcmsDir := owlcmsInstallDir
	entries, err := os.ReadDir(owlcmsDir)
	if err != nil {
		return ""
	}

	versionPattern := regexp.MustCompile(`^\d+\.\d+\.\d+-(?:rc|alpha|beta)(?:\d+)?$`)
	var versions []*semver.Version
	for _, entry := range entries {
		if entry.IsDir() && versionPattern.MatchString(entry.Name()) {
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

func createVersionList(w fyne.Window, stopButton *widget.Button) *widget.List {
	versions := getAllInstalledVersions()

	versionList = widget.NewList(
		func() int { return len(versions) },
		func() fyne.CanvasObject {
			label := widget.NewLabelWithStyle("Template", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			launchButton := widget.NewButton("Launch", nil)
			filesButton := widget.NewButton("Files", nil)
			updateButton := widget.NewButton("Update", nil) // New update button
			removeButton := widget.NewButton("Remove", nil)
			importButton := widget.NewButton("Import Data and Config", nil) // New button
			launchButton.Resize(fyne.NewSize(80, 25))
			removeButton.Resize(fyne.NewSize(80, 25))
			filesButton.Resize(fyne.NewSize(80, 25))
			updateButton.Resize(fyne.NewSize(150, 25))      // Resize new button
			importButton.Resize(fyne.NewSize(150, 25))      // Resize new button
			launchButton.Importance = widget.HighImportance // Make the launch button important
			buttonContainer := container.NewHBox(
				container.NewPadded(launchButton),
				container.NewPadded(filesButton),
				container.NewPadded(updateButton), // Add new button to container
				container.NewPadded(removeButton),
				container.NewPadded(importButton),
				layout.NewSpacer(), // Add spacer to push buttons to the left
			)
			grid := container.New(layout.NewHBoxLayout(), container.NewGridWrap(fyne.NewSize(120, 25), label), buttonContainer)
			return grid
		},
		func(index widget.ListItemID, item fyne.CanvasObject) {
			grid := item.(*fyne.Container)
			label := grid.Objects[0].(*fyne.Container).Objects[0].(*widget.Label)
			buttonContainer := grid.Objects[1].(*fyne.Container)
			launchButton := buttonContainer.Objects[0].(*fyne.Container).Objects[0].(*widget.Button)
			filesButton := buttonContainer.Objects[1].(*fyne.Container).Objects[0].(*widget.Button)

			var removeButton, updateButton, importButton *widget.Button
			log.Printf("=== Button container has %d objects\n", len(buttonContainer.Objects))

			if len(buttonContainer.Objects) == 6 {
				updateButton = buttonContainer.Objects[2].(*fyne.Container).Objects[0].(*widget.Button)
				removeButton = buttonContainer.Objects[3].(*fyne.Container).Objects[0].(*widget.Button)
				importButton = buttonContainer.Objects[4].(*fyne.Container).Objects[0].(*widget.Button)
				if len(versions) <= 1 {
					log.Printf("=== Removing import button\n")
					buttonContainer.Remove(buttonContainer.Objects[4])
					importButton = nil
				}
			} else if len(buttonContainer.Objects) == 5 {
				updateButton = nil
				removeButton = buttonContainer.Objects[2].(*fyne.Container).Objects[0].(*widget.Button)
				importButton = buttonContainer.Objects[3].(*fyne.Container).Objects[0].(*widget.Button)
				if len(versions) <= 1 {
					buttonContainer.Remove(buttonContainer.Objects[3])
				}
			} else if len(buttonContainer.Objects) == 4 {
				updateButton = nil
				removeButton = buttonContainer.Objects[2].(*fyne.Container).Objects[0].(*widget.Button)
				importButton = nil
			}

			version := versions[index]
			label.SetText(version)
			label.TextStyle = fyne.TextStyle{Bold: true} // Make the version number bold
			label.Refresh()
			launchButton.SetText("Launch")
			launchButton.OnTapped = func() {
				if currentProcess != nil {
					dialog.ShowError(fmt.Errorf("OWLCMS is already running"), w)
					return
				}

				log.Printf("Launching version %s\n", version)
				if err := checkJava(statusLabel); err != nil {
					dialog.ShowError(fmt.Errorf("java check/installation failed: %w", err), w)
					return
				}

				if err := launchOwlcms(version, launchButton, stopButton); err != nil {
					dialog.ShowError(err, w)
					return
				}
			}

			removeButton.SetText("Remove")
			removeButton.OnTapped = func() {
				dialog.ShowConfirm("Confirm Remove",
					fmt.Sprintf("Do you want to remove OWLCMS version %s?", version),
					func(ok bool) {
						if !ok {
							return
						}

						err := os.RemoveAll(filepath.Join(owlcmsInstallDir, version))
						if err != nil {
							dialog.ShowError(fmt.Errorf("failed to remove OWLCMS %s: %w", version, err), w)
							return
						}

						// Recompute the version list
						recomputeVersionList(w)

						// Check if a more recent version is available
						checkForNewerVersion()
						downloadContainer.Refresh()
					},
					w)
			}

			filesButton.SetText("Files")
			filesButton.OnTapped = func() {
				versionDir := filepath.Join(owlcmsInstallDir, version)
				if err := openFileExplorer(versionDir); err != nil {
					dialog.ShowError(fmt.Errorf("failed to open file explorer for %s: %w", versionDir, err), w)
				}
			}

			if len(allReleases) == 0 {
				buttonContainer.Remove(buttonContainer.Objects[2])
			} else {
				if updateButton != nil {
					updateButton.SetText("Update") // Set text for new button
					var mostRecent string
					var err error

					// Check if the current version is stable or a prerelease
					if !containsPreReleaseTag(version) {
						mostRecent, err = getMostRecentStableRelease()
						if err == nil {
							prepareButton(mostRecent, version, updateButton, buttonContainer, w)
						} else {
							log.Printf("failed to get most recent stable release: %v", err)
						}
					} else {
						mostRecent, err = getMostRecentPrerelease()
						if err == nil {
							prepareButton(mostRecent, version, updateButton, buttonContainer, w)
						} else {
							log.Printf("failed to get most recent prerelease: %v", err)
						}
					}
				}
			}

			if importButton != nil {
				importButton.SetText("Import Data and Config") // Set text for new button
				if len(versions) <= 1 {
					importButton.Hide() // Hide import button if there is only one installed version
				} else {
					importButton.Show()
					importButton.OnTapped = func() {
						// Open a dialog to select the source version
						sourceVersions := filterVersions(versions, version) // Filter out the current version
						sourceVersionDropdown := widget.NewSelect(sourceVersions, func(selected string) {})
						dialog.ShowForm("Import Data and Config",
							"Import",
							"Cancel",
							[]*widget.FormItem{
								widget.NewFormItem("Copy the database and locally modified configurations from a previous installation", sourceVersionDropdown),
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

								sourceDir := filepath.Join(owlcmsInstallDir, sourceVersion)
								destDir := filepath.Join(owlcmsInstallDir, version)

								if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
									dialog.ShowError(fmt.Errorf("source version %s does not exist", sourceVersion), w)
									return
								}

								// Copy database files
								if err := copyFiles(filepath.Join(sourceDir, "database"), filepath.Join(destDir, "database"), true); err != nil {
									log.Printf("No database files to copy from %s\n", sourceDir)
								}
								// Copy local files if they are newer
								if err := copyFiles(filepath.Join(sourceDir, "local"), filepath.Join(destDir, "local"), false); err != nil {
									log.Printf("No local files to copy from %s\n", sourceDir)
									dialog.ShowError(fmt.Errorf("failed to copy local files: %w", err), w)
									return
								}

								dialog.ShowInformation("Import Complete", fmt.Sprintf("Successfully imported data and config from version %s to version %s", sourceVersion, version), w)
							},
							w)
					}
				}
			}
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

	// Log the versions being added
	log.Println("Versions being added to the version list:")
	for _, version := range versions {
		log.Println(version)
	}

	return versionList
}

func prepareButton(mostRecent string, version string, updateButton *widget.Button, buttonContainer *fyne.Container, w fyne.Window) {
	latestStable, stableErr := getMostRecentStableRelease()
	latestPrerelease, preErr := getMostRecentPrerelease()
	latestStableInstalled := findLatestStableInstalled()
	latestPrereleaseInstalled := findLatestPrereleaseInstalled()

	// Check if the latest installed version is the latest stable or prerelease version
	if (!containsPreReleaseTag(latestStableInstalled) && stableErr == nil && latestStableInstalled == latestStable) ||
		(containsPreReleaseTag(latestPrereleaseInstalled) && preErr == nil && latestPrereleaseInstalled == latestPrerelease) {
		buttonContainer.Remove(buttonContainer.Objects[2])
		buttonContainer.Remove(buttonContainer.Objects[4])
		return
	}

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
			buttonContainer.Remove(buttonContainer.Objects[2]) // Remove padded container if no update is available
			buttonContainer.Refresh()
		}
	} else {
		log.Printf("failed to compare versions: %v %v", err, err2)
	}
}

func updateVersion(existingVersion string, targetVersion string, w fyne.Window) {
	// Note the timestamp of the current version's top-level directory
	currentVersionDir := filepath.Join(owlcmsInstallDir, existingVersion)
	modTime, err := os.Stat(currentVersionDir)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to get info for current version directory: %w", err), w)
		return
	}

	// Move the current version to a temporary directory in the installation area
	tempDir := filepath.Join(owlcmsInstallDir, "temp")
	err = os.Rename(currentVersionDir, tempDir)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to move current version to temporary directory: %w", err), w)
		return
	}
	// set the modification time of the directory to modTime
	err = os.Chtimes(tempDir, modTime.ModTime(), modTime.ModTime())
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to set modification time of temporary directory: %w", err), w)
		return
	}

	// Download and extract the version given by string
	var urlPrefix string
	if containsPreReleaseTag(targetVersion) {
		urlPrefix = "https://github.com/owlcms/owlcms4-prerelease/releases/download"
	} else {
		urlPrefix = "https://github.com/owlcms/owlcms4/releases/download"
	}
	fileName := fmt.Sprintf("owlcms_%s.zip", targetVersion)
	zipURL := fmt.Sprintf("%s/%s/%s", urlPrefix, targetVersion, fileName)
	zipPath := filepath.Join(owlcmsInstallDir, fileName)
	extractPath := filepath.Join(owlcmsInstallDir, targetVersion)

	progressDialog := dialog.NewCustom(
		"Updating OWLCMS",
		"Please wait...",
		widget.NewTextGridFromString("Downloading and extracting files..."),
		w)
	progressDialog.Show()

	defer progressDialog.Hide()

	err = downloadUtils.DownloadArchive(zipURL, zipPath)
	if err != nil {
		dialog.ShowError(fmt.Errorf("download failed: %w", err), w)
		return
	}

	err = downloadUtils.ExtractZip(zipPath, extractPath)
	if err != nil {
		dialog.ShowError(fmt.Errorf("extraction failed: %w", err), w)
		return
	}

	// Copy the database from the temporary directory to the new version
	err = copyFiles(filepath.Join(tempDir, "database"), filepath.Join(extractPath, "database"), true)
	if err != nil {
		log.Printf("No database files to copy from %s\n", tempDir)
	}

	// Copy files newer than the memorized timestamp from the temporary directory to the new version
	err = copyFiles(filepath.Join(tempDir, "local"), filepath.Join(extractPath, "local"), false)
	if err != nil {
		log.Printf("No local files to copy from %s\n", tempDir)
		dialog.ShowError(fmt.Errorf("failed to copy local files: %w", err), w)
		return
	}

	// Remove the temporary directory
	err = os.RemoveAll(tempDir)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to remove temporary directory: %w", err), w)
		return
	}

	dialog.ShowInformation("Update Complete", fmt.Sprintf("Successfully updated to version %s", targetVersion), w)

	// Recompute the version list
	recomputeVersionList(w)

	// Recompute the downloadTitle
	checkForNewerVersion()

}

func filterVersions(versions []string, currentVersion string) []string {
	var filtered []string
	for _, version := range versions {
		if version != currentVersion {
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
			return os.MkdirAll(destPath, info.Mode())
		}

		if !alwaysCopy {
			if info.ModTime().Before(localDirModTime) {
				// Skip copying if the file is older than the local directory timestamp
				return nil
			}
		}

		log.Printf("Copying file: %s to %s\n", path, destPath) // Log file names being copied

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		destFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer destFile.Close()

		_, err = io.Copy(destFile, srcFile)
		return err
	})
}

func recomputeVersionList(w fyne.Window) {
	// Reinitialize the version list
	log.Println("Reinitializing version list")
	versionContainer.Objects = nil // Clear the container
	newVersionList := createVersionList(w, stopButton)

	// Update the scroll container's size
	numVersions := len(getAllInstalledVersions())
	versionScroll := container.NewVScroll(newVersionList)
	versionScroll.SetMinSize(fyne.NewSize(400, computeVersionScrollHeight(numVersions)))

	versionLabel := widget.NewLabel("Installed Versions:")
	if numVersions == 0 {
		versionLabel.Hide()
		versionScroll.Hide()
		versionContainer.Hide()
	} else {
		versionLabel.Show()
		versionScroll.Show()
		versionContainer.Show()
	}
	versionContainer.Add(versionLabel)
	versionContainer.Add(versionScroll)

	log.Println("Version list reinitialized")
}
