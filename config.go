package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/Masterminds/semver/v3"
	"github.com/magiconair/properties"
)

var (
	launcherVersion = "2.6.0"              // Default launcher version
	buildVersion    = "_TAG_"              // Placeholder for build version
	environment     *properties.Properties // Global variable to hold the environment properties
)

func init() {
	if buildVersion != ("_" + "TAG" + "_") {
		// not running in a development environment
		launcherVersion = buildVersion
	}
}

// GetPort returns the configured port from env.properties, defaulting to "8080"
func GetPort() string {
	if environment == nil {
		return "8080"
	}
	port, ok := environment.Get("OWLCMS_PORT")
	if !ok {
		return "8080"
	}
	return port
}

// GetTemurinVersion returns the configured Temurin version from env.properties, defaulting to "jdk-17.0.15+6"
func GetTemurinVersion() string {
	if environment == nil {
		return "jdk-17.0.15+6"
	}
	version, ok := environment.Get("TEMURIN_VERSION")
	if !ok {
		return "jdk-17.0.15+6"
	}
	return version
}

func InitEnv() error {
	// Check for the presence of env.properties file in the owlcmsInstallDir
	envFilePath := filepath.Join(owlcmsInstallDir, "env.properties")
	if _, err := os.Stat(envFilePath); os.IsNotExist(err) {
		log.Printf("env.properties file not found at %s, creating with default values", envFilePath)

		// Ensure the directory exists before creating the file
		if err := os.MkdirAll(owlcmsInstallDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", owlcmsInstallDir, err)
		}

		// Create env.properties file with default entries
		props := properties.NewProperties()
		props.Set("OWLCMS_PORT", "8080")
		props.Set("TEMURIN_VERSION", "jdk-17.0.15+6")

		file, err := os.Create(envFilePath)
		if err != nil {
			return fmt.Errorf("failed to create env.properties file: %w", err)
		}
		defer file.Close()

		if _, err := props.Write(file, properties.UTF8); err != nil {
			return fmt.Errorf("failed to write env.properties file: %w", err)
		}

		// Add commented-out entries for user reference
		rawString := `# Add any environment variable you need. (remove the leading # to uncomment)
#OWLCMS_INITIALDATA=LARGEGROUP_DEMO
#OWLCMS_RESETMODE=true
#OWLCMS_MEMORYMODE=true

# this overrides all the feature toggles in the database (remove the leading # to uncomment)
#OWLCMS_FEATURESWITCHES=interimScores

# java options can be set with this variable (remove the leading # to uncomment)
#JAVA_OPTIONS=-Xmx512m -Xmx512m`

		if _, err := file.WriteString(rawString); err != nil {
			log.Printf("Failed to write comments to env.properties file: %v, but file is usable", err)
		}

		log.Printf("Successfully created env.properties file at %s", envFilePath)
	}

	// Load the properties into the global variable environment
	return loadProperties(envFilePath)
}

func loadProperties(envFilePath string) error {
	environment = properties.NewProperties()
	file, err := os.Open(envFilePath)
	if err != nil {
		return fmt.Errorf("failed to open env.properties file: %w", err)
	}
	defer file.Close()
	content, err := os.ReadFile(envFilePath)
	if err != nil {
		return fmt.Errorf("failed to read env.properties file: %w", err)
	}
	if err := environment.Load(content, properties.UTF8); err != nil {
		return fmt.Errorf("failed to load env.properties file: %w", err)
	}

	// Log the properties for debugging
	log.Printf("Loaded properties from %s:", envFilePath)
	for _, key := range environment.Keys() {
		value, _ := environment.Get(key)
		log.Printf("  %s = %s", key, value)
	}

	return nil
}

func checkForUpdates(win fyne.Window, showConfirmation bool) {
	const repoURL = "https://api.github.com/repos/owlcms/owlcms-controlpanel/releases/latest"
	log.Println("Checking for updates from:", repoURL)

	resp, err := http.Get(repoURL)
	if err != nil {
		log.Printf("Failed to check for updates: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to check for updates: received status code %d", resp.StatusCode)
		return
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		log.Printf("Failed to parse update information: %v", err)
		return
	}

	// Remove 'v' prefix if present in both versions for comparison
	remoteVersion := release.TagName
	if len(remoteVersion) > 0 && remoteVersion[0] == 'v' {
		remoteVersion = remoteVersion[1:]
	}

	currentVersion := launcherVersion
	if len(currentVersion) > 0 && currentVersion[0] == 'v' {
		currentVersion = currentVersion[1:]
	}

	// Parse versions for semantic comparison
	remoteSemver, remoteErr := semver.NewVersion(remoteVersion)
	currentSemver, currentErr := semver.NewVersion(currentVersion)

	if remoteErr != nil || currentErr != nil {
		log.Printf("Failed to parse version strings for comparison: remote=%v, current=%v", remoteErr, currentErr)
		return
	}

	log.Printf("Comparing versions - Remote: %s, Current: %s", remoteSemver, currentSemver)
	log.Printf("Remote greater than current?: %t", remoteSemver.GreaterThan(currentSemver))

	if remoteSemver.GreaterThan(currentSemver) {
		// Only prompt if the remote version is a stable release (no pre-release tag)
		if remoteSemver.Prerelease() == "" {
			log.Printf("Update available: %s -> %s", currentSemver, remoteSemver)

			url, err := url.Parse(release.HTMLURL)
			if err != nil {
				log.Printf("Failed to parse release URL: %v", err)
				return
			}
			link := widget.NewHyperlink("Release Notes and Installer", url)
			content := container.NewVBox(
				widget.NewLabel(fmt.Sprintf("A new version (%s) is available. You are currently using version %s.\nYou can simply download the new installer and install over the current version.", release.TagName, launcherVersion)),
				link,
			)
			dialog.ShowCustom("Update Available", "Close", content, win)
		} else {
			log.Printf("Remote version %s is a pre-release. No update prompt will be shown.", remoteSemver)
		}
	} else {
		log.Println("No updates available - you are using the latest version")
		// Only show confirmation when explicitly requested
		if showConfirmation {
			dialog.ShowInformation("No Updates", fmt.Sprintf("You are running the most recent version (v%s).", currentVersion), win)
		}
	}
}
