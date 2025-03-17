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
	"strconv"
	"strings"
	"syscall"
	"time"

	customdialog "owlcms-launcher/dialog" // Alias our custom dialog package

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/Masterminds/semver/v3"
)

var (
	versionList *widget.List
)

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
			// Template item for the version list. Used to compute sizes
			label := widget.NewLabelWithStyle("LabelTemplate", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			launchButton := widget.NewButton("ButtonTemplate", nil)
			launchButton.Resize(fyne.NewSize(80, 25))
			launchButton.Importance = widget.HighImportance
			buttonContainer := container.NewHBox(
				container.NewPadded(launchButton),
				layout.NewSpacer(), // Add spacer to push buttons to the left
			)
			grid := container.New(layout.NewHBoxLayout(), container.NewGridWrap(fyne.NewSize(120, 25), label), buttonContainer)
			return grid
		},
		func(index widget.ListItemID, item fyne.CanvasObject) {
			// This function is called for each item in the list to build the actual entries
			version := versions[index]

			grid := item.(*fyne.Container)

			label := grid.Objects[0].(*fyne.Container).Objects[0].(*widget.Label)
			label.SetText(version)
			label.TextStyle = fyne.TextStyle{Bold: true} // Make the version number bold
			label.Refresh()

			buttonContainer := grid.Objects[1].(*fyne.Container)
			buttonContainer.RemoveAll()

			createLaunchButton(w, version, stopButton, buttonContainer)
			createFilesButton(version, w, buttonContainer)
			if len(allReleases) > 0 {
				createUpdateButton(version, w, buttonContainer)
			}
			if len(versions) > 1 {
				createImportButton(versions, version, w, buttonContainer)
			}
			createRemoveButton(version, w, buttonContainer)
			buttonContainer.Add(layout.NewSpacer()) // Add spacer to push buttons to the left
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
	importButton := widget.NewButton("Import Data and Config", nil)
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
	buttonContainer.Add(container.NewPadded(importButton))
}

func createUpdateButton(version string, w fyne.Window, buttonContainer *fyne.Container) {
	updateButton := widget.NewButton("Update", nil)
	var mostRecent string
	var err error

	latestStable, stableErr := getMostRecentStableRelease()
	latestPrerelease, preErr := getMostRecentPrerelease()
	latestStableInstalled := findLatestStableInstalled()
	latestPrereleaseInstalled := findLatestPrereleaseInstalled()

	if (!containsPreReleaseTag(latestStableInstalled) && stableErr == nil && latestStableInstalled == latestStable) ||
		(containsPreReleaseTag(latestPrereleaseInstalled) && preErr == nil && latestPrereleaseInstalled == latestPrerelease) {
		// here is no point in updating since the most recent version is already installed
		return
	}

	// Check if the current version is stable or a prerelease
	if !containsPreReleaseTag(version) {
		mostRecent, err = getMostRecentStableRelease()
		if err == nil {
			adjustUpdateButton(mostRecent, version, updateButton, buttonContainer, w)
		} else {
			log.Printf("failed to get most recent stable release: %v", err)
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
	buttonContainer.Add(container.NewPadded(removeButton))
}

func createFilesButton(version string, w fyne.Window, buttonContainer *fyne.Container) *widget.Button {
	filesButton := widget.NewButton("Files", nil)
	filesButton.Resize(fyne.NewSize(80, 25))
	filesButton.SetText("Files")
	filesButton.OnTapped = func() {
		versionDir := filepath.Join(owlcmsInstallDir, version)
		if err := openFileExplorer(versionDir); err != nil {
			dialog.ShowError(fmt.Errorf("failed to open file explorer for %s: %w", versionDir, err), w)
		}
	}
	buttonContainer.Add(container.NewPadded(filesButton))
	return filesButton
}

func createLaunchButton(w fyne.Window, version string, stopButton *widget.Button, buttonContainer *fyne.Container) {
	launchButton := widget.NewButton("Launch", nil)
	launchButton.Resize(fyne.NewSize(80, 25))
	launchButton.Importance = widget.HighImportance
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
	buttonContainer.Add(container.NewPadded(launchButton))
}

func adjustUpdateButton(mostRecent string, version string, updateButton *widget.Button, buttonContainer *fyne.Container, w fyne.Window) {
	compare, err := semver.NewVersion(mostRecent)
	x, err2 := semver.NewVersion(version)
	if err == nil && err2 == nil {
		if compare.GreaterThan(x) {
			updateButton.SetText(fmt.Sprintf("Update to %s", mostRecent))
			updateButton.OnTapped = func() {
				currentOS := downloadUtils.GetGoos()
				if currentOS == "linux" || currentOS == "darwin" {
					data, err := os.ReadFile(pidFilePath)
					if err == nil && len(data) > 0 {
						pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
						log.Printf("found pid %d in file %s \n", pid, pidFilePath)
						if err == nil && pid != 0 {
							process, err := os.FindProcess(pid)
							if err == nil && process.Signal(syscall.Signal(0)) == nil {
								dialog.ShowError(fmt.Errorf("an OWLCMS process is already running with PID %d.\nUse the 'Processes' menu to stop it before updating", pid), w)
								return
							}
						}
					}
				}

				confirmDialog := dialog.NewConfirm("Backup Suggestion",
					"Before updating, we suggest that you take a backup of your current database using the 'Export Database' button of the 'Prepare Competition' page.",
					func(confirm bool) {
						if confirm {
							updateVersion(version, mostRecent, w)
						}
					},
					w,
				)
				confirmDialog.SetConfirmText("Perform Update")
				confirmDialog.SetDismissText("Cancel Update")
				confirmDialog.Show()
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
	// Note the timestamp of the current version's top-level directory
	currentVersionDir := filepath.Join(owlcmsInstallDir, existingVersion)
	// _, err := os.Stat(currentVersionDir)
	// if err != nil {
	// 	dialog.ShowError(fmt.Errorf("failed to get info for current version directory: %w", err), w)
	// 	return
	// }

	// Move the current version to a temporary directory in the installation area
	existingVersionDir := currentVersionDir
	// err = os.Rename(currentVersionDir, existingVersionDir)
	// if err != nil {
	// 	dialog.ShowError(fmt.Errorf("failed to move current version to temporary directory: %w", err), w)
	// 	return
	// }
	// // set the modification time of the directory to modTime
	// err = os.Chtimes(existingVersionDir, modTime.ModTime(), modTime.ModTime())
	// if err != nil {
	// 	dialog.ShowError(fmt.Errorf("failed to set modification time of temporary directory: %w", err), w)
	// 	return
	// }

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
	newVersionDir := filepath.Join(owlcmsInstallDir, targetVersion)

	// Create a cancel channel
	cancel := make(chan bool)

	progressDialog, progressBar := customdialog.NewDownloadDialog(
		"Updating OWLCMS",
		w,
		cancel)
	progressDialog.Show()
	// Remove the defer call as we want to control exactly when this hides
	// defer progressDialog.Hide()

	progressCallback := func(downloaded, total int64) {
		if total > 0 {
			percentage := float64(downloaded) / float64(total)
			progressBar.SetValue(percentage)
		}
	}

	err := downloadUtils.DownloadArchive(zipURL, zipPath, progressCallback, cancel)
	if err != nil {
		if err.Error() == "download cancelled" {
			// Handle cancellation
			log.Println("Update cancelled by user")

			// Clean up the incomplete zip file
			os.Remove(zipPath)

			return
		}
		dialog.ShowError(fmt.Errorf("download failed: %w", err), w)
		return
	}

	err = downloadUtils.ExtractZip(zipPath, newVersionDir)
	if err != nil {
		dialog.ShowError(fmt.Errorf("extraction failed: %w", err), w)
		return
	}

	// Track what was copied successfully
	var databaseCopied bool
	var localFilesCopied bool

	// Check if the database directory exists before attempting to copy
	existingDatabaseDir := filepath.Join(existingVersionDir, "database")
	if _, err := os.Stat(existingDatabaseDir); !os.IsNotExist(err) {
		// Copy the database from the temporary directory to the new version
		err = copyFiles(existingDatabaseDir, filepath.Join(newVersionDir, "database"), true)
		if err != nil {
			// copy failed, log the error
			log.Printf("could not copy the database from %s to %s: %v\n", existingDatabaseDir, filepath.Join(newVersionDir, "database"), err)

			// Ensure the progress dialog is hidden first
			progressDialog.Hide()

			// Check if this is likely a file lock issue (platform-independent way)
			isLockError := false
			if os.IsPermission(err) {
				isLockError = true
			}

			// On Windows, also check for sharing violation
			if pathErr, ok := err.(*os.PathError); ok {
				if errno, ok := pathErr.Err.(syscall.Errno); ok {
					// Windows ERROR_SHARING_VIOLATION (32) and ERROR_LOCK_VIOLATION (33)
					if errno == 32 || errno == 33 {
						isLockError = true
					}
				}
			}

			var errorMsg string
			if isLockError {
				errorMsg = "Database files are locked and cannot be copied.\n\nPlease make sure OWLCMS is not running before trying to update."
			} else {
				errorMsg = fmt.Sprintf("Failed to copy database: %v", err)
			}

			// Create a custom dialog that will be shown reliably
			content := container.NewVBox(
				widget.NewLabel(errorMsg),
				widget.NewLabel("\nThe update will be cancelled."),
			)

			modalDialog := dialog.NewCustom(
				"Database Copy Error",
				"OK",
				content,
				w,
			)

			// Set callback for when dialog is dismissed
			modalDialog.SetOnClosed(func() {
				log.Println("Error dialog closed, cleaning up...")

				// Clean up the downloaded directory since update failed
				cleanupErr := os.RemoveAll(newVersionDir)
				if cleanupErr != nil {
					log.Printf("Failed to clean up directory: %v", cleanupErr)
				}

				// Update UI
				recomputeVersionList(w)
				checkForNewerVersion()
				w.Content().Refresh()
			})

			// Show dialog
			modalDialog.Show()
			return
		}
		// Database copied successfully
		databaseCopied = true
		log.Println("Database files copied successfully")
	} else {
		log.Printf("Database directory does not exist in %s\n", existingDatabaseDir)
	}

	// Copy files newer than the memorized timestamp from the temporary directory to the new version
	err = copyFiles(filepath.Join(existingVersionDir, "local"), filepath.Join(newVersionDir, "local"), false)
	if err != nil {
		log.Printf("No local files to copy from %s\n", existingVersionDir)
		dialog.ShowError(fmt.Errorf("failed to copy local files: %w", err), w)
		return
	} else {
		localFilesCopied = true
		log.Println("Local configuration files copied successfully")
	}

	// // Remove the existing directory
	// err = os.RemoveAll(existingVersionDir)
	// if err != nil {
	// 	dialog.ShowError(fmt.Errorf("failed to remove temporary directory: %w", err), w)
	// 	return
	// }

	// Hide progress dialog before showing success dialog
	progressDialog.Hide()

	// Create a detailed success message
	var successMessage string
	successMessage = fmt.Sprintf("Successfully updated to version %s\n", targetVersion)

	if databaseCopied && localFilesCopied {
		successMessage += "\n✓ Database files have been copied\n✓ Local configuration files have been copied"
	} else if databaseCopied {
		successMessage += "\n✓ Database files have been copied"
	} else if localFilesCopied {
		successMessage += "\n✓ Local configuration files have been copied"
	}

	// Create a custom modal dialog that won't be dismissed automatically
	content := container.NewVBox(
		widget.NewLabel(successMessage),
	)

	// Create a custom dialog and capture its reference
	successDialog := dialog.NewCustom(
		"Update Complete",
		"OK",
		content,
		w,
	)

	// Set callback for when the dialog is closed
	successDialog.SetOnClosed(func() {
		log.Println("Success dialog acknowledged, updating UI...")

		// Recompute the version list
		recomputeVersionList(w)

		// Recompute the downloadTitle
		checkForNewerVersion()

		// Refresh UI components
		w.Content().Refresh()
	})

	// Show the dialog - it will block until the user dismisses it
	log.Println("Showing success dialog")
	successDialog.Show()
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
