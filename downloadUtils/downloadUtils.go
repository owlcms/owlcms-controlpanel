package downloadUtils

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// DownloadArchive downloads a zip file from the given URL and saves it to the specified path.
func DownloadArchive(url, destPath string) error {
	log.Printf("Attempting to download from URL: %s\n", url)

	client := &http.Client{
		Timeout: 180 * time.Second, // Set a timeout for the HTTP request
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download zip from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned non-200 status: %s for %s", resp.Status, url)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", destPath, err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to copy zip data: %w", err)
	}

	log.Printf("Successfully downloaded file to: %s\n", destPath)
	return nil
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

// GetDownloadURL returns the correct download URL based on the operating system.
func GetDownloadURL(baseURL string) string {
	var asset string
	goos := GetGoos()
	if IsWSL() {
		asset = "temurin.tar.gz"
	} else {
		switch goos {
		case "windows":
			asset = "temurin.zip"
		case "linux":
			asset = "temurin.tar.gz"
		default:
			asset = "temurin.zip"
		}
	}
	return fmt.Sprintf("%s/%s", baseURL, asset)
}

func GetGoos() string {
	return runtime.GOOS
}

// ExtractZip extracts a zip archive to the specified destination directory.
func ExtractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("failed to open zip file %s: %w", src, err)
	}

	for _, f := range r.File {
		if f.Name == "Procfile" || f.Name == "system.properties" {
			continue
		}

		fpath := filepath.Join(dest, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return fmt.Errorf("failed to open file for writing: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("failed to open file inside zip: %w", err)
		}

		_, err = io.Copy(outFile, rc)

		outFile.Close()
		rc.Close()

		if err != nil {
			return fmt.Errorf("failed to copy file data from zip: %w", err)
		}

		// Restore file modification and creation times
		if err := os.Chtimes(fpath, f.Modified, f.Modified); err != nil {
			return fmt.Errorf("failed to change file times: %w", err)
		}
	}

	r.Close() // Close the zip file after extracting

	// Remove the downloaded ZIP file
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("failed to remove downloaded file %s: %w", src, err)
	}

	return nil
}

// ExtractTarGz extracts a tar.gz archive to the specified destination directory.
func ExtractTarGz(tarGzPath, dest string) error {
	r, err := os.Open(tarGzPath)
	if err != nil {
		return err
	}

	gzr, err := gzip.NewReader(r)
	if err != nil {
		r.Close()
		return err
	}

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			gzr.Close()
			r.Close()
			return err
		}

		if header.Name == "Procfile" || header.Name == "system.properties" {
			continue
		}

		target := filepath.Join(dest, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				gzr.Close()
				r.Close()
				return err
			}
			if err := os.Chtimes(target, header.AccessTime, header.ModTime); err != nil {
				gzr.Close()
				r.Close()
				return err
			}
		case tar.TypeReg:
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				gzr.Close()
				r.Close()
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				gzr.Close()
				r.Close()
				return err
			}
			if err := os.Chtimes(target, header.AccessTime, header.ModTime); err != nil {
				log.Printf("failed to change file times: %v  %s %s", err, header.AccessTime, header.ModTime)
				gzr.Close()
				r.Close()
				return err
			}
			outFile.Close()
		}
	}

	gzr.Close()
	r.Close() // Close the archive after extracting

	// Remove the downloaded tar.gz file
	if err := os.Remove(tarGzPath); err != nil {
		return fmt.Errorf("failed to remove downloaded file %s: %w", tarGzPath, err)
	}

	return nil
}
