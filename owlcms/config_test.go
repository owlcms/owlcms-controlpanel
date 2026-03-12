package owlcms

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetTrackerConnectionPortForReleaseReadsStoredURL(t *testing.T) {
	installDir := t.TempDir()
	previousDir := GetInstallDir()
	SetInstallDir(installDir)
	t.Cleanup(func() {
		SetInstallDir(previousDir)
	})

	if err := os.WriteFile(filepath.Join(installDir, "env.properties"), []byte("OWLCMS_PORT=8080\nTEMURIN_VERSION=jdk-25\n"), 0o644); err != nil {
		t.Fatalf("write shared env: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(installDir, "65.0.0"), 0o755); err != nil {
		t.Fatalf("mkdir release dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "65.0.0", "env.properties"), []byte("OWLCMS_VIDEODATA=ws://127.0.0.1:18123/ws\n"), 0o644); err != nil {
		t.Fatalf("write release env: %v", err)
	}

	if got := GetTrackerConnectionPortForRelease("65.0.0"); got != "18123" {
		t.Fatalf("expected tracker port 18123, got %q", got)
	}
	if !GetTrackerConnectionEnabledForRelease("65.0.0") {
		t.Fatal("expected tracker connection to be enabled for release")
	}
}

func TestTrackerConnectionPortRejectsNonLocalTrackerURL(t *testing.T) {
	if port, ok := trackerConnectionPort("ws://example.com:18123/ws"); ok || port != "" {
		t.Fatalf("expected non-local URL to be rejected, got port=%q ok=%v", port, ok)
	}
}
