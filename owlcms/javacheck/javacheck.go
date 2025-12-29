package javacheck

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"

	customdialog "owlcms-launcher/owlcms/dialog"
	"owlcms-launcher/owlcms/downloadutils"
	"owlcms-launcher/shared"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

var owlcmsInstallDir string
var getTemurinVersionFunc func() string

func InitJavaCheck(installDir string, getVersionFunc func() string) {
	owlcmsInstallDir = installDir
	getTemurinVersionFunc = getVersionFunc
}

// compareVersions compares two jdk directory names and returns true if a is more recent than b
func compareVersions(a, b string) bool {
	// Extract version numbers from directory names (e.g., "jdk-17.0.9+9" -> "17.0.9")
	aVersion := strings.TrimPrefix(a, "jdk-")
	bVersion := strings.TrimPrefix(b, "jdk-")

	// Split into components
	aParts := strings.Split(strings.Split(aVersion, "+")[0], ".")
	bParts := strings.Split(strings.Split(bVersion, "+")[0], ".")

	// Compare each component
	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		aNum, _ := strconv.Atoi(aParts[i])
		bNum, _ := strconv.Atoi(bParts[i])
		if aNum != bNum {
			return aNum > bNum
		}
	}
	return len(aParts) > len(bParts)
}

func FindLocalJava() (string, error) {
	javaDir := filepath.Join(owlcmsInstallDir, "java17")
	if _, err := os.Stat(javaDir); err != nil {
		log.Printf("*** Java directory not found: %v\n", err)
		return "", fmt.Errorf("java17 directory not found")
	}

	entries, err := os.ReadDir(javaDir)
	if err != nil {
		log.Printf("*** Error reading java directory: %v\n", err)
		return "", fmt.Errorf("reading java directory: %w", err)
	}

	// Find directories starting with "jdk" or "jre"
	var jdkDirs []string
	for _, entry := range entries {
		if entry.IsDir() && (strings.HasPrefix(entry.Name(), "jdk") || strings.HasPrefix(entry.Name(), "jre")) {
			jdkDirs = append(jdkDirs, entry.Name())
		}
	}

	if len(jdkDirs) == 0 {
		log.Printf("*** No Java installation found in %s\n", javaDir)
		return "", fmt.Errorf("no Java installation found in %s", javaDir)
	}

	// Sort to get the latest version using semantic versioning
	sort.Slice(jdkDirs, func(i, j int) bool {
		return compareVersions(jdkDirs[i], jdkDirs[j])
	})
	latestJDK := jdkDirs[0]

	// Check for java executable
	var javaExe string
	var javaPath string
	goos := downloadutils.GetGoos()

	if goos == "windows" && !isWSL() {
		javaExe = "javaw.exe"
		javaPath = filepath.Join(javaDir, latestJDK, "bin", javaExe)
	} else if goos == "darwin" {
		javaExe = "java"
		javaPath = filepath.Join(javaDir, latestJDK, "Contents", "Home", "bin", javaExe)
	} else if goos == "linux" {
		javaExe = "java"
		javaPath = filepath.Join(javaDir, latestJDK, "bin", javaExe)
	} else {
		log.Printf("*** Unsupported OS: %s\n", goos)
		return "", fmt.Errorf("unsupported OS: %s", goos)
	}

	_, err = os.Stat(javaPath)
	if err != nil {
		log.Printf("*** Java executable NOT found in %s: %v\n", javaPath, err)
		return "", fmt.Errorf("java executable not found in %s: %v", javaPath, err)
	} else {
		log.Printf("*** Found local Java %s at: %s\n", latestJDK, javaPath)
		return javaPath, nil
	}
}

// CheckJava checks for Java 17 or later and downloads/installs it if necessary.
func CheckJava(statusLabel *widget.Label) error {
	// First check for local Java installation
	javaPath, err := FindLocalJava()
	if err == nil {
		log.Printf("*** Found local Java at: %s\n", javaPath)
		return nil
	} else {
		log.Printf("*** Local Java not found at %s: %v\n", javaPath, err)
	}

	fmt.Println("Suitable Java not found. Downloading from Temurin...")
	statusLabel.SetText("Downloading a local copy of the Java language runtime.")
	statusLabel.Refresh()
	statusLabel.Show()

	// Create a cancel channel
	cancel := make(chan bool)

	// Show the progress dialog immediately
	progressDialog, progressBar := customdialog.NewDownloadDialog(
		"Installing Java",
		fyne.CurrentApp().Driver().AllWindows()[0],
		cancel)
	progressDialog.Show()
	progressBar.SetValue(0.01) // Set a small initial value to show activity

	// Ensure the owlcms directory exists
	if _, err := os.Stat(owlcmsInstallDir); os.IsNotExist(err) {
		if err := shared.EnsureDir0755(owlcmsInstallDir); err != nil {
			progressDialog.Hide()
			return fmt.Errorf("creating owlcms directory: %w", err)
		}
	}

	javaDir := filepath.Join(owlcmsInstallDir, "java17")
	// Recursively delete the java17 directory if it exists
	if _, err := os.Stat(javaDir); err == nil {
		err := os.RemoveAll(javaDir)
		if err != nil {
			progressDialog.Hide()
			return fmt.Errorf("failed to delete existing java17 directory: %w", err)
		}
	}

	if _, err := os.Stat(javaDir); os.IsNotExist(err) {
		if err := shared.EnsureDir0755(javaDir); err != nil {
			progressDialog.Hide()
			return fmt.Errorf("creating java directory: %w", err)
		}
	}

	// Show activity while getting the download URL
	progressBar.SetValue(0.05)
	downloadURL, err := getTemurinDownloadURL()
	if err != nil {
		progressDialog.Hide()
		return fmt.Errorf("getting Temurin download URL: %w", err)
	}

	archivePath := filepath.Join(javaDir, "temurin")
	if downloadutils.GetGoos() == "windows" && !isWSL() {
		archivePath += ".zip"
	} else {
		archivePath += ".tar.gz"
	}

	progressCallback := func(downloaded, total int64) {
		if total > 0 {
			percentage := float64(downloaded) / float64(total)
			progressBar.SetValue(percentage)
		}
	}

	if err := downloadutils.DownloadArchive(downloadURL, archivePath, progressCallback, cancel); err != nil {
		progressDialog.Hide()
		if err.Error() == "download cancelled" {
			// Handle cancellation
			log.Println("Java download cancelled by user")

			// Clean up the incomplete archive file
			os.Remove(archivePath)

			return nil
		}
		return fmt.Errorf("error downloading Java: %w", err)
	}

	// Show extraction progress
	progressBar.SetValue(0.9)
	if downloadutils.GetGoos() == "windows" && !isWSL() {
		if err := downloadutils.ExtractZip(archivePath, javaDir); err != nil {
			progressDialog.Hide()
			return fmt.Errorf("error extracting Temurin zip: %w", err)
		}
	} else {
		if err := downloadutils.ExtractTarGz(archivePath, javaDir); err != nil {
			progressDialog.Hide()
			return fmt.Errorf("extracting Temurin tar.gz: %w", err)
		}
	}

	// Indicate completion
	progressBar.SetValue(1.0)
	progressDialog.Hide()

	// extract now removes the archive
	log.Printf("Java downloaded and installed to %s\n", javaDir)
	return nil
}

type TemurinRelease struct {
	Name    string `json:"name"`
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// isWSL returns true if running under Windows Subsystem for Linux
func isWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}

func findLatestTemurinRelease(version string) (string, error) {
	// Determine the API endpoint based on whether a specific version is requested
	var apiURL string
	if version == "" {
		// Get latest release info from API
		apiURL = "https://api.github.com/repos/adoptium/temurin17-binaries/releases/latest"
	} else {
		// Get specific version release info from API - URL encode the version tag
		encodedVersion := url.QueryEscape(version)
		apiURL = fmt.Sprintf("https://api.github.com/repos/adoptium/temurin17-binaries/releases/tags/%s", encodedVersion)
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "owlcms-launcher")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to fetch latest release: %v", err)
		return "", fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub API returned status %d: %s\nBody: %s", resp.StatusCode, resp.Status, string(body))
	}

	var release TemurinRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		log.Printf("Failed to parse release: %v", err)
		return "", fmt.Errorf("failed to parse release: %w", err)
	}

	log.Printf("Latest Temurin release: %s\n", release.TagName)
	return release.TagName, nil
}

func getTemurinDownloadURL() (string, error) {
	// Get the specific release tag from configuration
	var version string
	if getTemurinVersionFunc != nil {
		version = getTemurinVersionFunc()
	} else {
		version = "jdk-17.0.15+6" // fallback
	}

	tag, err := findLatestTemurinRelease(version)
	if err != nil {
		log.Printf("Failed to get version number: %v", err)
		return "", fmt.Errorf("failed to get version number: %w", err)
	}

	// Extract version number from tag (e.g., "jdk-17.0.13+11" -> "17.0.13_11")
	version = strings.TrimPrefix(tag, "jdk-")
	version = strings.ReplaceAll(version, "+", "_")

	// Use the tag to get specific release
	releaseURL := fmt.Sprintf("https://api.github.com/repos/adoptium/temurin17-binaries/releases/tags/%s", tag)

	req, err := http.NewRequest("GET", releaseURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers required by GitHub API
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "owlcms-launcher")

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to fetch release: %v", err)
		return "", fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub API returned status %d: %s\nBody: %s", resp.StatusCode, resp.Status, string(body))
	}

	var release TemurinRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		log.Printf("Failed to parse release: %v", err)
		return "", fmt.Errorf("failed to parse release: %w", err)
	}

	// Print environment info for debugging
	log.Printf("Running on: OS=%s, ARCH=%s, WSL=%v\n", downloadutils.GetGoos(), runtime.GOARCH, isWSL())

	// Always use Linux pattern for WSL/Linux, but with correct version
	var pattern string
	goos := downloadutils.GetGoos()
	if goos == "darwin" {
		switch runtime.GOARCH {
		case "amd64":
			pattern = fmt.Sprintf("OpenJDK17U-jre_x64_mac_hotspot_%s.tar.gz", version)
		case "arm64":
			pattern = fmt.Sprintf("OpenJDK17U-jre_aarch64_mac_hotspot_%s.tar.gz", version)
		default:
			return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
		}
	} else if isWSL() || goos == "linux" {
		switch runtime.GOARCH {
		case "amd64":
			pattern = fmt.Sprintf("OpenJDK17U-jre_x64_linux_hotspot_%s.tar.gz", version)
		case "arm64":
			pattern = fmt.Sprintf("OpenJDK17U-jre_aarch64_linux_hotspot_%s.tar.gz", version)
		default:
			return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
		}
	} else if goos == "windows" {
		switch runtime.GOARCH {
		case "amd64":
			pattern = fmt.Sprintf("OpenJDK17U-jre_x64_windows_hotspot_%s.zip", version)
		case "arm64":
			pattern = fmt.Sprintf("OpenJDK17U-jre_aarch64_windows_hotspot_%s.zip", version)
		default:
			return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
		}
	} else {
		return "", fmt.Errorf("unsupported OS: %s", downloadutils.GetGoos())
	}

	log.Printf("Looking for asset: %s\n", pattern)

	// Look for exact matching JRE asset
	for _, asset := range release.Assets {
		if asset.Name == pattern {
			log.Printf("Found matching JRE: %s\n", asset.Name)
			return asset.BrowserDownloadURL, nil
		}
	}

	return "", fmt.Errorf("no matching JRE found (looking for %s)", pattern)
}

func findJava() (string, error) {
	javaHome := os.Getenv("JAVA_HOME")
	javaCommand := "java"
	if downloadutils.GetGoos() == "windows" && !isWSL() {
		javaCommand = "javaw"
	}
	if javaHome != "" {
		javaExecutable := filepath.Join(javaHome, "bin", javaCommand)
		if _, err := os.Stat(javaExecutable); err == nil {
			return javaExecutable, nil
		}
	}

	javaPath, err := exec.LookPath(javaCommand)
	if err == nil {
		return javaPath, nil
	}
	return "", fmt.Errorf("java executable not found")
}

func getJavaVersion(javaPath string) (int, error) {
	cmd := exec.Command(javaPath, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("getting java version: %w", err)
	}

	versionRegex := regexp.MustCompile(`(?:java|openjdk) version "(?:(\d+)\.)?(?:(\d+)\.)?(\d+)"`)
	matches := versionRegex.FindStringSubmatch(string(output))
	if len(matches) > 1 {
		majorVersionStr := matches[1]
		if majorVersionStr == "" {
			majorVersionStr = matches[2]
			if majorVersionStr == "" {
				majorVersionStr = matches[3]
			}
		}
		majorVersion, err := strconv.Atoi(majorVersionStr)
		if err != nil {
			return 0, fmt.Errorf("parsing java version: %w", err)
		}
		return majorVersion, nil
	}

	versionRegex = regexp.MustCompile(`(?:java|openjdk) (\d+)`)
	matches = versionRegex.FindStringSubmatch(string(output))
	if len(matches) > 1 {
		majorVersion, err := strconv.Atoi(matches[1])
		if err != nil {
			return 0, fmt.Errorf("parsing java version: %w", err)
		}
		return majorVersion, nil
	}

	return 0, fmt.Errorf("could not parse java version from output: %s", string(output))
}
