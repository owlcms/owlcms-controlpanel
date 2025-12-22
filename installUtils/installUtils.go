package installUtils

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"owlcms-launcher/downloadUtils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/Masterminds/semver/v3"
)

// ProcessLocalZipFile handles a ZIP file selected from the file system
func ProcessLocalZipFile(zipPath string, w fyne.Window, owlcmsInstallDir string,
	copyFile func(string, string) error,
	updateExplanation func(),
	recomputeVersionList func(fyne.Window),
	checkForNewerVersion func()) {
	// Extract version number from filename if possible
	fileName := filepath.Base(zipPath)
	version := ""

	// Try to extract version from filename if possible
	// Handle formats (PREFIX is optional):
	// - "VERSION.zip"
	// - "PREFIX[-_]VERSION.zip"
	// - "owlcms_VERSION_YYYY-MM-DD_HHMMSS.zip" (export format with timestamp)
	// PREFIX: alphanumeric starting with a letter and ending with - or _
	// VERSION: full semver like "1.2.3-rc.1+metadata"
	// TIMESTAMP: [._]YYYY-MM-DD_HHMMSS where separator before date can be . or _
	if strings.HasSuffix(fileName, ".zip") {
		// Remove .zip extension
		nameWithoutExt := strings.TrimSuffix(fileName, ".zip")

		// Remove any optional prefix matching pattern: letter followed by alphanumerics, ending with - or _
		prefixRegex := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*[-_]`)
		nameWithoutExt = prefixRegex.ReplaceAllString(nameWithoutExt, "")

		// Strip the export timestamp format: [._]YYYY-MM-DD_HHMMSS
		dateTimeRegex := regexp.MustCompile(`[._]\d{4}-\d{2}-\d{2}_\d{6}$`)
		if dateTimeRegex.MatchString(nameWithoutExt) {
			nameWithoutExt = dateTimeRegex.ReplaceAllString(nameWithoutExt, "")
		}

		// Check if what remains is a valid semver
		if IsValidSemVer(nameWithoutExt) {
			version = nameWithoutExt
		}
	}

	// If version couldn't be determined or is invalid, ask the user
	if version == "" || !IsValidSemVer(version) {
		content := widget.NewEntry()
		content.SetPlaceHolder("e.g., 4.24.1")

		message := widget.NewLabel("Could not identify a version number in the file name, please provide one")
		message.Wrapping = fyne.TextWrapWord

		formContent := container.NewVBox(message, content)

		versionDialog := dialog.NewCustomConfirm(
			"Enter Version",
			"Install",
			"Cancel",
			formContent,
			func(confirmed bool) {
				if !confirmed || content.Text == "" {
					return
				}

				if IsValidSemVer(content.Text) {
					InstallLocalZipFile(zipPath, content.Text, w, owlcmsInstallDir, copyFile, updateExplanation, recomputeVersionList, checkForNewerVersion)
				} else {
					dialog.ShowError(fmt.Errorf("invalid version format, please use semantic versioning (e.g., 4.24.1)"), w)
				}
			},
			w,
		)
		versionDialog.Show()
	} else {
		// We have a valid version, proceed with installation
		InstallLocalZipFile(zipPath, version, w, owlcmsInstallDir, copyFile, updateExplanation, recomputeVersionList, checkForNewerVersion)
	}
}

// IsValidSemVer checks if a string is a valid semantic version
func IsValidSemVer(version string) bool {
	_, err := semver.NewVersion(version)
	return err == nil
}

// IsAllDigits checks if a string contains only digits
func IsAllDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// GetInstallationDirectoryName determines the directory name for installing a version,
// handling collisions by adding version suffixes to metadata.
// For versions with metadata (e.g., 1.0.0-rc02+fhq):
//   - First collision: 1.0.0-rc02+fhq.01
//   - Second collision: 1.0.0-rc02+fhq.02
//
// For versions without metadata (e.g., 1.0.0):
//   - First collision: 1.0.0+01
//   - Second collision: 1.0.0+02
//
// Returns the final directory name (just the name, not the full path)
func GetInstallationDirectoryName(baseVersion string, owlcmsInstallDir string) string {
	basePath := filepath.Join(owlcmsInstallDir, baseVersion)

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
	entries, err := os.ReadDir(owlcmsInstallDir)
	if err != nil {
		// If we can't read the directory, just use .01
		// (the installation will fail with a more appropriate error if there's a real issue)
		maxSuffix = 0
	} else {
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
					// Extract the number suffix
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

	// Build the next version number
	nextSuffix := maxSuffix + 1
	if metadata != "" {
		return fmt.Sprintf("%s%s.%02d", versionPrefix, metadata, nextSuffix)
	}
	return fmt.Sprintf("%s%02d", versionPrefix, nextSuffix)
}

// InstallLocalZipFile installs from a local ZIP file
func InstallLocalZipFile(zipPath, version string, w fyne.Window, owlcmsInstallDir string,
	copyFile func(string, string) error,
	updateExplanation func(),
	recomputeVersionList func(fyne.Window),
	checkForNewerVersion func()) {
	// Create a custom progress dialog
	progressBar := widget.NewProgressBar()
	progressBar.SetValue(0.1)
	messageLabel := widget.NewLabel(fmt.Sprintf("Installing OWLCMS %s from local file...", version))
	content := container.NewVBox(messageLabel, progressBar)
	progressDialog := dialog.NewCustom(
		"Installing OWLCMS",
		"Please wait...",
		content,
		w)
	progressDialog.Show()

	// Ensure the owlcms directory exists
	owlcmsDir := owlcmsInstallDir
	if _, err := os.Stat(owlcmsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(owlcmsDir, 0755); err != nil {
			progressDialog.Hide()
			dialog.ShowError(fmt.Errorf("creating owlcms directory: %w", err), w)
			return
		}
	}

	originalFileName := filepath.Base(zipPath)
	destOriginalPath := filepath.Join(owlcmsDir, originalFileName)

	messageLabel.SetText("Copying ZIP file...")
	messageLabel.Refresh()

	progressBar.SetValue(0.3)

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
		// Determine final installation directory name, handling collisions
		finalVersionName := GetInstallationDirectoryName(version, owlcmsInstallDir)
		finalExtractPath := filepath.Join(owlcmsDir, finalVersionName)

		progressBar.SetValue(0.5)
		messageLabel.SetText("Extracting files...")
		messageLabel.Refresh()

		// Use the original copied file for extraction
		log.Printf("Extracting ZIP file to: %s\n", finalExtractPath)
		err := downloadUtils.ExtractZip(destOriginalPath, finalExtractPath)
		if err != nil {
			progressDialog.Hide()
			dialog.ShowError(fmt.Errorf("extraction failed: %w", err), w)
			return
		}

		// Set to complete
		progressBar.SetValue(1.0)

		// Log when extraction is done
		log.Println("Extraction completed")
		updateExplanation()

		// Hide progress dialog
		progressDialog.Hide()

		// Show success panel with installation details
		message := fmt.Sprintf(
			"Successfully installed OWLCMS version %s\n\n"+
				"Location: %s\n\n"+
				"The program files have been extracted to the above directory.\n\n",
			finalVersionName, finalExtractPath)

		dialog.ShowInformation("Installation Complete", message, w)

		// Recompute the version list
		recomputeVersionList(w)

		// Recompute the downloadTitle
		checkForNewerVersion()
	}()
}

// ZipCurrentSetup creates a ZIP file of a selected installed version
func ZipCurrentSetup(w fyne.Window, owlcmsInstallDir string,
	getAllInstalledVersions func() []string,
	selectSaveZip func(fyne.Window, string, func(string, error))) {
	versions := getAllInstalledVersions()
	if len(versions) == 0 {
		dialog.ShowError(fmt.Errorf("no versions installed to zip"), w)
		return
	}

	// Create a dialog to select which version to zip
	versionSelect := widget.NewSelect(versions, func(selected string) {})
	if len(versions) == 1 {
		versionSelect.Selected = versions[0]
	}

	dialog.ShowForm("Zip Current Setup",
		"Create ZIP",
		"Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Select version to zip", versionSelect),
		},
		func(ok bool) {
			if !ok || versionSelect.Selected == "" {
				return
			}

			version := versionSelect.Selected
			sourceDir := filepath.Join(owlcmsInstallDir, version)

			// Check if directory exists
			if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
				dialog.ShowError(fmt.Errorf("version directory does not exist: %s", version), w)
				return
			}

			// Create filename with version and timestamp in ISO format
			timestamp := time.Now().Format("2006-01-02_150405")
			zipFileName := fmt.Sprintf("owlcms_%s.%s.zip", version, timestamp)

			// Ask user where to save the zip file using platform-specific dialog
			selectSaveZip(w, zipFileName, func(zipPath string, err error) {
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to select save location: %w", err), w)
					return
				}
				if zipPath == "" {
					// User cancelled
					return
				}

				// Create progress dialog
				progressBar := widget.NewProgressBar()
				messageLabel := widget.NewLabel(fmt.Sprintf("Creating ZIP file for version %s...", version))
				progressContent := container.NewVBox(messageLabel, progressBar)
				progressDialog := dialog.NewCustom(
					"Creating ZIP",
					"Please wait...",
					progressContent,
					w)
				progressDialog.Show()

				go func() {
					defer progressDialog.Hide()

					// Create the zip file
					err := CreateZipArchive(sourceDir, zipPath, func(progress float64) {
						progressBar.SetValue(progress)
					})

					if err != nil {
						dialog.ShowError(fmt.Errorf("failed to create ZIP file: %w", err), w)
						return
					}

					dialog.ShowInformation("Success",
						fmt.Sprintf("Successfully created ZIP file:\n%s", zipPath), w)
				}()
			})
		},
		w)
}

// CreateZipArchive creates a zip file from a directory
func CreateZipArchive(sourceDir, zipPath string, progressCallback func(float64)) error {
	// Count total files first for progress tracking
	var totalFiles int
	filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			totalFiles++
		}
		return nil
	})

	zipFile, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	var processedFiles int
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Create zip header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// Use forward slashes in zip paths
		header.Name = filepath.ToSlash(relPath)

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(writer, file)
			if err != nil {
				return err
			}

			processedFiles++
			if progressCallback != nil && totalFiles > 0 {
				progressCallback(float64(processedFiles) / float64(totalFiles))
			}
		}

		return nil
	})

	return err
}
