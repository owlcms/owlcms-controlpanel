package shared

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/Masterminds/semver/v3"
)

// CreateDuplicateButton creates a duplicate button for a version.
// installDir is the base directory containing version folders.
// version is the version name to duplicate.
// w is the parent window.
// buttonContainer is the container to add the button to.
// onSuccess is called after successful duplication with the new version name.
func CreateDuplicateButton(installDir, version string, w fyne.Window, buttonContainer *fyne.Container, onSuccess func(newVersion string)) {
	duplicateButton := widget.NewButton("Duplicate", nil)
	duplicateButton.OnTapped = func() {
		ShowDuplicateVersionDialog(installDir, version, w, onSuccess)
	}
	buttonContainer.Add(container.NewPadded(duplicateButton))
}

// CreateRenameButton creates a rename button for a version.
// installDir is the base directory containing version folders.
// version is the version name to rename.
// w is the parent window.
// buttonContainer is the container to add the button to.
// onSuccess is called after successful rename with the new version name.
func CreateRenameButton(installDir, version string, w fyne.Window, buttonContainer *fyne.Container, onSuccess func(newVersion string)) {
	renameButton := widget.NewButton("Rename", nil)
	renameButton.OnTapped = func() {
		ShowRenameVersionDialog(installDir, version, w, onSuccess)
	}
	buttonContainer.Add(container.NewPadded(renameButton))
}

// ShowRenameVersionDialog displays a dialog to rename a version.
// installDir is the base directory containing version folders.
// version is the current version name.
// w is the parent window.
// onSuccess is called after successful rename with the new version name.
func ShowRenameVersionDialog(installDir, version string, w fyne.Window, onSuccess func(newVersion string)) {
	// Create entry widget for the new build identifier
	buildEntry := widget.NewEntry()
	buildEntry.SetPlaceHolder("e.g., test, backup, 1, myversion")
	buildEntry.SetText(GetCurrentBuildString(version))

	// Variable to hold the dialog so we can close it from ENTER handler
	var d dialog.Dialog

	// Function to perform the rename
	doRename := func() {
		newBuild := buildEntry.Text

		// If not empty, validate the build metadata
		if newBuild != "" {
			// Validate the new build metadata by attempting to construct and parse the new version
			baseVersion, _ := ParseVersionWithBuild(version)
			sanitizedBuild := SanitizeVersionBuild(newBuild)
			if sanitizedBuild == "" {
				dialog.ShowError(fmt.Errorf("build identifier '%s' becomes empty after sanitization", newBuild), w)
				return
			}

			// Test if the resulting version string is valid semver
			testVersion := fmt.Sprintf("%s+%s", baseVersion, sanitizedBuild)
			if _, err := semver.NewVersion(testVersion); err != nil {
				dialog.ShowError(fmt.Errorf("invalid build identifier '%s': %w", newBuild, err), w)
				return
			}
		}

		newVersion, err := RenameVersion(installDir, version, newBuild)
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to rename version: %w", err), w)
			return
		}

		if d != nil {
			d.Hide()
		}

		dialog.ShowInformation("Rename Complete",
			fmt.Sprintf("Successfully renamed %s to %s", version, newVersion), w)

		if onSuccess != nil {
			onSuccess(newVersion)
		}
	}

	// Handle ENTER key to trigger rename
	buildEntry.OnSubmitted = func(text string) {
		doRename()
	}

	// Create the form content with label on left, entry on right
	label := widget.NewLabel("New Name")
	entryContainer := container.NewGridWrap(fyne.NewSize(300, 35), buildEntry)
	formRow := container.NewHBox(label, entryContainer)

	noteLabel := widget.NewLabel("Note: Spaces become dots, only ASCII alphanumerics allowed")
	noteLabel.TextStyle = fyne.TextStyle{Italic: true}

	content := container.NewVBox(
		formRow,
		noteLabel,
	)

	// Create custom dialog
	d = dialog.NewCustomConfirm("Rename Version", "Rename", "Cancel", content,
		func(ok bool) {
			if !ok {
				return
			}
			doRename()
		}, w)
	d.Show()

	// Focus the entry widget after showing dialog
	w.Canvas().Focus(buildEntry)
}

// ShowDuplicateVersionDialog displays a dialog to duplicate a version with a new name.
// installDir is the base directory containing version folders.
// version is the current version name.
// w is the parent window.
// onSuccess is called after successful duplication with the new version name.
func ShowDuplicateVersionDialog(installDir, version string, w fyne.Window, onSuccess func(newVersion string)) {
	// Get current build metadata and compute the collision-resolved name
	baseVersion, currentBuild := ParseVersionWithBuild(version)
	suggestedBuild := ResolveCollisionForBuild(installDir, baseVersion, currentBuild)

	// Create entry widget for the new build identifier
	buildEntry := widget.NewEntry()
	buildEntry.SetPlaceHolder("e.g., test, backup, copy")
	buildEntry.SetText(suggestedBuild)

	// Variable to hold the dialog so we can close it from ENTER handler
	var d dialog.Dialog

	// Create a status label that we can update
	statusLabel := widget.NewLabel("")
	progressBar := widget.NewProgressBar()
	progressBar.Hide()

	// Create the form content with label on left, entry on right
	label := widget.NewLabel("New Name")
	entryContainer := container.NewGridWrap(fyne.NewSize(300, 35), buildEntry)
	formRow := container.NewHBox(label, entryContainer)

	noteLabel := widget.NewLabel("Note: Spaces become dots, only ASCII alphanumerics allowed")
	noteLabel.TextStyle = fyne.TextStyle{Italic: true}

	// Variables to hold buttons so we can modify them
	var duplicateBtn *widget.Button
	var cancelBtn *widget.Button

	// Function to perform the duplication
	doDuplicate := func() {
		newBuild := buildEntry.Text

		// If not empty, validate the build metadata
		if newBuild != "" {
			// Validate the new build metadata by attempting to construct and parse the new version
			baseVersion, _ := ParseVersionWithBuild(version)
			sanitizedBuild := SanitizeVersionBuild(newBuild)
			if sanitizedBuild == "" {
				dialog.ShowError(fmt.Errorf("build identifier '%s' becomes empty after sanitization", newBuild), w)
				return
			}

			// Test if the resulting version string is valid semver
			testVersion := fmt.Sprintf("%s+%s", baseVersion, sanitizedBuild)
			if _, err := semver.NewVersion(testVersion); err != nil {
				dialog.ShowError(fmt.Errorf("invalid build identifier '%s': %w", newBuild, err), w)
				return
			}
		}

		// Disable buttons and entry during duplication
		duplicateBtn.Disable()
		cancelBtn.Disable()
		buildEntry.Disable()

		// Show progress message
		statusLabel.SetText("Duplicating...")
		statusLabel.Refresh()
		progressBar.SetValue(0)
		progressBar.Show()
		progressBar.Refresh()
		stopProgress := startTimedProgress(progressBar, 0, 1, 5*time.Second)

		// Perform duplication in background
		go func() {
			newVersion, err := DuplicateVersionWithName(installDir, version, newBuild)

			if err != nil {
				// Re-enable on error
				statusLabel.SetText("")
				statusLabel.Refresh()
				stopProgress()
				progressBar.Hide()
				buildEntry.Enable()
				duplicateBtn.Enable()
				cancelBtn.Enable()
				dialog.ShowError(fmt.Errorf("failed to duplicate version: %w", err), w)
				return
			}

			// Show success message
			stopProgress()
			progressBar.SetValue(1)
			progressBar.Refresh()
			statusLabel.SetText(fmt.Sprintf("Successfully created %s as a duplicate of %s", newVersion, version))
			statusLabel.Refresh()

			// Hide the form fields and note
			formRow.Hide()
			noteLabel.Hide()

			// Change button to "Close"
			duplicateBtn.SetText("Close")
			duplicateBtn.OnTapped = func() {
				if d != nil {
					d.Hide()
				}
				if onSuccess != nil {
					onSuccess(newVersion)
				}
			}
			duplicateBtn.Enable()
			duplicateBtn.Refresh()
		}()
	}

	// Handle ENTER key to trigger duplication
	buildEntry.OnSubmitted = func(text string) {
		doDuplicate()
	}

	content := container.NewVBox(
		formRow,
		noteLabel,
		progressBar,
		statusLabel,
	)

	// Create buttons manually to control dialog closing behavior
	duplicateBtn = widget.NewButton("Duplicate", func() {
		doDuplicate()
	})
	cancelBtn = widget.NewButton("Cancel", func() {
		if d != nil {
			d.Hide()
		}
	})

	buttons := container.NewHBox(
		layout.NewSpacer(),
		cancelBtn,
		duplicateBtn,
	)

	dialogContent := container.NewVBox(
		content,
		buttons,
	)

	// Create custom dialog that won't auto-close
	d = dialog.NewCustomWithoutButtons("Duplicate Version", dialogContent, w)
	d.Show()

	// Focus the entry widget after showing dialog
	w.Canvas().Focus(buildEntry)
}

// PromptForInstallVersionName checks for a version collision and prompts for a new build name when needed.
// If no collision, it calls onConfirm with the original version.
func PromptForInstallVersionName(installDir, version string, w fyne.Window, onConfirm func(newVersion string)) {
	if version == "" {
		onConfirm(version)
		return
	}

	basePath := filepath.Join(installDir, version)
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		onConfirm(version)
		return
	}

	baseVersion, currentBuild := ParseVersionWithBuild(version)
	suggestedBuild := ResolveCollisionForBuild(installDir, baseVersion, currentBuild)

	buildEntry := widget.NewEntry()
	buildEntry.SetPlaceHolder("e.g., test, backup, 1, myversion")
	buildEntry.SetText(suggestedBuild)

	message := widget.NewLabel("This version name already exists. Please enter a new build identifier to install this copy.")
	message.Wrapping = fyne.TextWrapWord

	label := widget.NewLabel("New Name")
	entryContainer := container.NewGridWrap(fyne.NewSize(300, 35), buildEntry)
	formRow := container.NewHBox(label, entryContainer)

	noteLabel := widget.NewLabel("Note: Spaces become dots, only ASCII alphanumerics allowed")
	noteLabel.TextStyle = fyne.TextStyle{Italic: true}

	content := container.NewVBox(
		message,
		formRow,
		noteLabel,
	)

	var d dialog.Dialog
	confirm := func() {
		newBuild := SanitizeVersionBuild(buildEntry.Text)
		if newBuild == "" {
			dialog.ShowError(fmt.Errorf("new name cannot be empty"), w)
			return
		}

		testVersion := fmt.Sprintf("%s+%s", baseVersion, newBuild)
		if _, err := semver.NewVersion(testVersion); err != nil {
			dialog.ShowError(fmt.Errorf("invalid build identifier '%s': %w", newBuild, err), w)
			return
		}

		if _, err := os.Stat(filepath.Join(installDir, testVersion)); err == nil {
			dialog.ShowError(fmt.Errorf("version %s already exists", testVersion), w)
			return
		}

		if d != nil {
			d.Hide()
		}
		onConfirm(testVersion)
	}

	buildEntry.OnSubmitted = func(text string) {
		confirm()
	}

	d = dialog.NewCustomConfirm("Rename Version", "Continue", "Cancel", content, func(ok bool) {
		if !ok {
			return
		}
		confirm()
	}, w)
	d.Show()
	w.Canvas().Focus(buildEntry)
}

// startTimedProgress updates the progress bar from start to end over the given duration.
// It returns a stop function to halt updates when the task completes early.
func startTimedProgress(bar *widget.ProgressBar, start, end float64, duration time.Duration) func() {
	if bar == nil {
		return func() {}
	}
	if end < start {
		end = start
	}
	startTime := time.Now()
	ticker := time.NewTicker(200 * time.Millisecond)
	done := make(chan struct{})

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				elapsed := time.Since(startTime)
				frac := float64(elapsed) / float64(duration)
				if frac > 1 {
					frac = 1
				}
				value := start + (end-start)*frac
				bar.SetValue(value)
				if frac >= 1 {
					return
				}
			}
		}
	}()

	return func() {
		select {
		case <-done:
			return
		default:
			close(done)
		}
	}
}

// FindNextUniqueVersionName finds the next unique version name by adding +1, +2, +3, etc.
// If the version already has a +N suffix, it increments it. Otherwise, it starts with +1.
func FindNextUniqueVersionName(baseDir, version string) (string, error) {
	// Parse the version to extract the base and any existing +N suffix
	baseVersion, buildMeta := ParseVersionWithBuild(version)

	// Try to parse build metadata as a number, or start from 1
	buildNum := 1
	if buildMeta != "" {
		if num, err := strconv.Atoi(buildMeta); err == nil {
			buildNum = num + 1
		}
	}

	// Try successive numbers until we find one that doesn't exist
	for {
		newVersion := fmt.Sprintf("%s+%d", baseVersion, buildNum)
		newPath := filepath.Join(baseDir, newVersion)
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newVersion, nil
		}
		buildNum++

		// Safety check to avoid infinite loop
		if buildNum > 1000 {
			return "", fmt.Errorf("could not find unique version name after 1000 attempts")
		}
	}
}

// ParseVersionWithBuild parses a version string and extracts the base version and build metadata.
// For example: "1.2.3+4" returns ("1.2.3", "4"), "1.2.3+test.1" returns ("1.2.3", "test.1"), "1.2.3" returns ("1.2.3", "")
// Uses semver library to properly parse build metadata which can contain dots and alphanumerics.
func ParseVersionWithBuild(version string) (string, string) {
	v, err := semver.NewVersion(version)
	if err != nil {
		// If parsing fails, try simple split as fallback
		parts := strings.Split(version, "+")
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
		return version, ""
	}

	// Get the base version without build metadata
	baseVersion := fmt.Sprintf("%d.%d.%d", v.Major(), v.Minor(), v.Patch())
	if v.Prerelease() != "" {
		baseVersion += "-" + v.Prerelease()
	}

	// Get build metadata
	buildMetadata := v.Metadata()

	return baseVersion, buildMetadata
}

// ResolveCollisionForBuild computes the collision-resolved build metadata for a version.
// If there's no collision, returns the original build metadata.
// If there's a collision, applies auto-increment logic.
func ResolveCollisionForBuild(installDir, baseVersion, currentBuild string) string {
	// Construct the test version
	var testVersion string
	if currentBuild == "" {
		testVersion = baseVersion
	} else {
		testVersion = fmt.Sprintf("%s+%s", baseVersion, currentBuild)
	}

	// Check if this would collide
	testPath := filepath.Join(installDir, testVersion)
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		// No collision - return original
		return currentBuild
	}

	// Collision detected - compute the auto-incremented name
	resolvedBuild := currentBuild

	if currentBuild == "" {
		// No build suffix - would become +1, +2, +3, etc.
		for i := 1; i < 1000; i++ {
			testVersion = fmt.Sprintf("%s+%d", baseVersion, i)
			testPath = filepath.Join(installDir, testVersion)
			if _, err := os.Stat(testPath); os.IsNotExist(err) {
				resolvedBuild = strconv.Itoa(i)
				break
			}
		}
	} else {
		// Has build suffix - check last dot-separated identifier
		parts := strings.Split(currentBuild, ".")
		lastPart := parts[len(parts)-1]

		if collisionNum, err := strconv.Atoi(lastPart); err == nil && len(lastPart) == 1 && collisionNum < 9 {
			// Single-digit 0-8 - try to increment
			baseParts := parts[:len(parts)-1]
			foundFree := false
			for i := collisionNum + 1; i < 10; i++ {
				if len(baseParts) > 0 {
					resolvedBuild = strings.Join(baseParts, ".") + "." + strconv.Itoa(i)
				} else {
					resolvedBuild = strconv.Itoa(i)
				}
				testVersion = fmt.Sprintf("%s+%s", baseVersion, resolvedBuild)
				testPath = filepath.Join(installDir, testVersion)
				if _, err := os.Stat(testPath); os.IsNotExist(err) {
					foundFree = true
					break
				}
			}
			if !foundFree {
				// All single digits taken, append .1
				for i := 1; i < 1000; i++ {
					resolvedBuild = fmt.Sprintf("%s.%d", currentBuild, i)
					testVersion = fmt.Sprintf("%s+%s", baseVersion, resolvedBuild)
					testPath = filepath.Join(installDir, testVersion)
					if _, err := os.Stat(testPath); os.IsNotExist(err) {
						break
					}
				}
			}
		} else {
			// Multi-digit, 9, or non-numeric - append .1
			for i := 1; i < 1000; i++ {
				resolvedBuild = fmt.Sprintf("%s.%d", currentBuild, i)
				testVersion = fmt.Sprintf("%s+%s", baseVersion, resolvedBuild)
				testPath = filepath.Join(installDir, testVersion)
				if _, err := os.Stat(testPath); os.IsNotExist(err) {
					break
				}
			}
		}
	}

	return resolvedBuild
}

// SanitizeVersionBuild sanitizes a user-provided build identifier to be semver-compliant.
// Spaces become dots, and only ASCII alphanumerics, dots, and hyphens are kept.
func SanitizeVersionBuild(input string) string {
	// Replace spaces with dots
	input = strings.ReplaceAll(input, " ", ".")

	// Build the sanitized string keeping only allowed characters
	var result strings.Builder
	for _, r := range input {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			// Keep ASCII letters and digits
			if r <= unicode.MaxASCII {
				result.WriteRune(r)
			}
		} else if r == '.' || r == '-' {
			// Keep dots and hyphens
			result.WriteRune(r)
		}
	}

	sanitized := result.String()

	// Remove leading/trailing dots and hyphens
	sanitized = strings.Trim(sanitized, ".-")

	// Collapse multiple consecutive dots or hyphens
	for strings.Contains(sanitized, "..") {
		sanitized = strings.ReplaceAll(sanitized, "..", ".")
	}
	for strings.Contains(sanitized, "--") {
		sanitized = strings.ReplaceAll(sanitized, "--", "-")
	}

	return sanitized
}

// RenameVersion renames a version by updating the build metadata after the +.
// If newBuild is empty, removes the build metadata. If there's a collision, auto-increments.
// Returns the new version name.
func RenameVersion(baseDir, oldVersion, newBuild string) (string, error) {
	baseVersion, _ := ParseVersionWithBuild(oldVersion)

	// Sanitize the new build identifier (empty is allowed)
	newBuild = SanitizeVersionBuild(newBuild)

	// Use collision resolution to get the final build metadata
	resolvedBuild := ResolveCollisionForBuild(baseDir, baseVersion, newBuild)

	// Create the new version name
	var newVersion string
	if resolvedBuild == "" {
		newVersion = baseVersion
	} else {
		newVersion = fmt.Sprintf("%s+%s", baseVersion, resolvedBuild)
	}

	if resolvedBuild != newBuild {
		log.Printf("RenameVersion: resolved collision from %s to %s", newBuild, resolvedBuild)
	}

	// Construct full paths
	oldPath := filepath.Join(baseDir, oldVersion)
	newPath := filepath.Join(baseDir, newVersion)

	// Log the current working directory and full paths before rename
	cwd, _ := os.Getwd()
	log.Printf("RenameVersion: current working directory: %s", cwd)
	log.Printf("RenameVersion: baseDir: %s", baseDir)
	log.Printf("RenameVersion: attempting to rename:")
	log.Printf("  FROM: %s", oldPath)
	log.Printf("  TO:   %s", newPath)

	// Change to the base directory to avoid locking the directory being renamed
	// This is needed on Windows where a directory cannot be renamed if it's the current working directory
	oldCwd, err := os.Getwd()
	if err == nil {
		if chErr := os.Chdir(baseDir); chErr == nil {
			newCwd, _ := os.Getwd()
			log.Printf("RenameVersion: changed working directory to: %s", newCwd)
			defer os.Chdir(oldCwd)
		} else {
			log.Printf("RenameVersion: failed to change directory to %s: %v", baseDir, chErr)
		}
	}

	// Rename the directory
	if err := os.Rename(oldPath, newPath); err != nil {
		return "", fmt.Errorf("failed to rename directory: %w", err)
	}

	log.Printf("RenameVersion: successfully renamed to %s", newVersion)
	return newVersion, nil
}

// DuplicateVersion creates a copy of a version directory with a new unique name.
// Returns the new version name.
func DuplicateVersion(baseDir, version string) (string, error) {
	// Find a unique name for the duplicate
	newVersion, err := FindNextUniqueVersionName(baseDir, version)
	if err != nil {
		return "", err
	}

	// Copy the directory
	srcPath := filepath.Join(baseDir, version)
	dstPath := filepath.Join(baseDir, newVersion)

	if err := CopyDir(srcPath, dstPath); err != nil {
		return "", fmt.Errorf("failed to copy directory: %w", err)
	}

	return newVersion, nil
}

// DuplicateVersionWithName creates a copy of a version directory with a specified build name.
// Handles collision resolution automatically. Returns the new version name.
func DuplicateVersionWithName(baseDir, version, newBuild string) (string, error) {
	baseVersion, _ := ParseVersionWithBuild(version)

	// Sanitize the new build identifier (empty is allowed)
	newBuild = SanitizeVersionBuild(newBuild)

	// Use collision resolution to get the final build metadata
	resolvedBuild := ResolveCollisionForBuild(baseDir, baseVersion, newBuild)

	// Create the new version name
	var newVersion string
	if resolvedBuild == "" {
		newVersion = baseVersion
	} else {
		newVersion = fmt.Sprintf("%s+%s", baseVersion, resolvedBuild)
	}

	if resolvedBuild != newBuild {
		log.Printf("DuplicateVersionWithName: resolved collision from %s to %s", newBuild, resolvedBuild)
	}

	// Copy the directory
	srcPath := filepath.Join(baseDir, version)
	dstPath := filepath.Join(baseDir, newVersion)

	log.Printf("DuplicateVersionWithName: copying %s to %s", srcPath, dstPath)
	if err := CopyDir(srcPath, dstPath); err != nil {
		return "", fmt.Errorf("failed to copy directory: %w", err)
	}

	log.Printf("DuplicateVersionWithName: successfully duplicated to %s", newVersion)
	return newVersion, nil
}

// CopyDir recursively copies a directory
func CopyDir(src, dst string) error {
	// Get properties of source dir
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create the destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectory
			if err := CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Copy file
			if err := CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// CopyFile copies a single file and preserves timestamps
func CopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := srcFile.WriteTo(dstFile); err != nil {
		return err
	}

	// Preserve the modification time
	if err := os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime()); err != nil {
		// Log but don't fail if we can't preserve timestamps
		log.Printf("Warning: failed to preserve timestamp for %s: %v", dst, err)
	}

	return nil
}

// GetVersionsWithBuildMetadata returns a list of all versions including those with +N suffixes
func GetVersionsWithBuildMetadata(baseDir string) ([]string, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, err
	}

	// Pattern matches: X.Y.Z, X.Y.Z-prerelease, X.Y.Z+build, X.Y.Z-prerelease+build
	versionPattern := regexp.MustCompile(`^\d+\.\d+\.\d+(?:-(?:rc|alpha|beta)(?:\d+)?)?(?:\+.*)?$`)

	var versions []string
	for _, entry := range entries {
		if entry.IsDir() && versionPattern.MatchString(entry.Name()) {
			versions = append(versions, entry.Name())
		}
	}

	// Sort versions using semver logic
	var semvers []*semver.Version
	versionMap := make(map[string]string) // Maps parsed version to original string

	for _, v := range versions {
		// For sorting purposes, we need to handle the +build metadata
		// semver library ignores build metadata in comparisons, which is what we want
		baseVersion, _ := ParseVersionWithBuild(v)
		sv, err := semver.NewVersion(baseVersion)
		if err == nil {
			semvers = append(semvers, sv)
			versionMap[sv.String()] = v
		}
	}

	// Sort in reverse (newest first)
	var sorted []string
	for i := len(semvers) - 1; i >= 0; i-- {
		originalVersion := versionMap[semvers[i].String()]
		sorted = append(sorted, originalVersion)
	}

	return sorted, nil
}

// GetCurrentBuildString extracts the current build metadata from a version string.
// Returns empty string if no build metadata exists.
func GetCurrentBuildString(version string) string {
	_, buildMetadata := ParseVersionWithBuild(version)
	return buildMetadata
}

// IsPrerelease returns true if the version string contains a prerelease tag.
// Uses semver parsing to properly detect prereleases.
func IsPrerelease(version string) bool {
	v, err := semver.NewVersion(version)
	if err != nil {
		// Fall back to string check if semver parsing fails
		return strings.Contains(version, "-")
	}
	return v.Prerelease() != ""
}

// normalizeForComparison converts SNAPSHOT to lowercase so it sorts after rc/alpha/beta.
// Semver sorts prereleases alphabetically, so "snapshot" > "rc" > "beta" > "alpha".
// This is only used for comparison; actual version strings remain unchanged.
func normalizeForComparison(version string) string {
	// Replace SNAPSHOT with snapshot (lowercase) so it sorts after rc
	return strings.Replace(version, "SNAPSHOT", "snapshot", 1)
}

// NewVersionForComparison creates a semver.Version with SNAPSHOT normalized for proper ordering.
func NewVersionForComparison(version string) (*semver.Version, error) {
	return semver.NewVersion(normalizeForComparison(version))
}

// CompareVersions compares two version strings and returns true if v1 > v2.
// SNAPSHOT prereleases are considered more recent than other prereleases (rc, alpha, beta)
// because SNAPSHOT is normalized to lowercase for comparison (snapshot > rc alphabetically).
func CompareVersions(v1Str, v2Str string) bool {
	v1, err1 := NewVersionForComparison(v1Str)
	v2, err2 := NewVersionForComparison(v2Str)
	if err1 != nil || err2 != nil {
		return v1Str > v2Str
	}
	return v1.GreaterThan(v2)
}
