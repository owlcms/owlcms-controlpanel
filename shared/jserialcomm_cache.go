package shared

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
)

// PurgeJSerialCommCaches removes any cached jSerialComm native libraries
// from the two locations the library uses on Windows. A wrong-architecture
// dll cached from an earlier install (e.g. a previously installed ARM64
// build) prevents the library from loading even when the current JVM is
// AMD64. Removing the caches forces jSerialComm to re-extract a native
// library matching the current JVM on next startup.
//
// Best-effort: logs but does not return errors. No-op on non-Windows.
func PurgeJSerialCommCaches() {
	if runtime.GOOS != "windows" {
		return
	}

	candidates := []string{}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, ".jSerialComm"))
	}
	if tmp := os.Getenv("LOCALAPPDATA"); tmp != "" {
		candidates = append(candidates, filepath.Join(tmp, "Temp", "jSerialComm"))
	}
	if tmp := os.TempDir(); tmp != "" {
		candidates = append(candidates, filepath.Join(tmp, "jSerialComm"))
	}

	for _, dir := range candidates {
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("PurgeJSerialCommCaches: could not remove %s: %v", dir, err)
			continue
		}
		log.Printf("PurgeJSerialCommCaches: removed %s", dir)
	}
}
