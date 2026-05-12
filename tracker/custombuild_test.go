package tracker

import (
	"os"
	"strings"
	"testing"
)

func TestRemoveCustomBuildMarkerIgnoresMissingMarker(t *testing.T) {
	versionDir := t.TempDir()
	if err := removeCustomBuildMarker(versionDir); err != nil {
		t.Fatalf("remove missing marker: %v", err)
	}
}

func TestHasAndRemoveCustomBuildMarker(t *testing.T) {
	versionDir := t.TempDir()
	markerPath := customBuildMarkerPath(versionDir)

	if hasCustomBuildMarker(versionDir) {
		t.Fatalf("expected no marker in %s", versionDir)
	}
	if err := os.WriteFile(markerPath, []byte("custom build\n"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	if !hasCustomBuildMarker(versionDir) {
		t.Fatalf("expected marker to be detected")
	}
	if err := removeCustomBuildMarker(versionDir); err != nil {
		t.Fatalf("remove marker: %v", err)
	}
	if hasCustomBuildMarker(versionDir) {
		t.Fatalf("expected marker to be removed")
	}
}

func TestReadCustomBuildPlugins(t *testing.T) {
	versionDir := t.TempDir()

	if _, ok := readCustomBuildPlugins(versionDir); ok {
		t.Fatal("expected no marker")
	}

	markerPath := customBuildMarkerPath(versionDir)

	content := "This tracker package was built with custom selection options.\nplugins: Alpha, Beta, Gamma\n"
	if err := os.WriteFile(markerPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	plugins, ok := readCustomBuildPlugins(versionDir)
	if !ok {
		t.Fatal("expected marker to be found")
	}
	if len(plugins) != 3 || plugins[0] != "Alpha" || plugins[1] != "Beta" || plugins[2] != "Gamma" {
		t.Fatalf("unexpected plugins: %v", plugins)
	}

	if err := os.WriteFile(markerPath, []byte("custom build\nplugins: (none)\n"), 0o644); err != nil {
		t.Fatalf("write none marker: %v", err)
	}
	plugins, ok = readCustomBuildPlugins(versionDir)
	if !ok {
		t.Fatal("expected marker")
	}
	if len(plugins) != 0 {
		t.Fatalf("expected empty list, got %v", plugins)
	}

	if err := os.WriteFile(markerPath, []byte("custom build\n"), 0o644); err != nil {
		t.Fatalf("write old marker: %v", err)
	}
	plugins, ok = readCustomBuildPlugins(versionDir)
	if !ok {
		t.Fatal("expected marker")
	}
	if len(plugins) != 0 {
		t.Fatalf("expected empty list for old format, got %v", plugins)
	}
}

func TestCustomBuildPluginsEqual(t *testing.T) {
	if !customBuildPluginsEqual([]string{"A", "B"}, []string{"A", "B"}) {
		t.Fatal("expected equal")
	}
	if customBuildPluginsEqual([]string{"A"}, []string{"A", "B"}) {
		t.Fatal("expected not equal (length)")
	}
	if customBuildPluginsEqual([]string{"A", "B"}, []string{"A", "C"}) {
		t.Fatal("expected not equal (content)")
	}
}

func TestUpdateCustomBuildBlockMessage(t *testing.T) {
	message := updateCustomBuildBlockMessage("1.0.0+custom", "1.1.0")
	for _, want := range []string{"Cannot update:", "1.0.0+custom", "1.1.0", "The standard version does not include your custom plugins."} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected %q in update block message, got:\n%s", want, message)
		}
	}
}

func TestImportMatchedCustomBuildWarning(t *testing.T) {
	message := importMatchedCustomBuildWarning("1.0.0+custom", "1.1.0+custom", []string{"Plugin A"})
	for _, want := range []string{"1.0.0+custom", "1.1.0+custom", "Plugin A", "compatible", "provider"} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected %q in matched warning, got:\n%s", want, message)
		}
	}
}

func TestImportMismatchedCustomBuildWarning(t *testing.T) {
	message := importMismatchedCustomBuildWarning("1.0.0+src", "1.1.0+dst", []string{"Plugin A"}, []string{"Plugin B"})
	for _, want := range []string{"WARNING:", "1.0.0+src", "1.1.0+dst", "Plugin A", "Plugin B", "provider"} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected %q in mismatched warning, got:\n%s", want, message)
		}
	}
}

func TestImportBlockMessageSourceOnly(t *testing.T) {
	message := importBlockMessage("1.0.0+custom", "1.1.0", []string{"Plugin A"}, true, nil, false)
	for _, want := range []string{"1.0.0+custom", "1.1.0", "Plugin A", "The standard build does not contain the code for your custom plugins.", "Import is blocked."} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected %q in source-only block message, got:\n%s", want, message)
		}
	}
}

func TestImportBlockMessageDestOnly(t *testing.T) {
	message := importBlockMessage("1.0.0", "1.1.0+custom", nil, false, []string{"Plugin B"}, true)
	for _, want := range []string{"1.0.0", "1.1.0+custom", "The standard version does not include your custom plugins."} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected %q in dest-only block message, got:\n%s", want, message)
		}
	}
}
