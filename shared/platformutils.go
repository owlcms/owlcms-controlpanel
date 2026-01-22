package shared

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// IsWSL checks if the program is running under Windows Subsystem for Linux.
func IsWSL() bool {
	_, err := os.Stat("/proc/version")
	if err != nil {
		return false
	}
	version, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(string(version), "Microsoft")
}

// GetGoos returns the current operating system identifier.
// Returns "linux" if running under WSL.
func GetGoos() string {
	if IsWSL() {
		return "linux"
	}
	return runtime.GOOS
}

// GetGoarch returns the current architecture identifier.
func GetGoarch() string {
	return runtime.GOARCH
}

// OpenBrowser opens the specified URL in the system default browser
func OpenBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	if err := cmd.Start(); err != nil {
		log.Printf("Failed to open browser: %v\n", err)
		return fmt.Errorf("failed to open browser: %w", err)
	}

	return nil
}

// OpenFileExplorer opens the specified path in the system file explorer
func OpenFileExplorer(path string) error {
	var cmd *exec.Cmd

	// Ensure path is absolute and clean
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	log.Printf("OpenFileExplorer: opening path %s\n", absPath)

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", absPath)
	case "darwin":
		cmd = exec.Command("open", absPath)
	case "linux":
		cmd = exec.Command("xdg-open", absPath)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	if err := cmd.Start(); err != nil {
		log.Printf("Failed to open file explorer: %v\n", err)
		return fmt.Errorf("failed to open file explorer: %w", err)
	}

	return nil
}

// OpenFile opens the specified file with the system default application
func OpenFile(filePath string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", filePath)
	case "darwin":
		cmd = exec.Command("open", filePath)
	case "linux":
		cmd = exec.Command("xdg-open", filePath)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	if err := cmd.Start(); err != nil {
		log.Printf("Failed to open file: %v\n", err)
		return fmt.Errorf("failed to open file: %w", err)
	}

	return nil
}

// GetOwlcmsInstallDir returns the OWLCMS installation directory for the current platform.
// This is used by other packages (like tracker) that need to check OWLCMS versions
// without creating an import cycle.
func GetOwlcmsInstallDir() string {
	switch GetGoos() {
	case "windows":
		return os.Getenv("APPDATA") + string(os.PathSeparator) + "owlcms"
	case "darwin":
		return os.Getenv("HOME") + "/Library/Application Support/owlcms"
	case "linux":
		return os.Getenv("HOME") + "/.local/share/owlcms"
	default:
		return "./owlcms"
	}
}
