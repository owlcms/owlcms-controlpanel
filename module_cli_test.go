package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustMkdir(t *testing.T, baseDir, name string) {
	t.Helper()
	if err := os.Mkdir(filepath.Join(baseDir, name), 0755); err != nil {
		t.Fatalf("creating %s: %v", name, err)
	}
}

func TestParseModuleCommandInstallLatest(t *testing.T) {
	cmd, handled, err := parseModuleCommand([]string{"--module", "owlcms", "--install", "latest"})
	if err != nil {
		t.Fatalf("parseModuleCommand returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected command to be handled")
	}
	if cmd.Module != "owlcms" || cmd.Action != "install" || cmd.InstallVersion != "latest" {
		t.Fatalf("unexpected command: %#v", cmd)
	}
}

func TestParseModuleCommandInstallDefaultsToLatest(t *testing.T) {
	cmd, handled, err := parseModuleCommand([]string{"--module", "tracker", "--install"})
	if err != nil {
		t.Fatalf("parseModuleCommand returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected command to be handled")
	}
	if cmd.Module != "tracker" || cmd.Action != "install" || cmd.InstallVersion != "latest" {
		t.Fatalf("unexpected command: %#v", cmd)
	}
}

func TestParseModuleCommandSeparatesLocalAndRemoteLatest(t *testing.T) {
	cmd, handled, err := parseModuleCommand([]string{"--module", "owlcms", "--version", "latest", "--update-to", "latest"})
	if err != nil {
		t.Fatalf("parseModuleCommand returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected command to be handled")
	}
	if cmd.Module != "owlcms" || cmd.Action != "update" || cmd.Version != "latest" || cmd.UpdateTo != "latest" {
		t.Fatalf("unexpected command: %#v", cmd)
	}
}

func TestParseModuleCommandLocalTrackerDefaultPort(t *testing.T) {
	cmd, handled, err := parseModuleCommand([]string{"--module", "owlcms", "--launch", "--local-tracker"})
	if err != nil {
		t.Fatalf("parseModuleCommand returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected command to be handled")
	}
	if cmd.Action != "launch" || cmd.LocalTrackerPort != "8096" {
		t.Fatalf("unexpected command: %#v", cmd)
	}
}

func TestParseModuleCommandBackgroundAliasEnablesDaemonMode(t *testing.T) {
	cmd, handled, err := parseModuleCommand([]string{"--module", "owlcms", "--launch", "--background"})
	if err != nil {
		t.Fatalf("parseModuleCommand returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected command to be handled")
	}
	if cmd.Action != "launch" || !cmd.DaemonMode {
		t.Fatalf("unexpected command: %#v", cmd)
	}
}

func TestParseModuleCommandInstallZip(t *testing.T) {
	cmd, handled, err := parseModuleCommand([]string{"--module", "owlcms", "--install-zip", "C:/Downloads/owlcms_66.0.0.zip"})
	if err != nil {
		t.Fatalf("parseModuleCommand returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected command to be handled")
	}
	if cmd.Action != "install-zip" || cmd.InstallZipPath != "C:/Downloads/owlcms_66.0.0.zip" {
		t.Fatalf("unexpected command: %#v", cmd)
	}
}

func TestParseModuleCommandCreateZip(t *testing.T) {
	cmd, handled, err := parseModuleCommand([]string{"--module", "tracker", "--create-zip", "C:/Backups/tracker.zip", "--version", "3.4.0"})
	if err != nil {
		t.Fatalf("parseModuleCommand returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected command to be handled")
	}
	if cmd.Action != "create-zip" || cmd.CreateZipPath != "C:/Backups/tracker.zip" || cmd.Version != "3.4.0" {
		t.Fatalf("unexpected command: %#v", cmd)
	}
}

func TestParseModuleCommandRejectsLegacyOwlcmsFlag(t *testing.T) {
	_, handled, err := parseModuleCommand([]string{"--owlcms", "latest"})
	if !handled {
		t.Fatal("expected legacy flag to be handled as a command-line error")
	}
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("expected unsupported legacy flag error, got %v", err)
	}
}

func TestParseModuleCommandRejectsLocalTrackerForTrackerModule(t *testing.T) {
	_, handled, err := parseModuleCommand([]string{"--module", "tracker", "--launch", "--local-tracker"})
	if !handled {
		t.Fatal("expected command to be handled")
	}
	if err == nil || !strings.Contains(err.Error(), "--module owlcms") {
		t.Fatalf("expected local-tracker module error, got %v", err)
	}
}

func TestExecuteModuleDuplicateRequiresFromVersion(t *testing.T) {
	err := executeModuleCommand(moduleCLICommand{Module: "owlcms", Action: "duplicate", DuplicateName: "copy"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--from-version") {
		t.Fatalf("expected from-version error, got %v", err)
	}
}

func TestResolveLocalVersionSelectorPreviousUsesPenultimateInstalledVersion(t *testing.T) {
	base := t.TempDir()
	mustMkdir(t, base, "65.0.0")
	mustMkdir(t, base, "64.0.0")
	mustMkdir(t, base, "63.0.0")

	version, err := resolveLocalVersionSelector("owlcms", "previous", []string{"65.0.0", "64.0.0", "63.0.0"}, base)
	if err != nil {
		t.Fatalf("resolveLocalVersionSelector returned error: %v", err)
	}
	if version != "64.0.0" {
		t.Fatalf("expected previous=64.0.0, got %q", version)
	}
}

func TestResolveLocalVersionSelectorAllowsSpecificNonSemverDirectory(t *testing.T) {
	base := t.TempDir()
	mustMkdir(t, base, "duplicate-65.0.0")

	version, err := resolveLocalVersionSelector("owlcms", "duplicate-65.0.0", nil, base)
	if err != nil {
		t.Fatalf("resolveLocalVersionSelector returned error: %v", err)
	}
	if version != "duplicate-65.0.0" {
		t.Fatalf("expected exact duplicate directory, got %q", version)
	}
}
