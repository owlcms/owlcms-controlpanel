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

func TestInitWithoutInstanceUsesMainInstance(t *testing.T) {
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

	if err := applyCLIInstanceOptions(cliOptions{init: true}); err != nil {
		t.Fatalf("initialize default instance: %v", err)
	}

	base := filepath.Join(home, ".local", "share")
	checks := []string{
		filepath.Join(base, "owlcms-controlpanel"),
		filepath.Join(base, "owlcms-controlpanel", controlPanelEnvFileName),
		filepath.Join(base, "owlcms"),
		filepath.Join(base, "owlcms", "env.properties"),
		filepath.Join(base, "owlcms-tracker"),
		filepath.Join(base, "owlcms-tracker", "env.properties"),
	}

	for _, path := range checks {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}

	if got := os.Getenv("CONTROLPANEL_INSTANCE"); got != mainInstanceName {
		t.Fatalf("expected CONTROLPANEL_INSTANCE=%s, got %q", mainInstanceName, got)
	}
	if got := os.Getenv("RUNTIME_DIR"); got != filepath.Join(base, "owlcms-controlpanel") {
		t.Fatalf("expected RUNTIME_DIR to default to main control panel dir, got %q", got)
	}
	if got := shared.GetControlPanelInstallDir(); got != filepath.Join(base, "owlcms-controlpanel") {
		t.Fatalf("expected control panel dir %q, got %q", filepath.Join(base, "owlcms-controlpanel"), got)
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

func TestMainInstanceDoesNotRequireStoredRuntimeDir(t *testing.T) {
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

	base := filepath.Join(home, ".local", "share")
	for _, dir := range []string{
		filepath.Join(base, "owlcms-controlpanel"),
		filepath.Join(base, "owlcms"),
		filepath.Join(base, "owlcms-tracker"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("create %s: %v", dir, err)
		}
	}

	if err := applyCLIInstanceOptions(cliOptions{instanceArg: mainInstanceName}); err != nil {
		t.Fatalf("use main instance without init metadata: %v", err)
	}

	if got := os.Getenv("RUNTIME_DIR"); got != filepath.Join(base, "owlcms-controlpanel") {
		t.Fatalf("expected default main runtime dir, got %q", got)
	}
	if got := os.Getenv("CONTROLPANEL_INSTANCE"); got != mainInstanceName {
		t.Fatalf("expected CONTROLPANEL_INSTANCE=%s, got %q", mainInstanceName, got)
	}
	if got := shared.GetControlPanelInstallDir(); got != filepath.Join(base, "owlcms-controlpanel") {
		t.Fatalf("expected control panel dir %q, got %q", filepath.Join(base, "owlcms-controlpanel"), got)
	}
	if got := owlcms.GetInstallDir(); got != filepath.Join(base, "owlcms") {
		t.Fatalf("expected owlcms dir %q, got %q", filepath.Join(base, "owlcms"), got)
	}
	if got := tracker.GetInstallDir(); got != filepath.Join(base, "owlcms-tracker") {
		t.Fatalf("expected tracker dir %q, got %q", filepath.Join(base, "owlcms-tracker"), got)
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

func TestParseCLIOptionsUsesBareArgumentAsInstanceName(t *testing.T) {
	opts := parseCLIOptions([]string{"records", "--owlcms", "latest"})

	if opts.instanceArg != "records" {
		t.Fatalf("expected instanceArg=records, got %q", opts.instanceArg)
	}
}

func TestParseCLIOptionsDoesNotTreatDaemonValueAsInstanceName(t *testing.T) {
	opts := parseCLIOptions([]string{"--owlcms", "latest"})

	if opts.instanceArg != "" {
		t.Fatalf("expected empty instanceArg, got %q", opts.instanceArg)
	}
}

func TestParseCLIOptionsAllowsPositionalInstanceAfterOtherSwitches(t *testing.T) {
	opts := parseCLIOptions([]string{"--runtime-dir", "/tmp/runtime", "records", "--tracker", "latest"})

	if opts.runtimeArg != "/tmp/runtime" {
		t.Fatalf("expected runtimeArg=/tmp/runtime, got %q", opts.runtimeArg)
	}
	if opts.instanceArg != "records" {
		t.Fatalf("expected instanceArg=records, got %q", opts.instanceArg)
	}
}

func TestParseCLIOptionsDoesNotTreatPositionalInstanceAsOwlcmsValueWhenValueOmitted(t *testing.T) {
	opts := parseCLIOptions([]string{"records", "--owlcms"})

	if opts.instanceArg != "records" {
		t.Fatalf("expected instanceArg=records, got %q", opts.instanceArg)
	}
}

func TestParseDaemonFlagsDefaultsMissingValuesToLatest(t *testing.T) {
	owlcmsValue, trackerValue := parseDaemonFlags([]string{"--owlcms", "--tracker"})

	if owlcmsValue != "latest" {
		t.Fatalf("expected owlcms value latest, got %q", owlcmsValue)
	}
	if trackerValue != "latest" {
		t.Fatalf("expected tracker value latest, got %q", trackerValue)
	}
}

func TestParseDaemonFlagsKeepsExplicitValues(t *testing.T) {
	owlcmsValue, trackerValue := parseDaemonFlags([]string{"--owlcms", "3.3.0", "--tracker", "stop"})

	if owlcmsValue != "3.3.0" {
		t.Fatalf("expected owlcms value 3.3.0, got %q", owlcmsValue)
	}
	if trackerValue != "stop" {
		t.Fatalf("expected tracker value stop, got %q", trackerValue)
	}
}

func TestParseDaemonFlagsDoesNotConsumePositionalInstance(t *testing.T) {
	owlcmsValue, trackerValue := parseDaemonFlags([]string{"records", "--owlcms", "--tracker"})

	if owlcmsValue != "latest" {
		t.Fatalf("expected owlcms value latest, got %q", owlcmsValue)
	}
	if trackerValue != "latest" {
		t.Fatalf("expected tracker value latest, got %q", trackerValue)
	}
}
