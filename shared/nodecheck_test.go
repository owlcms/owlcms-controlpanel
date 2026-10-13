package shared

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFakeNode(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0755); err != nil {
		t.Fatal(err)
	}
}

func TestFindLocalNodeForVersionWindowsLayouts(t *testing.T) {
	windows := func() string { return "windows" }

	t.Run("archive root preserved", func(t *testing.T) {
		runtime := t.TempDir()
		t.Setenv("RUNTIME_DIR", runtime)
		want := filepath.Join(runtime, "node", "v24.21.0", "node-v24.21.0-win-x64", "node.exe")
		writeFakeNode(t, want)

		got, err := FindLocalNodeForVersion("v24.21.0", windows)
		if err != nil || got != want {
			t.Fatalf("got %q, %v; want %q", got, err, want)
		}
	})

	t.Run("archive root stripped", func(t *testing.T) {
		runtime := t.TempDir()
		t.Setenv("RUNTIME_DIR", runtime)
		want := filepath.Join(runtime, "node", "v24.21.0", "node.exe")
		writeFakeNode(t, want)
		writeFakeNode(t, filepath.Join(runtime, "node", "v24.21.0", "node_modules", "npm", "node.exe"))

		got, err := FindLocalNodeForVersion("v24.21.0", windows)
		if err != nil || got != want {
			t.Fatalf("got %q, %v; want %q", got, err, want)
		}
	})
}

func TestFindLocalNodeForVersionUnixLayout(t *testing.T) {
	runtime := t.TempDir()
	t.Setenv("RUNTIME_DIR", runtime)
	want := filepath.Join(runtime, "node", "v24.21.0", "node-v24.21.0-linux-x64", "bin", "node")
	writeFakeNode(t, want)

	got, err := FindLocalNodeForVersion("v24.0.0", func() string { return "linux" })
	if err != nil || got != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
}
