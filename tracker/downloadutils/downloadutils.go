package downloadutils

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"owlcms-launcher/shared"
)

// DownloadArchive downloads a zip file from the given URL and saves it to the specified path.
// Includes progress callback support.
func DownloadArchive(url, destPath string, progressCallback func(downloaded, total int64), cancelChan chan struct{}) error {
	log.Printf("Attempting to download from URL: %s\n", url)

	client := &http.Client{
		Timeout: 300 * time.Second, // Set a longer timeout for larger files
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

	// Get total size for progress reporting
	totalSize := resp.ContentLength

	// Create a progress writer
	var downloaded int64
	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		// Check for cancellation
		select {
		case <-cancelChan:
			return fmt.Errorf("download cancelled")
		default:
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := out.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("failed to write to file: %w", writeErr)
			}
			downloaded += int64(n)
			if progressCallback != nil && totalSize > 0 {
				progressCallback(downloaded, totalSize)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
	}

	log.Printf("Successfully downloaded file to: %s\n", destPath)
	return nil
}

// IsWSL checks if the program is running under Windows Subsystem for Linux.
// Delegates to shared package.
func IsWSL() bool {
	return shared.IsWSL()
}

// GetGoos returns the current operating system identifier.
// Delegates to shared package.
func GetGoos() string {
	return shared.GetGoos()
}

// GetGoarch returns the current architecture identifier.
// Delegates to shared package.
func GetGoarch() string {
	return shared.GetGoarch()
}

// ExtractZip extracts a zip file to the specified destination directory.
func ExtractZip(zipPath, destDir string) error {
	log.Printf("Extracting ZIP file: %s to %s\n", zipPath, destDir)

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer r.Close()

	// Create the destination directory if it doesn't exist
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	for _, f := range r.File {
		fpath := filepath.Join(destDir, f.Name)

		// Check for ZipSlip vulnerability
		if !strings.HasPrefix(fpath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return fmt.Errorf("failed to open file in zip: %w", err)
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return fmt.Errorf("failed to extract file: %w", err)
		}
	}

	// Delete the zip file after successful extraction
	if err := os.Remove(zipPath); err != nil {
		log.Printf("Warning: failed to delete zip file: %v\n", err)
	} else {
		log.Printf("Deleted zip file: %s\n", zipPath)
	}

	log.Printf("Successfully extracted ZIP to: %s\n", destDir)
	return nil
}
