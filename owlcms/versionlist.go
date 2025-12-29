package owlcms

import (
	"archive/zip"
	"crypto/sha256"
	"fmt"
	"image/color"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	customdialog "owlcms-launcher/owlcms/dialog"
	"owlcms-launcher/owlcms/downloadutils"
	"owlcms-launcher/shared"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
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
	owlcmsDir := installDir
	entries, err := os.ReadDir(owlcmsDir)
	if err != nil {
		return nil
	}

	var versions []*semver.Version
	for _, entry := range entries {
		if entry.IsDir() {
			// Try to parse the directory name as a semver version
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
	owlcmsDir := installDir
	entries, err := os.ReadDir(owlcmsDir)
	if err != nil {
		return ""
	}

	// Accept any valid semver directory name
	var versions []*semver.Version
	for _, entry := range entries {
		if entry.IsDir() {
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
	owlcmsDir := installDir
	entries, err := os.ReadDir(owlcmsDir)
	if err != nil {
		return ""
	}

	var versions []*semver.Version
	for _, entry := range entries {
		if entry.IsDir() {
			v, err := semver.NewVersion(entry.Name())
			if err == nil && v.Prerelease() == "" {
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
	owlcmsDir := installDir
	entries, err := os.ReadDir(owlcmsDir)
	if err != nil {
		return ""
	}

	// Accept any valid semver with a pre-release tag
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
			// Template item for the version list. Used to compute sizes
			label := widget.NewLabelWithStyle("LabelTemplate", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			launchButton := widget.NewButton("ButtonTemplate", nil)
			launchButton.Resize(fyne.NewSize(80, 25))
			launchButton.Importance = widget.HighImportance
			buttonContainer := container.NewHBox(
				container.NewPadded(launchButton),
				layout.NewSpacer(), // Add spacer to push buttons to the left
			)
			grid := container.New(layout.NewHBoxLayout(), container.NewGridWrap(fyne.NewSize(250, 25), label), buttonContainer)
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

			createLaunchButton(w, version, stopBtn, buttonContainer)
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

				sourceDir := filepath.Join(installDir, sourceVersion)
				destDir := filepath.Join(installDir, version)

				if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
					dialog.ShowError(fmt.Errorf("source version %s does not exist", sourceVersion), w)
					return
				}

				// Show warning about destructive operation
				warningMsg := fmt.Sprintf("⚠️ WARNING: Import Process\n\n"+
					"This will:\n"+
					"1. Extract fresh local files from version %s (any changes you made directly in version %s will be LOST)\n"+
					"2. Apply the same additions, deletions and modifications you made in version %s\n\n"+
					"Do you want to continue?", version, version, sourceVersion)

				dialog.ShowConfirm("Confirm Import",
					warningMsg,
					func(confirmed bool) {
						if !confirmed {
							return
						}

						// Copy database files
						if err := copyFiles(filepath.Join(sourceDir, "database"), filepath.Join(destDir, "database"), true); err != nil {
							log.Printf("No database files to copy from %s\n", sourceDir)
						}

						// Use the new logic to restore local files
						err := restoreLocalFilesFromPreviousVersion(destDir, sourceDir)
						if err != nil {
							log.Printf("Error while processing local files: %v\n", err)
							dialog.ShowError(fmt.Errorf("failed to process local files: %w", err), w)
							return
						}

						dialog.ShowInformation("Import Complete", fmt.Sprintf("Successfully imported data and config from version %s to version %s", sourceVersion, version), w)
					},
					w)
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

				err := os.RemoveAll(filepath.Join(installDir, version))
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
		versionDir := filepath.Join(installDir, version)
		if err := shared.OpenFileExplorer(versionDir); err != nil {
			dialog.ShowError(fmt.Errorf("failed to open file explorer for %s: %w", versionDir, err), w)
		}
	}
	buttonContainer.Add(container.NewPadded(filesButton))
	return filesButton
}

func createLaunchButton(w fyne.Window, version string, stopBtn *widget.Button, buttonContainer *fyne.Container) {
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
		ver := GetTemurinVersion()
		if err := shared.CheckAndInstallJava(ver, statusLabel, w, checkJava); err != nil {
			return
		}

		if err := launchOwlcms(version, launchButton, stopBtn); err != nil {
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
				currentOS := downloadutils.GetGoos()
				if currentOS == "linux" || currentOS == "darwin" {
					data, err := os.ReadFile(pidFilePath)
					if err == nil && len(data) > 0 {
						pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
						log.Printf("found pid %d in file %s \n", pid, pidFilePath)
						if err == nil && pid != 0 {
							process, err := os.FindProcess(pid)
							if err == nil && process.Signal(syscall.Signal(0)) == nil {
								dialog.ShowError(fmt.Errorf("an OWLCMS process is already running with PID %d.\nStop it first. You can use the 'Processes' menu to stop it before updating", pid), w)
								return
							}
						}
					}
				}

				confirmDialog := dialog.NewConfirm("Backup Suggestion",
					"The update process keeps your current version intact so you can revert if needed.\n\nBut we nevertheless suggest that you take a backup of your current database using the 'Export Database' button of the 'Prepare Competition' page.",
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
	currentVersionDir := filepath.Join(installDir, existingVersion)
	existingVersionDir := currentVersionDir

	// Download and extract the version given by string
	var urlPrefix string
	if containsPreReleaseTag(targetVersion) {
		urlPrefix = "https://github.com/owlcms/owlcms4-prerelease/releases/download"
	} else {
		urlPrefix = "https://github.com/owlcms/owlcms4/releases/download"
	}
	fileName := fmt.Sprintf("owlcms_%s.zip", targetVersion)
	zipURL := fmt.Sprintf("%s/%s/%s", urlPrefix, targetVersion, fileName)
	zipPath := filepath.Join(installDir, fileName)
	newVersionDir := filepath.Join(installDir, targetVersion)

	// Create a cancel channel
	cancel := make(chan bool)

	progressDialog, progressBar := customdialog.NewDownloadDialog(
		"Updating OWLCMS",
		w,
		cancel)
	progressDialog.Show()

	progressCallback := func(downloaded, total int64) {
		if total > 0 {
			percentage := float64(downloaded) / float64(total)
			progressBar.SetValue(percentage)
		}
	}

	err := downloadutils.DownloadArchive(zipURL, zipPath, progressCallback, cancel)
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

	// new version is downloaded, now extract it to its own directory
	err = downloadutils.ExtractZip(zipPath, newVersionDir)
	if err != nil {
		dialog.ShowError(fmt.Errorf("extraction failed: %w", err), w)
		return
	}

	// Track what was copied successfully
	var databaseCopied bool
	var localFilesCopied bool

	// Check if the old database directory exists before attempting to copy
	existingDatabaseDir := filepath.Join(existingVersionDir, "database")
	if _, statErr := os.Stat(existingDatabaseDir); !os.IsNotExist(statErr) {
		// Copy the database from the old directory to the new version
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
					// Fix the type mismatch by comparing errno to each value separately
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

	// Use the new logic to restore local files from previous version
	err = restoreLocalFilesFromPreviousVersion(newVersionDir, existingVersionDir)
	if err != nil {
		log.Printf("Error while restoring local configuration files: %v\n", err)
		dialog.ShowError(fmt.Errorf("failed to restore local files: %w", err), w)
		return
	} else {
		localFilesCopied = true
		log.Println("Local configuration files processed successfully")
	}

	// Hide progress dialog before showing success dialog
	progressDialog.Hide()

	// Create a detailed success message
	var successMessage string
	successMessage = fmt.Sprintf("Successfully updated to version %s\n", targetVersion)

	if databaseCopied && localFilesCopied {
		successMessage += "\n✓ Database files have been copied\n✓ Local configuration files have been processed"
	} else if databaseCopied {
		successMessage += "\n✓ Database files have been copied"
	} else if localFilesCopied {
		successMessage += "\n✓ Local configuration files have been processed"
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

// restoreLocalFilesFromPreviousVersion restores files in newDir/local from oldDir/local
// according to the logic described in the prompt.
func restoreLocalFilesFromPreviousVersion(newDir, oldDir string) error {
	newLocal := filepath.Join(newDir, "local")
	oldLocal := filepath.Join(oldDir, "local")
	oldJar := filepath.Join(oldDir, "owlcms.jar")

	// Create import log file
	logFilePath := filepath.Join(newDir, "import.log")
	logFile, err := os.Create(logFilePath)
	if err != nil {
		return fmt.Errorf("failed to create import log: %w", err)
	}
	defer logFile.Close()

	// Helper function to write to both console and log file
	logBoth := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		log.Print(msg)
		fmt.Fprint(logFile, msg)
	}

	logBoth("\n=== Starting Import Process ===\n")
	logBoth("From: %s\n", oldDir)
	logBoth("To: %s\n", newDir)
	logBoth("============================\n\n")

	// Phase 1: Analyze changes made in old version
	logBoth("Phase 1: Analyzing changes in old version...\n")

	// 1. Get top-level directories in newDir/local
	logBoth("  - Getting top-level directories from new version...\n")
	topLevelDirs, err := getTopLevelDirs(newLocal)
	if err != nil {
		return fmt.Errorf("failed to get top-level dirs: %w", err)
	}

	// 2. Build oldJarFiles: map[path]checksum for files in topLevelDirs inside oldJar
	logBoth("  - Reading files from old JAR (reference state)...\n")
	oldJarFiles, err := getJarFilesChecksums(oldJar, topLevelDirs)
	if err != nil {
		return fmt.Errorf("failed to get jar files: %w", err)
	}

	// 3. Create map of files in oldDir/local
	logBoth("  - Reading files from old local directory...\n")
	oldLocalFiles, err := getLocalFiles(oldLocal, topLevelDirs)
	if err != nil {
		return fmt.Errorf("failed to get local files: %w", err)
	}

	// Track changes made in the old local directory relative to the old owlcms.jar (reference)
	var filesDeletedFromOldJar []string   // Files/dirs in old owlcms.jar but deleted from old local
	var filesAddedToOldLocal []string     // Files/dirs added to old local (not in old owlcms.jar)
	var filesModifiedInOldLocal []string  // Files modified in old local vs old owlcms.jar
	var filesUnchangedInOldLocal []string // Files unchanged in old local (same as old owlcms.jar)

	// Build sorted lists of file paths for parallel traversal
	oldJarFilesList := make([]string, 0, len(oldJarFiles))
	for relPath := range oldJarFiles {
		oldJarFilesList = append(oldJarFilesList, relPath)
	}
	sort.Strings(oldJarFilesList)

	oldLocalFilesList := make([]string, 0, len(oldLocalFiles))
	for relPath := range oldLocalFiles {
		oldLocalFilesList = append(oldLocalFilesList, relPath)
	}
	sort.Strings(oldLocalFilesList)

	// Parallel traversal to detect added/deleted directories early
	logBoth("  - Performing parallel traversal to identify changes...\n")
	jarIdx := 0
	localIdx := 0
	processedJarFiles := make(map[string]bool)
	processedLocalFiles := make(map[string]bool)
	oldLocalFilesWithChecksums := make(map[string]string) // Cache checksums as we compute them

	for jarIdx < len(oldJarFilesList) || localIdx < len(oldLocalFilesList) {
		var jarPath, localPath string
		if jarIdx < len(oldJarFilesList) {
			jarPath = oldJarFilesList[jarIdx]
		}
		if localIdx < len(oldLocalFilesList) {
			localPath = oldLocalFilesList[localIdx]
		}

		if jarIdx >= len(oldJarFilesList) {
			// Only local files remain - these are additions
			// Check if this is part of a new directory
			if dir := filepath.Dir(localPath); dir != "." {
				// Check if entire directory is new
				if isCompleteDirectoryNew(localPath, oldLocalFilesList[localIdx:], oldJarFiles) {
					filesAddedToOldLocal = append(filesAddedToOldLocal, dir+string(filepath.Separator))
					// Skip all files in this directory (no need to compute checksums)
					for localIdx < len(oldLocalFilesList) && strings.HasPrefix(oldLocalFilesList[localIdx], dir+string(filepath.Separator)) {
						processedLocalFiles[oldLocalFilesList[localIdx]] = true
						localIdx++
					}
					continue
				}
			}
			processedLocalFiles[localPath] = true
			filesAddedToOldLocal = append(filesAddedToOldLocal, localPath)
			localIdx++
		} else if localIdx >= len(oldLocalFilesList) {
			// Only jar files remain - these are deletions
			if dir := filepath.Dir(jarPath); dir != "." {
				// Check if entire directory was deleted
				if isCompleteDirectoryDeleted(jarPath, oldJarFilesList[jarIdx:], oldLocalFiles) {
					filesDeletedFromOldJar = append(filesDeletedFromOldJar, dir+string(filepath.Separator))
					// Skip all files in this directory
					for jarIdx < len(oldJarFilesList) && strings.HasPrefix(oldJarFilesList[jarIdx], dir+string(filepath.Separator)) {
						processedJarFiles[oldJarFilesList[jarIdx]] = true
						jarIdx++
					}
					continue
				}
			}
			processedJarFiles[jarPath] = true
			filesDeletedFromOldJar = append(filesDeletedFromOldJar, jarPath)
			jarIdx++
		} else {
			// Both lists have files, compare them
			cmp := strings.Compare(jarPath, localPath)
			if cmp == 0 {
				// Same file in both - compute checksum and check if modified
				oldFilePath := filepath.Join(oldLocal, localPath)
				oldLocalChecksum, err := fileChecksum(oldFilePath)
				if err != nil {
					logBoth("Warning: failed to compute checksum for %s: %v\n", localPath, err)
				} else {
					oldLocalFilesWithChecksums[localPath] = oldLocalChecksum // Cache the checksum
					if oldLocalChecksum != oldJarFiles[jarPath] {
						filesModifiedInOldLocal = append(filesModifiedInOldLocal, localPath)
					} else {
						filesUnchangedInOldLocal = append(filesUnchangedInOldLocal, localPath)
					}
				}
				processedJarFiles[jarPath] = true
				processedLocalFiles[localPath] = true
				jarIdx++
				localIdx++
			} else if cmp < 0 {
				// jarPath comes before localPath - it's deleted
				if dir := filepath.Dir(jarPath); dir != "." {
					if isCompleteDirectoryDeleted(jarPath, oldJarFilesList[jarIdx:], oldLocalFiles) {
						filesDeletedFromOldJar = append(filesDeletedFromOldJar, dir+string(filepath.Separator))
						for jarIdx < len(oldJarFilesList) && strings.HasPrefix(oldJarFilesList[jarIdx], dir+string(filepath.Separator)) {
							processedJarFiles[oldJarFilesList[jarIdx]] = true
							jarIdx++
						}
						continue
					}
				}
				processedJarFiles[jarPath] = true
				filesDeletedFromOldJar = append(filesDeletedFromOldJar, jarPath)
				jarIdx++
			} else {
				// localPath comes before jarPath - it's added
				if dir := filepath.Dir(localPath); dir != "." {
					if isCompleteDirectoryNew(localPath, oldLocalFilesList[localIdx:], oldJarFiles) {
						filesAddedToOldLocal = append(filesAddedToOldLocal, dir+string(filepath.Separator))
						for localIdx < len(oldLocalFilesList) && strings.HasPrefix(oldLocalFilesList[localIdx], dir+string(filepath.Separator)) {
							processedLocalFiles[oldLocalFilesList[localIdx]] = true
							localIdx++
						}
						continue
					}
				}
				processedLocalFiles[localPath] = true
				filesAddedToOldLocal = append(filesAddedToOldLocal, localPath)
				localIdx++
			}
		}
	}

	// Log comprehensive summary of changes relative to old owlcms.jar
	logBoth("\n=== Import Analysis: User Changes Relative to Old owlcms.jar ===\n")
	logBoth("Old owlcms.jar (reference): %s\n", oldJar)
	logBoth("Old local directory: %s\n", oldLocal)
	logBoth("\nFiles in old owlcms.jar: %d\n", len(oldJarFiles))
	logBoth("Files in old local directory: %d\n", len(oldLocalFiles))
	logBoth("\nFiles DELETED from old owlcms.jar (in old jar but not in old local): %d\n", len(filesDeletedFromOldJar))
	if len(filesDeletedFromOldJar) > 0 {
		sort.Strings(filesDeletedFromOldJar)
		for _, f := range filesDeletedFromOldJar {
			logBoth("  - DELETED in old local: %s\n", f)
		}
	}
	logBoth("\nFiles ADDED to old local (in old local but not in old jar): %d\n", len(filesAddedToOldLocal))
	if len(filesAddedToOldLocal) > 0 {
		sort.Strings(filesAddedToOldLocal)
		for _, f := range filesAddedToOldLocal {
			logBoth("  + ADDED to old local: %s\n", f)
		}
	}
	logBoth("\nFiles MODIFIED in old local (checksum differs from old jar): %d\n", len(filesModifiedInOldLocal))
	if len(filesModifiedInOldLocal) > 0 {
		sort.Strings(filesModifiedInOldLocal)
		for _, f := range filesModifiedInOldLocal {
			logBoth("  * MODIFIED in old local: %s\n", f)
		}
	}
	logBoth("\nFiles UNCHANGED in old local (same checksum as old jar): %d\n", len(filesUnchangedInOldLocal))
	logBoth("=== End Import Analysis ===\n\n")

	// Now apply the changes to the new version:
	logBoth("\nPhase 2: Extracting fresh reference from new JAR...\n")

	// Step 1: Remove existing local directory to start fresh
	if _, err := os.Stat(newLocal); err == nil {
		logBoth("  - Removing existing local directory: %s\n", newLocal)
		if err := os.RemoveAll(newLocal); err != nil {
			return fmt.Errorf("failed to remove existing local directory: %w", err)
		}
	} else {
		logBoth("  - No existing local directory to remove\n")
	}

	// Step 2: Extract fresh local directory from new JAR to get the new reference state
	logBoth("  - Extracting local/ directory from new JAR...\n")
	newJar := filepath.Join(newDir, "owlcms.jar")
	err = extractLocalFromJar(newJar, newLocal, topLevelDirs)
	if err != nil {
		return fmt.Errorf("failed to extract local from new jar: %w", err)
	}

	// Step 2: Delete files/directories that were deleted in old version
	logBoth("\nPhase 3: Applying deletions (%d items)...\n", len(filesDeletedFromOldJar))
	if len(filesDeletedFromOldJar) > 0 {
		// Log what we're actually deleting, filtering out files that are under already-deleted directories
		deletedDirs := make(map[string]bool)
		for _, deleted := range filesDeletedFromOldJar {
			if strings.HasSuffix(deleted, string(filepath.Separator)) {
				deletedDirs[deleted] = true
			}
		}

		for _, deleted := range filesDeletedFromOldJar {
			// Skip logging files that are under an already-deleted directory
			isUnderDeletedDir := false
			for dir := range deletedDirs {
				if deleted != dir && strings.HasPrefix(deleted, dir) {
					isUnderDeletedDir = true
					break
				}
			}
			if !isUnderDeletedDir {
				logBoth("  - Deleting: %s\n", deleted)
			}
		}
	}
	for _, deleted := range filesDeletedFromOldJar {
		targetPath := filepath.Join(newLocal, deleted)
		if strings.HasSuffix(deleted, string(filepath.Separator)) {
			// It's a directory - remove entire directory
			if err := os.RemoveAll(targetPath); err != nil && !os.IsNotExist(err) {
				logBoth("Warning: failed to remove directory %s: %v\n", deleted, err)
			}
		} else {
			// It's a file
			if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
				logBoth("Warning: failed to remove file %s: %v\n", deleted, err)
			}
		}
	}

	// Step 3: Copy modified files from old version (overwriting new version)
	logBoth("\nPhase 4: Applying modifications (%d files)...\n", len(filesModifiedInOldLocal))
	if len(filesModifiedInOldLocal) > 0 {
		for _, modified := range filesModifiedInOldLocal {
			logBoth("  - Updating: %s\n", modified)
		}
	}
	for _, modified := range filesModifiedInOldLocal {
		oldFilePath := filepath.Join(oldLocal, modified)
		newFilePath := filepath.Join(newLocal, modified)
		if err := copyFile(oldFilePath, newFilePath); err != nil {
			return fmt.Errorf("failed to copy modified file %s: %w", modified, err)
		}
	}

	// Step 4: Copy added files/directories from old version
	logBoth("\nPhase 5: Applying additions (%d items)...\n", len(filesAddedToOldLocal))
	if len(filesAddedToOldLocal) > 0 {
		// Log what we're actually adding, filtering out files that are under already-added directories
		addedDirs := make(map[string]bool)
		for _, added := range filesAddedToOldLocal {
			if strings.HasSuffix(added, string(filepath.Separator)) {
				addedDirs[added] = true
			}
		}

		for _, added := range filesAddedToOldLocal {
			// Skip logging files that are under an already-added directory
			isUnderAddedDir := false
			for dir := range addedDirs {
				if added != dir && strings.HasPrefix(added, dir) {
					isUnderAddedDir = true
					break
				}
			}
			if !isUnderAddedDir {
				logBoth("  + Adding: %s\n", added)
			}
		}
	}
	for _, added := range filesAddedToOldLocal {
		oldPath := filepath.Join(oldLocal, added)
		newPath := filepath.Join(newLocal, added)

		if strings.HasSuffix(added, string(filepath.Separator)) {
			// It's a directory - copy entire directory recursively
			oldDirPath := strings.TrimSuffix(oldPath, string(filepath.Separator))
			newDirPath := strings.TrimSuffix(newPath, string(filepath.Separator))
			if err := copyDirectoryRecursive(oldDirPath, newDirPath); err != nil {
				return fmt.Errorf("failed to copy added directory %s: %w", added, err)
			}
		} else {
			// It's a file
			if err := copyFile(oldPath, newPath); err != nil {
				return fmt.Errorf("failed to copy added file %s: %w", added, err)
			}
		}
	}

	logBoth("\n=== Import Complete ===\n")
	logBoth("Successfully applied all changes from %s to %s\n", oldDir, newDir)
	logBoth("=======================\n")
	return nil
}

// getTopLevelDirs returns the names of top-level directories in dir.
func getTopLevelDirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	return dirs, nil
}

// getJarFilesChecksums returns a map of file paths (relative to local/) to their SHA256 checksums
// for files in the given topLevelDirs inside the jar file.
func getJarFilesChecksums(jarPath string, topLevelDirs []string) (map[string]string, error) {
	result := make(map[string]string)
	r, err := zip.OpenReader(jarPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	for _, f := range r.File {
		// Only consider files under topLevelDirs
		parts := strings.SplitN(f.Name, "/", 2)
		if len(parts) < 2 {
			continue
		}
		topLevel := parts[0]
		if !containsString(topLevelDirs, topLevel) {
			continue
		}
		relPath := filepath.FromSlash(f.Name)
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		sum, err := streamChecksum(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		result[relPath] = sum
	}
	return result, nil
}

// getLocalFiles returns a map of relative file paths that exist in the given directory
func getLocalFiles(dir string, topLevelDirs []string) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		// Only consider files under topLevelDirs
		topLevel := strings.Split(relPath, string(os.PathSeparator))[0]
		if !containsString(topLevelDirs, topLevel) {
			return nil
		}

		result[relPath] = struct{}{}
		return nil
	})
	return result, err
}

// fileChecksum returns the SHA256 checksum of a file.
func fileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return streamChecksum(f)
}

// streamChecksum returns the SHA256 checksum of the data read from r.
func streamChecksum(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// copyFile copies a file from src to dst, overwriting dst if it exists.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// copyDirectoryRecursive copies a directory and all its contents recursively.
func copyDirectoryRecursive(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

// extractLocalFromJar extracts files from specific top-level directories in a JAR to the local directory.
func extractLocalFromJar(jarPath, localDir string, topLevelDirs []string) error {
	r, err := zip.OpenReader(jarPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		// Only extract files under topLevelDirs
		parts := strings.SplitN(f.Name, "/", 2)
		if len(parts) < 2 {
			continue
		}
		topLevel := parts[0]
		if !containsString(topLevelDirs, topLevel) {
			continue
		}

		// Convert to local file path
		relPath := filepath.FromSlash(f.Name)
		targetPath := filepath.Join(localDir, relPath)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, f.Mode()); err != nil {
				return err
			}
			continue
		}

		// Create parent directory
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		// Extract file
		rc, err := f.Open()
		if err != nil {
			return err
		}

		outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

// containsString returns true if s is in list.
func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// isCompleteDirectoryNew checks if the current path is the start of a complete new directory
// by checking if all files in the directory are present in the remaining sorted list and none exist in oldJar.
// Returns true only if the directory itself didn't exist in the JAR (truly new directory).
func isCompleteDirectoryNew(currentPath string, remainingLocalFiles []string, oldJarFiles map[string]string) bool {
	dir := filepath.Dir(currentPath)
	if dir == "." {
		return false
	}

	// Check if this is the first file in the directory
	if filepath.Dir(currentPath) != dir {
		return false
	}

	// First, check if ANY file from this directory existed in oldJarFiles
	// If so, this is a replacement, not a new directory
	for jarPath := range oldJarFiles {
		if strings.HasPrefix(jarPath, dir+string(filepath.Separator)) {
			// Directory existed in JAR, so this is a replacement scenario, not a new directory
			return false
		}
	}

	// Count files in this directory from remaining list
	fileCount := 0
	for _, path := range remainingLocalFiles {
		if strings.HasPrefix(path, dir+string(filepath.Separator)) {
			fileCount++
		} else if strings.Compare(path, dir+string(filepath.Separator)+"\xff") > 0 {
			// Past this directory
			break
		}
	}

	// Only consolidate if there are multiple files AND the directory didn't exist in JAR
	return fileCount > 1
}

// isCompleteDirectoryDeleted checks if the current path is the start of a complete deleted directory
// by checking if all files in the directory from the jar are not present in oldLocal.
func isCompleteDirectoryDeleted(currentPath string, remainingJarFiles []string, oldLocalFiles map[string]struct{}) bool {
	dir := filepath.Dir(currentPath)
	if dir == "." {
		return false
	}

	// Count files in this directory from remaining jar list
	fileCount := 0
	for _, path := range remainingJarFiles {
		if strings.HasPrefix(path, dir+string(filepath.Separator)) {
			// Check if any file in this directory exists in oldLocalFiles
			if _, exists := oldLocalFiles[path]; exists {
				return false
			}
			fileCount++
		} else if strings.Compare(path, dir+string(filepath.Separator)+"\xff") > 0 {
			// Past this directory
			break
		}
	}

	// Only consolidate if there are multiple files
	return fileCount > 1
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
	log.Println("Reinitializing version list")
	versionContainer.Objects = nil // Clear the container
	newVersionList := createVersionList(w, stopButton)

	numVersions := len(getAllInstalledVersions())
	versionScroll := container.NewVScroll(newVersionList)
	// Ensure the version scroll has enough height to display up to 4 rows
	versionScroll.SetMinSize(fyne.NewSize(0, computeVersionScrollHeight(numVersions)))
	center := container.NewStack(versionScroll)

	if numVersions == 0 {
		// Reset the tab to explanation mode so the bottom download UI is cleared first
		resetToExplainMode(w)
		// No version list to add — return now so we don't overwrite the explanation
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
	log.Println("Version list reinitialized")
}
