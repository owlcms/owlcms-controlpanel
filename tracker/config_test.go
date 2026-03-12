package tracker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetPortForReleaseUsesReleaseOverride(t *testing.T) {
	installDir := t.TempDir()
	previousDir := GetInstallDir()
	SetInstallDir(installDir)
	t.Cleanup(func() {
		SetInstallDir(previousDir)
	})

	if err := os.WriteFile(filepath.Join(installDir, "env.properties"), []byte("TRACKER_PORT=8096\n"), 0o644); err != nil {
		t.Fatalf("write shared env: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(installDir, "2.3.0"), 0o755); err != nil {
		t.Fatalf("mkdir release dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "2.3.0", "env.properties"), []byte("TRACKER_PORT=18123\n"), 0o644); err != nil {
		t.Fatalf("write release env: %v", err)
	}

	if got := GetPortForRelease("2.3.0"); got != "18123" {
		t.Fatalf("expected release port 18123, got %q", got)
	}
	if got := GetPortForRelease("missing"); got != "8096" {
		t.Fatalf("expected fallback shared port 8096, got %q", got)
	}
}
