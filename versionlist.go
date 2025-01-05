package main

import (
	"fmt"
	"io"
	"log"
	"os"
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

func createVersionList(w fyne.Window, stopButton *widget.Button) *widget.List {
	versions := getAllInstalledVersions()

	versionList = widget.NewList(
		func() int { return len(versions) },
		func() fyne.CanvasObject {
			label := widget.NewLabel("Template")
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
			updateButton := buttonContainer.Objects[2].(*fyne.Container).Objects[0].(*widget.Button) // New button
			removeButton := buttonContainer.Objects[3].(*fyne.Container).Objects[0].(*widget.Button)
			importButton := buttonContainer.Objects[4].(*fyne.Container).Objects[0].(*widget.Button) // New button

			version := versions[index]
			label.SetText(version)
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
				updateButton.Hide() // Hide update button if allReleases is empty
			} else {
				updateButton.SetText("Update") // Set text for new button
				var mostRecent string
				var err error

				// Check if the current version is stable or a prerelease
				if !containsPreReleaseTag(version) {
					mostRecent, err = getMostRecentStableRelease()
					if err == nil {
						prepareButton(mostRecent, version, updateButton, w)
					} else {
						log.Printf("failed to get most recent stable release: %v", err)
					}
				} else {
					mostRecent, err = getMostRecentPrerelease()
					if err == nil {
						prepareButton(mostRecent, version, updateButton, w)
					} else {
						log.Printf("failed to get most recent prerelease: %v", err)
					}
				}
			}

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

func prepareButton(mostRecent string, version string, updateButton *widget.Button, w fyne.Window) {
	go log.Printf("mostRecent: %s, version: %s", mostRecent, version)
	compare, err := semver.NewVersion(mostRecent)
	x, err2 := semver.NewVersion(version)
	if err == nil && err2 == nil {
		if compare.GreaterThan(x) {
			updateButton.SetText(fmt.Sprintf("Update to %s", mostRecent))
			updateButton.OnTapped = func() {
				updateVersion(mostRecent, w)
			}
			updateButton.Refresh()
		} else {
			updateButton.Hide()
		}
	} else {
		log.Printf("failed to compare versions: %v %v", err, err2)
	}
}

func updateVersion(version string, w fyne.Window) {
	// Implement the update logic here
	dialog.ShowInformation("Update", fmt.Sprintf("Updating version %s", version), w)
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
