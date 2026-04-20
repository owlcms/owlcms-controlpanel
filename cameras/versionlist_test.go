package cameras

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyVersionConfigArtifactsCopiesRootConfigAndLegacyDir(t *testing.T) {
	srcDir := filepath.Join(t.TempDir(), "src")
	destDir := filepath.Join(t.TempDir(), "dest")

	if err := os.MkdirAll(filepath.Join(srcDir, "config"), 0755); err != nil {
		t.Fatalf("mkdir src config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "config.toml"), []byte("[[rtsp]]\nsourceId = \"rtsp-1\"\n"), 0644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "config", "legacy.toml"), []byte("legacy=true\n"), 0644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	if err := copyVersionConfigArtifacts(srcDir, destDir); err != nil {
		t.Fatalf("copyVersionConfigArtifacts: %v", err)
	}

	configToml, err := os.ReadFile(filepath.Join(destDir, "config.toml"))
	if err != nil {
		t.Fatalf("read copied config.toml: %v", err)
	}
	if string(configToml) != "[[rtsp]]\nsourceId = \"rtsp-1\"\n" {
		t.Fatalf("unexpected copied config.toml contents: %q", string(configToml))
	}

	legacyConfig, err := os.ReadFile(filepath.Join(destDir, "config", "legacy.toml"))
	if err != nil {
		t.Fatalf("read copied legacy config: %v", err)
	}
	if string(legacyConfig) != "legacy=true\n" {
		t.Fatalf("unexpected copied legacy config contents: %q", string(legacyConfig))
	}
}

func TestCopyVersionConfigArtifactsRequiresAtLeastOneConfigArtifact(t *testing.T) {
	srcDir := filepath.Join(t.TempDir(), "src")
	destDir := filepath.Join(t.TempDir(), "dest")

	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}

	if err := copyVersionConfigArtifacts(srcDir, destDir); err == nil {
		t.Fatal("expected error when no config artifacts exist")
	}
}
