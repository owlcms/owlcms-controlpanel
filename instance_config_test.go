package main

import (
	"os"
	"path/filepath"
	"testing"

	"controlpanel/owlcms"
	"controlpanel/shared"
	"controlpanel/tracker"
)

func resetInstallDirsForTest() {
	owlcms.SetInstallDir(shared.GetOwlcmsInstallDir())
	tracker.SetInstallDir(shared.GetTrackerInstallDir())
}

func TestInitCreatesFullInstanceLayout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GOOS", "linux")
	t.Setenv("APPDATA", "")
	t.Setenv("CONTROLPANEL_INSTALLDIR", "")
	t.Setenv("OWLCMS_INSTALLDIR", "")
	t.Setenv("TRACKER_INSTALLDIR", "")
	t.Setenv("RUNTIME_DIR", "")
	t.Setenv("CONTROLPANEL_INSTANCE", "")
	resetInstallDirsForTest()

	if err := applyCLIInstanceOptions(cliOptions{instanceArg: "records", init: true}); err != nil {
		t.Fatalf("initialize instance: %v", err)
	}

	checks := []string{
		filepath.Join(home, ".local", "share", "records-controlpanel"),
		filepath.Join(home, ".local", "share", "records-controlpanel", controlPanelEnvFileName),
		filepath.Join(home, ".local", "share", "records-owlcms"),
		filepath.Join(home, ".local", "share", "records-owlcms", "env.properties"),
		filepath.Join(home, ".local", "share", "records-tracker"),
		filepath.Join(home, ".local", "share", "records-tracker", "env.properties"),
	}

	for _, path := range checks {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
}

func TestImplicitHeadlessInstanceUsesInitializedInstance(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GOOS", "linux")
	t.Setenv("APPDATA", "")
	t.Setenv("CONTROLPANEL_INSTALLDIR", "")
	t.Setenv("OWLCMS_INSTALLDIR", "")
	t.Setenv("TRACKER_INSTALLDIR", "")
	t.Setenv("RUNTIME_DIR", "")
	t.Setenv("CONTROLPANEL_INSTANCE", "")
	resetInstallDirsForTest()

	if err := applyCLIInstanceOptions(cliOptions{instanceArg: "records", init: true}); err != nil {
		t.Fatalf("initialize instance: %v", err)
	}

	t.Setenv("CONTROLPANEL_INSTALLDIR", "")
	t.Setenv("OWLCMS_INSTALLDIR", "")
	t.Setenv("TRACKER_INSTALLDIR", "")
	t.Setenv("RUNTIME_DIR", "")
	t.Setenv("CONTROLPANEL_INSTANCE", "")
	resetInstallDirsForTest()

	owlcmsValue, trackerValue, err := maybeApplyImplicitInstanceForHeadless(cliOptions{}, "records", "")
	if err != nil {
		t.Fatalf("infer implicit instance: %v", err)
	}
	if owlcmsValue != "latest" {
		t.Fatalf("expected owlcms value latest, got %q", owlcmsValue)
	}
	if trackerValue != "" {
		t.Fatalf("expected empty tracker value, got %q", trackerValue)
	}
	if got := os.Getenv("CONTROLPANEL_INSTANCE"); got != "records" {
		t.Fatalf("expected CONTROLPANEL_INSTANCE=records, got %q", got)
	}

	wantOwlcmsDir := filepath.Join(home, ".local", "share", "owlcms-records")
	wantOwlcmsDir = filepath.Join(home, ".local", "share", "records-owlcms")
	if got := owlcms.GetInstallDir(); got != wantOwlcmsDir {
		t.Fatalf("expected owlcms install dir %q, got %q", wantOwlcmsDir, got)
	}
}

func TestResolveInstancePathsForSimpleNameUsesPrefixNaming(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GOOS", "linux")

	paths, err := resolveInstancePaths("records")
	if err != nil {
		t.Fatalf("resolve instance paths: %v", err)
	}

	base := filepath.Join(home, ".local", "share")
	if paths.ControlPanelDir != filepath.Join(base, "records-controlpanel") {
		t.Fatalf("unexpected control panel dir %q", paths.ControlPanelDir)
	}
	if paths.OwlcmsDir != filepath.Join(base, "records-owlcms") {
		t.Fatalf("unexpected owlcms dir %q", paths.OwlcmsDir)
	}
	if paths.TrackerDir != filepath.Join(base, "records-tracker") {
		t.Fatalf("unexpected tracker dir %q", paths.TrackerDir)
	}
}

func TestResolveInstancePathsForAbsolutePathUsesBaseNameAsInstance(t *testing.T) {
	paths, err := resolveInstancePaths(filepath.Join(string(os.PathSeparator), "data", "records"))
	if err != nil {
		t.Fatalf("resolve instance paths: %v", err)
	}

	if paths.InstanceName != "records" {
		t.Fatalf("unexpected instance name %q", paths.InstanceName)
	}
	if paths.ControlPanelDir != filepath.Join(string(os.PathSeparator), "data", "records") {
		t.Fatalf("unexpected control panel dir %q", paths.ControlPanelDir)
	}
	if paths.OwlcmsDir != filepath.Join(string(os.PathSeparator), "data", "records-owlcms") {
		t.Fatalf("unexpected owlcms dir %q", paths.OwlcmsDir)
	}
	if paths.TrackerDir != filepath.Join(string(os.PathSeparator), "data", "records-tracker") {
		t.Fatalf("unexpected tracker dir %q", paths.TrackerDir)
	}
}

func TestResolveInstancePathsForAbsoluteControlPanelPathStripsSuffix(t *testing.T) {
	paths, err := resolveInstancePaths(filepath.Join(string(os.PathSeparator), "data", "records-controlpanel"))
	if err != nil {
		t.Fatalf("resolve instance paths: %v", err)
	}

	if paths.InstanceName != "records" {
		t.Fatalf("unexpected instance name %q", paths.InstanceName)
	}
	if paths.OwlcmsDir != filepath.Join(string(os.PathSeparator), "data", "records-owlcms") {
		t.Fatalf("unexpected owlcms dir %q", paths.OwlcmsDir)
	}
	if paths.TrackerDir != filepath.Join(string(os.PathSeparator), "data", "records-tracker") {
		t.Fatalf("unexpected tracker dir %q", paths.TrackerDir)
	}
}

func TestResolveInstancePathsForMainInstanceKeepsLegacyNames(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GOOS", "linux")

	paths, err := resolveInstancePaths("owlcms")
	if err != nil {
		t.Fatalf("resolve instance paths: %v", err)
	}

	base := filepath.Join(home, ".local", "share")
	if paths.ControlPanelDir != filepath.Join(base, "owlcms-controlpanel") {
		t.Fatalf("unexpected control panel dir %q", paths.ControlPanelDir)
	}
	if paths.OwlcmsDir != filepath.Join(base, "owlcms") {
		t.Fatalf("unexpected owlcms dir %q", paths.OwlcmsDir)
	}
	if paths.TrackerDir != filepath.Join(base, "owlcms-tracker") {
		t.Fatalf("unexpected tracker dir %q", paths.TrackerDir)
	}
}

func TestResolveInstancePathsForMainInstanceAcceptsOwlcmsOwlcms(t *testing.T) {
	root := t.TempDir()
	aliasDir := filepath.Join(root, "owlcms-owlcms")
	if err := os.MkdirAll(aliasDir, 0755); err != nil {
		t.Fatalf("create alias dir: %v", err)
	}

	paths, err := resolveInstancePaths(filepath.Join(root, "owlcms-controlpanel"))
	if err != nil {
		t.Fatalf("resolve instance paths: %v", err)
	}

	if paths.InstanceName != "owlcms" {
		t.Fatalf("unexpected instance name %q", paths.InstanceName)
	}
	if paths.OwlcmsDir != aliasDir {
		t.Fatalf("expected owlcms dir %q, got %q", aliasDir, paths.OwlcmsDir)
	}
}

func TestImplicitHeadlessInstanceDoesNotOverrideInstalledVersion(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GOOS", "linux")
	t.Setenv("APPDATA", "")
	t.Setenv("CONTROLPANEL_INSTALLDIR", "")
	t.Setenv("OWLCMS_INSTALLDIR", "")
	t.Setenv("TRACKER_INSTALLDIR", "")
	t.Setenv("RUNTIME_DIR", "")
	t.Setenv("CONTROLPANEL_INSTANCE", "")
	resetInstallDirsForTest()

	if err := applyCLIInstanceOptions(cliOptions{instanceArg: "records", init: true}); err != nil {
		t.Fatalf("initialize instance: %v", err)
	}

	t.Setenv("CONTROLPANEL_INSTALLDIR", "")
	t.Setenv("OWLCMS_INSTALLDIR", "")
	t.Setenv("TRACKER_INSTALLDIR", "")
	t.Setenv("RUNTIME_DIR", "")
	t.Setenv("CONTROLPANEL_INSTANCE", "")
	resetInstallDirsForTest()

	defaultVersionDir := filepath.Join(shared.GetOwlcmsInstallDir(), "records")
	if err := os.MkdirAll(defaultVersionDir, 0755); err != nil {
		t.Fatalf("create default version dir: %v", err)
	}

	owlcmsValue, trackerValue, err := maybeApplyImplicitInstanceForHeadless(cliOptions{}, "records", "")
	if err != nil {
		t.Fatalf("infer implicit instance: %v", err)
	}
	if owlcmsValue != "records" {
		t.Fatalf("expected owlcms value to stay records, got %q", owlcmsValue)
	}
	if trackerValue != "" {
		t.Fatalf("expected empty tracker value, got %q", trackerValue)
	}
	if got := os.Getenv("CONTROLPANEL_INSTANCE"); got != "" {
		t.Fatalf("expected CONTROLPANEL_INSTANCE to remain empty, got %q", got)
	}
}
