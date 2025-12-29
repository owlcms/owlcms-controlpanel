package shared

import (
	"log"
	"os"
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
