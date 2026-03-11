package shared

import (
	"path/filepath"
	"testing"
)

func TestDetectInstalledModulesUsesActualDirectoryPresence(t *testing.T) {
	tempDir := t.TempDir()

	owlcmsDir := filepath.Join(tempDir, "owlcms")
	trackerDir := filepath.Join(tempDir, "tracker")
	videoDir := filepath.Join(tempDir, "video")
	missingFirmataDir := filepath.Join(tempDir, "firmata")

	for _, dir := range []string{owlcmsDir, trackerDir, videoDir} {
		if err := EnsureDir0755(dir); err != nil {
			t.Fatalf("create %s: %v", dir, err)
		}
	}

	mods := DetectInstalledModules(owlcmsDir, trackerDir, missingFirmataDir, videoDir)

	if !mods.OWLCMS {
		t.Fatalf("expected OWLCMS=true for existing dir %s", owlcmsDir)
	}
	if !mods.Tracker {
		t.Fatalf("expected Tracker=true for existing dir %s", trackerDir)
	}
	if mods.Firmata {
		t.Fatalf("expected Firmata=false for missing dir %s", missingFirmataDir)
	}
	if !mods.Video {
		t.Fatalf("expected Video=true for existing dir %s", videoDir)
	}
	if mods.OWLCMSPath != owlcmsDir || mods.TrackerPath != trackerDir || mods.FirmataPath != missingFirmataDir || mods.VideoPath != videoDir {
		t.Fatalf("unexpected paths in ModulePresence: %+v", mods)
	}
}
