package javacheck

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
)

// CheckJava checks for Java 17 or later and downloads/installs it if necessary.
func CheckJava() error {
	// javaPath, err := findJava()
	// if err == nil {
	// 	version, err := getJavaVersion(javaPath)
	// 	if err == nil && version >= 17 {
	// 		fmt.Println("Java 17 or later found:", javaPath, "Version:", version)
	// 		return nil // Java 17+ is already installed.
	// 	}
	// }

	fmt.Println("Java 17 or later not found. Downloading from Temurin...")

	javaDir := "java17"
	if _, err := os.Stat(javaDir); os.IsNotExist(err) {
		if err := os.Mkdir(javaDir, 0755); err != nil {
			return fmt.Errorf("creating java directory: %w", err)
		}
	}

	url, err := getTemurinDownloadURL()
	if err != nil {
		return fmt.Errorf("getting Temurin download URL: %w", err)
	}

	archivePath := filepath.Join(javaDir, "temurin")
	if runtime.GOOS == "windows" {
		archivePath += ".zip"
	} else {
		archivePath += ".tar.gz"
	}

	if err := downloadFile(archivePath, url); err != nil {
		return fmt.Errorf("downloading Temurin: %w", err)
	}

	if runtime.GOOS == "windows" {
		if err := extractZip(archivePath, javaDir); err != nil {
			return fmt.Errorf("extracting Temurin zip: %w", err)
		}
	} else {
		if err := extractTarGz(archivePath, javaDir); err != nil {
			return fmt.Errorf("extracting Temurin tar.gz: %w", err)
		}
	}

	if err := os.Remove(archivePath); err != nil { // Clean up downloaded file
		return fmt.Errorf("cleaning up downloaded file: %w", err)
	}

	fmt.Println("Java downloaded and installed to ./java17")
	return nil
}

type TemurinRelease struct {
	Name    string `json:"name"`
	TagName string `json:"tag_name"`
}

// func getLatestTemurinVersion() (string, error) {
// 	// Get all releases instead of just the latest
// 	resp, err := http.Get("https://api.github.com/repos/adoptium/temurin17-binaries/releases")
// 	if err != nil {
// 		return "", fmt.Errorf("failed to fetch versions: %w", err)
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		return "", fmt.Errorf("server returned status %s", resp.Status)
// 	}

// 	body, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to read response: %w", err)
// 	}

// 	var releases []TemurinRelease
// 	if err := json.Unmarshal(body, &releases); err != nil {
// 		return "", fmt.Errorf("failed to parse response: %w", err)
// 	}

// 	// Find the latest Java 17 version and keep exact version format
// 	for _, release := range releases {
// 		if strings.HasPrefix(release.TagName, "jdk-17.") {
// 			// Split the version into parts: jdk-17.0.9+9.1 -> [17.0.9, 9.1]
// 			version := strings.TrimPrefix(release.TagName, "jdk-")
// 			parts := strings.Split(version, "+")
// 			if len(parts) != 2 {
// 				continue
// 			}
// 			// Format for download URL: 17.0.9_9.1
// 			downloadVersion := fmt.Sprintf("%s_%s", parts[0], parts[1])
// 			// Keep original version for URL path
// 			urlVersion := version
// 			return fmt.Sprintf("%s|%s", urlVersion, downloadVersion), nil
// 		}
// 	}

// 	return "", fmt.Errorf("no Java 17 version found")
// }

func getTemurinDownloadURL() (string, error) {
	// Use fixed version 17.0.9+9 which is known to work
	baseURL := "https://github.com/adoptium/temurin17-binaries/releases/download/jdk-17.0.9%2B9/"
	var filename string

	osName := runtime.GOOS
	arch := runtime.GOARCH

	switch osName {
	case "windows":
		filename = "OpenJDK17U-jre_x64_windows_hotspot_17.0.9_9.zip"
	case "darwin":
		if arch == "arm64" {
			filename = "OpenJDK17U-jre_aarch64_mac_hotspot_17.0.9_9.tar.gz"
		} else {
			filename = "OpenJDK17U-jre_x64_mac_hotspot_17.0.9_9.tar.gz"
		}
	case "linux":
		switch arch {
		case "amd64":
			filename = "OpenJDK17U-jre_x64_linux_hotspot_17.0.9_9.tar.gz"
		case "arm64":
			filename = "OpenJDK17U-jre_aarch64_linux_hotspot_17.0.9_9.tar.gz"
		case "arm":
			filename = "OpenJDK17U-jre_arm_linux_hotspot_17.0.9_9.tar.gz"
		default:
			return "", fmt.Errorf("unsupported linux architecture: %s", arch)
		}
	default:
		return "", fmt.Errorf("unsupported operating system: %s", osName)
	}
	return baseURL + filename, nil
}

func findJava() (string, error) {
	javaHome := os.Getenv("JAVA_HOME")
	if javaHome != "" {
		javaExecutable := filepath.Join(javaHome, "bin", "java")
		if runtime.GOOS == "windows" {
			javaExecutable += ".exe" // Add .exe extension on Windows
		}
		if _, err := os.Stat(javaExecutable); err == nil {
			return javaExecutable, nil
		}
	}
	javaPath, err := exec.LookPath("java")
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

func extractTarGz(tarGzPath, dest string) error {
	r, err := os.Open(tarGzPath)
	if err != nil {
		return err
	}
	defer r.Close()

	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dest, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}
	return nil
}

func downloadFile(filepath string, url string) error {
	fmt.Printf("Attempting to download from URL: %s\n", url)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %s", resp.Status)
	}

	// Check if we have a content length
	if resp.ContentLength <= 0 {
		return fmt.Errorf("invalid file size received from server")
	}

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()

	// Copy with size validation
	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	if resp.ContentLength != written {
		return fmt.Errorf("incomplete download: expected %d bytes but got %d", resp.ContentLength, written)
	}

	return nil
}

func extractZip(zipPath, dest string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, os.ModePerm); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}
