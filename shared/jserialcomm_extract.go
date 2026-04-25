package shared

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// jSerialCommNativeRelPath returns the path *inside* the jar where the native
// library for the current OS+arch is stored, plus the bare filename it must be
// extracted as. Layout matches what jSerialComm's manual loader expects:
//
//	<libraryPath>/<OS>/<arch>/<filename>
//
// where <libraryPath> is what we will pass via -DjSerialComm.library.path.
func jSerialCommNativeRelPath() (osDir, archDir, fileName string, ok bool) {
	switch runtime.GOOS {
	case "windows":
		osDir = "Windows"
		fileName = "jSerialComm.dll"
		switch runtime.GOARCH {
		case "amd64":
			archDir = "x86_64"
		case "arm64":
			archDir = "aarch64"
		case "386":
			archDir = "x86"
		default:
			return "", "", "", false
		}
	case "linux":
		osDir = "Linux"
		fileName = "libjSerialComm.so"
		switch runtime.GOARCH {
		case "amd64":
			archDir = "x86_64"
		case "arm64":
			archDir = "armv8_64"
		case "386":
			archDir = "x86"
		default:
			return "", "", "", false
		}
	case "darwin":
		osDir = "OSX"
		fileName = "libjSerialComm.jnilib"
		switch runtime.GOARCH {
		case "amd64":
			archDir = "x86_64"
		case "arm64":
			archDir = "aarch64"
		default:
			return "", "", "", false
		}
	default:
		return "", "", "", false
	}
	return osDir, archDir, fileName, true
}

// ExtractJSerialCommNative extracts the native jSerialComm library that
// matches the current OS/arch out of jarPath into destDir, laid out as
//
//	<destDir>/<OS>/<arch>/<filename>
//
// so that passing -DjSerialComm.library.path=<destDir>/ to the JVM bypasses
// jSerialComm's autodetection (which is unreliable when a Go launcher starts
// the JVM under WoW emulation).
//
// Returns the absolute library path string suitable for the
// -DjSerialComm.library.path system property (always with a trailing
// separator, since jSerialComm appends "<OS>/<arch>/<file>" to it).
//
// The extraction is idempotent: if the destination file already exists with
// the same byte size as the entry inside the jar, it is left untouched.
func ExtractJSerialCommNative(jarPath, destDir string) (string, error) {
	osDir, archDir, fileName, ok := jSerialCommNativeRelPath()
	if !ok {
		return "", fmt.Errorf("jSerialComm: unsupported OS/arch %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	jarEntry := osDir + "/" + archDir + "/" + fileName

	zr, err := zip.OpenReader(jarPath)
	if err != nil {
		return "", fmt.Errorf("opening jar %s: %w", jarPath, err)
	}
	defer zr.Close()

	var src *zip.File
	for _, f := range zr.File {
		if f.Name == jarEntry {
			src = f
			break
		}
	}
	if src == nil {
		return "", fmt.Errorf("jSerialComm native %q not found inside %s", jarEntry, jarPath)
	}

	outDir := filepath.Join(destDir, osDir, archDir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("creating native dir %s: %w", outDir, err)
	}
	outPath := filepath.Join(outDir, fileName)

	// Skip extraction when the file is already present and matches the jar entry size.
	if info, err := os.Stat(outPath); err == nil && uint64(info.Size()) == src.UncompressedSize64 {
		log.Printf("jSerialComm native already present at %s (%d bytes)", outPath, info.Size())
		return ensureTrailingSep(destDir), nil
	}

	in, err := src.Open()
	if err != nil {
		return "", fmt.Errorf("opening jar entry %s: %w", jarEntry, err)
	}
	defer in.Close()

	out, err := os.OpenFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return "", fmt.Errorf("creating %s: %w", outPath, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return "", fmt.Errorf("writing %s: %w", outPath, err)
	}
	if err := out.Close(); err != nil {
		return "", fmt.Errorf("closing %s: %w", outPath, err)
	}

	log.Printf("Extracted jSerialComm native %s to %s", jarEntry, outPath)
	return ensureTrailingSep(destDir), nil
}

func ensureTrailingSep(p string) string {
	if strings.HasSuffix(p, string(os.PathSeparator)) || strings.HasSuffix(p, "/") {
		return p
	}
	return p + string(os.PathSeparator)
}
