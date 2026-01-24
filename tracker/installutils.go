package tracker

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"owlcms-launcher/shared"
	"owlcms-launcher/tracker/downloadutils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// ProcessLocalZipFile handles a ZIP file selected from the file system.
func ProcessLocalZipFile(zipPath string, w fyne.Window, trackerInstallDir string,
	updateExplanation func(),
	recomputeVersionList func(fyne.Window),
	checkForNewerVersion func()) {
	shared.ProcessLocalZipFile(zipPath, w, "1.2.3", func(zipPath, version string) {
		InstallLocalZipFile(zipPath, version, w, trackerInstallDir, updateExplanation, recomputeVersionList, checkForNewerVersion)
	})
}

// GetInstallationDirectoryName determines the directory name for installing a version,
// handling collisions by adding version suffixes to metadata.
func GetInstallationDirectoryName(baseVersion string, trackerInstallDir string) string {
	basePath := filepath.Join(trackerInstallDir, baseVersion)

	// Check if the base version directory already exists
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		// No collision, use base version as-is
		return baseVersion
	}

	// Collision detected - need to add version suffix
	// Determine if version has metadata
	var versionPrefix, metadata string
	if strings.Contains(baseVersion, "+") {
		// Has metadata: split on + and build pattern with .NN suffix on metadata
		parts := strings.Split(baseVersion, "+")
		versionPrefix = parts[0] + "+"
		metadata = parts[1]
	} else {
		// No metadata: we'll add +NN
		versionPrefix = baseVersion + "+"
		metadata = ""
	}

	// Scan for existing versioned directories
	maxSuffix := 0
	entries, err := os.ReadDir(trackerInstallDir)
	if err == nil {
		// Build pattern to match existing versioned directories
		var pattern *regexp.Regexp
		if metadata != "" {
			// Pattern: versionPrefix + metadata + .NN (e.g., "1.0.0-rc02+fhq.01")
			escapedMetadata := regexp.QuoteMeta(metadata)
			pattern = regexp.MustCompile(fmt.Sprintf(`^%s%s\.(\d{2})$`,
				regexp.QuoteMeta(versionPrefix), escapedMetadata))
		} else {
			// Pattern: versionPrefix + NN (e.g., "1.0.0+01")
			pattern = regexp.MustCompile(fmt.Sprintf(`^%s(\d{2})$`,
				regexp.QuoteMeta(versionPrefix)))
		}

		for _, entry := range entries {
			if entry.IsDir() {
				matches := pattern.FindStringSubmatch(entry.Name())
				if len(matches) == 2 {
					var suffix int
					if _, err := fmt.Sscanf(matches[1], "%d", &suffix); err == nil {
						if suffix > maxSuffix {
							maxSuffix = suffix
						}
					}
				}
			}
		}
	}

	nextSuffix := maxSuffix + 1
	if metadata != "" {
		return fmt.Sprintf("%s%s.%02d", versionPrefix, metadata, nextSuffix)
	}
	return fmt.Sprintf("%s%02d", versionPrefix, nextSuffix)
}

// InstallLocalZipFile installs Tracker from a local ZIP file.
func InstallLocalZipFile(zipPath, version string, w fyne.Window, trackerInstallDir string,
	updateExplanation func(),
	recomputeVersionList func(fyne.Window),
	checkForNewerVersion func()) {
	shared.PromptForInstallVersionName(trackerInstallDir, version, w, func(finalVersionName string) {
		progressBar := widget.NewProgressBar()
		progressBar.SetValue(0.01)
		messageLabel := widget.NewLabel(fmt.Sprintf("Installing Tracker %s from local file...", finalVersionName))
		content := container.NewVBox(messageLabel, progressBar)
		progressDialog := dialog.NewCustom(
			"Installing Tracker",
			"Please wait...",
			content,
			w)
		progressDialog.Show()

		// Ensure the tracker directory exists
		trackerDir := trackerInstallDir
		if _, err := os.Stat(trackerDir); os.IsNotExist(err) {
			if err := shared.EnsureDir0755(trackerDir); err != nil {
				progressDialog.Hide()
				dialog.ShowError(fmt.Errorf("creating tracker directory: %w", err), w)
				return
			}
		}

		originalFileName := filepath.Base(zipPath)
		destOriginalPath := filepath.Join(trackerDir, originalFileName)

		messageLabel.SetText("Copying ZIP file...")
		messageLabel.Refresh()

		progressBar.SetValue(0.02)

		// Preserve a copy of the ZIP file in the installation directory for reference
		if zipPath != destOriginalPath {
			err := copyFile(zipPath, destOriginalPath)
			if err != nil {
				progressDialog.Hide()
				dialog.ShowError(fmt.Errorf("failed to copy ZIP file: %w", err), w)
				return
			}
		}

		go func() {
			finalExtractPath := filepath.Join(trackerDir, finalVersionName)

			messageLabel.SetText("Extracting files...")
			messageLabel.Refresh()

			log.Printf("Extracting ZIP file to: %s\n", finalExtractPath)
			extractProgress := func(extracted, total int64) {
				if total > 0 {
					progressBar.SetValue(float64(extracted) / float64(total))
				}
			}
			err := downloadutils.ExtractZip(destOriginalPath, finalExtractPath, extractProgress)
			if err != nil {
				progressDialog.Hide()
				dialog.ShowError(fmt.Errorf("extraction failed: %w", err), w)
				return
			}

			progressBar.SetValue(1.0)
			log.Println("Extraction completed")

			// Ensure the tab UI is initialized so download UI widgets exist
			initializeTab(w)
			updateExplanation()

			progressDialog.Hide()

			message := fmt.Sprintf(
				"Successfully installed Tracker version %s\n\n"+
					"Location: %s\n\n"+
					"The program files have been extracted to the above directory.",
				finalVersionName, finalExtractPath)

			dialog.ShowInformation("Installation Complete", message, w)

			recomputeVersionList(w)
			checkForNewerVersion()
		}()
	})
}

// copyFile copies a file from src to dst, overwriting dst if it exists.
func copyFile(src, dst string) error {
	if err := shared.EnsureDir0755(filepath.Dir(dst)); err != nil {
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
