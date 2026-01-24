package shared

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// ModulePresence represents whether each module is installed and the
// filesystem paths that were checked.
type ModulePresence struct {
	OWLCMS      bool
	Tracker     bool
	Firmata     bool
	OWLCMSPath  string
	TrackerPath string
	FirmataPath string
}

// DetectInstalledModules checks for presence of module installation
// directories using the exact paths provided by each package. It logs
// each path checked and whether it was found.
func DetectInstalledModules(owlcmsPath, trackerPath, firmataPath string) ModulePresence {
	owlexists := dirExists(owlcmsPath)
	trackerexists := dirExists(trackerPath)
	firmataexists := dirExists(firmataPath)

	// TEMPORARY TEST OVERRIDE: force all modules to appear NOT INSTALLED.
	// This is intentional for temporary UI testing and should be removed.
	if true {
		log.Printf("DetectInstalledModules: forcing all modules to false for testing")
		owlexists = false
		trackerexists = false
		firmataexists = false
	}

	log.Printf("DetectInstalledModules: checked OWLCMS path=%s exists=%v", owlcmsPath, owlexists)
	log.Printf("DetectInstalledModules: checked Tracker path=%s exists=%v", trackerPath, trackerexists)
	log.Printf("DetectInstalledModules: checked Firmata path=%s exists=%v", firmataPath, firmataexists)

	return ModulePresence{
		OWLCMS:      owlexists,
		Tracker:     trackerexists,
		Firmata:     firmataexists,
		OWLCMSPath:  owlcmsPath,
		TrackerPath: trackerPath,
		FirmataPath: firmataPath,
	}
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// ProcessLocalZipFile handles a ZIP file selected from the file system.
// installFunc is called with (zipPath, version) to perform the actual installation.
// versionPlaceholder is the example version shown in the manual entry dialog (e.g., "1.2.3" or "4.24.1").
func ProcessLocalZipFile(zipPath string, w fyne.Window, versionPlaceholder string,
	installFunc func(zipPath, version string)) {
	// Extract version number from filename if possible
	fileName := filepath.Base(zipPath)
	version, err := ExtractVersionFromFilename(fileName)

	// If version couldn't be determined, ask the user
	if err != nil {
		content := widget.NewEntry()
		content.SetPlaceHolder("e.g., " + versionPlaceholder)

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
					installFunc(zipPath, content.Text)
				} else {
					dialog.ShowError(fmt.Errorf("invalid version format, please use semantic versioning (e.g., %s)", versionPlaceholder), w)
				}
			},
			w,
		)
		versionDialog.Show()
		return
	}

	// We have a valid version, proceed with installation
	installFunc(zipPath, version)
}
