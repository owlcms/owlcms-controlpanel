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

const tailviewerPath = `C:\Program Files\Tailviewer\Tailviewer.exe`

func bashSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

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

// ResetLogFile ensures the parent directory exists and truncates the log file.
func ResetLogFile(logPath string) error {
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to reset log file: %w", err)
	}
	return file.Close()
}

// TailLogFile opens a terminal tailing the provided log file.
func TailLogFile(logPath string) error {
	goos := GetGoos()

	switch goos {
	case "windows":
		if _, err := os.Stat(tailviewerPath); err == nil {
			if err := exec.Command(tailviewerPath, logPath).Start(); err != nil {
				return fmt.Errorf("failed to start Tailviewer: %w", err)
			}
			return nil
		}

		escaped := strings.ReplaceAll(logPath, "'", "''")
		psCmd := fmt.Sprintf("Get-Content -Path '%s' -Wait -Tail 200", escaped)
		if err := exec.Command("cmd", "/c", "start", "powershell", "-NoExit", "-Command", psCmd).Start(); err != nil {
			return fmt.Errorf("failed to start PowerShell tail: %w", err)
		}
		return nil

	case "darwin":
		script := fmt.Sprintf(`tell application "Terminal" to do script "tail -n 200 -f %s"`, bashSingleQuote(logPath))
		if err := exec.Command("osascript", "-e", script, "-e", `tell application "Terminal" to activate`).Start(); err != nil {
			return fmt.Errorf("failed to start Terminal tail: %w", err)
		}
		return nil

	case "linux":
		cmd := fmt.Sprintf("tail -n 200 -f %s", bashSingleQuote(logPath))
		try := func(name string, args ...string) bool {
			if _, err := exec.LookPath(name); err != nil {
				return false
			}
			if err := exec.Command(name, args...).Start(); err != nil {
				log.Printf("Failed to start %s for tail: %v", name, err)
				return false
			}
			return true
		}
		if try("x-terminal-emulator", "-e", "bash", "-lc", cmd) {
			return nil
		}
		if try("gnome-terminal", "--", "bash", "-lc", cmd) {
			return nil
		}
		if try("konsole", "-e", "bash", "-lc", cmd) {
			return nil
		}
		if try("xfce4-terminal", "-e", "bash", "-lc", cmd) {
			return nil
		}
		if try("xterm", "-e", "bash", "-lc", cmd) {
			return nil
		}
		return fmt.Errorf("no terminal emulator found to tail logs")

	default:
		return fmt.Errorf("unsupported operating system: %s", goos)
	}
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
