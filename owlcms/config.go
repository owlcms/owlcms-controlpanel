package owlcms

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"owlcms-launcher/shared"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/Masterminds/semver/v3"
	"github.com/magiconair/properties"
)

var (
	environment *properties.Properties
	installDir  = getInstallDir()
)

// GetLauncherVersion returns the current launcher version
func GetLauncherVersion() string {
	return shared.GetLauncherVersion()
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

// GetTemurinVersion returns the configured Temurin version from env.properties
func GetTemurinVersion() string {
	if environment == nil {
		return "jdk-25"
	}
	version, ok := environment.Get("TEMURIN_VERSION")
	if !ok {
		return "jdk-25"
	}
	return version
}

// GetTemurinVersionForRelease returns the Temurin version for a specific release.
// It first checks the version-specific env.properties file, then falls back to
// the shared env.properties file, and finally to the default value.
func GetTemurinVersionForRelease(releaseVersion string) string {
	// First, try to load version-specific env.properties
	versionEnvPath := filepath.Join(installDir, releaseVersion, "env.properties")
	if _, err := os.Stat(versionEnvPath); err == nil {
		// Version-specific env.properties exists, try to load it
		versionProps := properties.NewProperties()
		content, err := os.ReadFile(versionEnvPath)
		if err == nil {
			if err := versionProps.Load(content, properties.UTF8); err == nil {
				if temurinVersion, ok := versionProps.Get("TEMURIN_VERSION"); ok {
					log.Printf("Using version-specific Temurin %s for owlcms %s", temurinVersion, releaseVersion)
					return temurinVersion
				}
			}
		}
	}

	// Fall back to shared env.properties
	temurinVersion := GetTemurinVersion()
	log.Printf("Using shared Temurin %s for owlcms %s", temurinVersion, releaseVersion)
	return temurinVersion
}

// GetInstallDir returns the installation directory
func GetInstallDir() string {
	return installDir
}

// GetEnvironment returns the environment properties
func GetEnvironment() *properties.Properties {
	return environment
}

func getInstallDir() string {
	switch shared.GetGoos() {
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "owlcms")
	case "darwin":
		return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "owlcms")
	case "linux":
		return filepath.Join(os.Getenv("HOME"), ".local", "share", "owlcms")
	default:
		return "./owlcms"
	}
}

// InitEnv initializes the environment properties from env.properties
func InitEnv() error {
	envFilePath := filepath.Join(installDir, "env.properties")
	if _, err := os.Stat(envFilePath); os.IsNotExist(err) {
		log.Printf("env.properties file not found at %s, creating with default values", envFilePath)

		if err := shared.EnsureDir0755(installDir); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", installDir, err)
		}

		props := properties.NewProperties()
		props.Set("OWLCMS_PORT", "8080")
		props.Set("TEMURIN_VERSION", "jdk-25")

		file, err := os.Create(envFilePath)
		if err != nil {
			return fmt.Errorf("failed to create env.properties file: %w", err)
		}
		defer file.Close()

		if _, err := props.Write(file, properties.UTF8); err != nil {
			return fmt.Errorf("failed to write env.properties file: %w", err)
		}

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

	log.Printf("Loaded properties from %s:", envFilePath)
	for _, key := range environment.Keys() {
		value, _ := environment.Get(key)
		log.Printf("  %s = %s", key, value)
	}

	return nil
}

// SaveProperty saves a key-value pair to env.properties and reloads the environment
func SaveProperty(key, value string) error {
	envFilePath := filepath.Join(installDir, "env.properties")

	// Ensure environment is loaded
	if environment == nil {
		if err := InitEnv(); err != nil {
			return fmt.Errorf("failed to initialize environment: %w", err)
		}
	}

	// Set the property
	environment.Set(key, value)

	// Write back to file
	file, err := os.Create(envFilePath)
	if err != nil {
		return fmt.Errorf("failed to open env.properties for writing: %w", err)
	}
	defer file.Close()

	if _, err := environment.Write(file, properties.UTF8); err != nil {
		return fmt.Errorf("failed to write env.properties: %w", err)
	}

	log.Printf("Saved property %s = %s to %s", key, value, envFilePath)
	return nil
}

// DeleteProperty removes a key from env.properties and reloads the environment
func DeleteProperty(key string) error {
	envFilePath := filepath.Join(installDir, "env.properties")

	// Ensure environment is loaded
	if environment == nil {
		if err := InitEnv(); err != nil {
			return fmt.Errorf("failed to initialize environment: %w", err)
		}
	}

	// Delete the property
	environment.Delete(key)

	// Write back to file
	file, err := os.Create(envFilePath)
	if err != nil {
		return fmt.Errorf("failed to open env.properties for writing: %w", err)
	}
	defer file.Close()

	if _, err := environment.Write(file, properties.UTF8); err != nil {
		return fmt.Errorf("failed to write env.properties: %w", err)
	}

	log.Printf("Deleted property %s from %s", key, envFilePath)
	return nil
}

// GetTrackerConnectionEnabled returns true if OWLCMS_VIDEODATA is set to a local tracker URL
func GetTrackerConnectionEnabled() bool {
	if environment == nil {
		return false
	}
	value, ok := environment.Get("OWLCMS_VIDEODATA")
	if !ok {
		return false
	}
	// Check if it's a local tracker URL (ws://127.0.0.1:*/ws)
	return value != "" && (value[:len("ws://127.0.0.1:")] == "ws://127.0.0.1:" && value[len(value)-3:] == "/ws")
}

// CheckForUpdates checks for updates to the control panel itself
func CheckForUpdates(win fyne.Window, showConfirmation bool) {
	currentVersion := shared.GetLauncherVersion()
	if len(currentVersion) > 0 && currentVersion[0] == 'v' {
		currentVersion = currentVersion[1:]
	}
	// Normalize SNAPSHOT to lowercase so it sorts after rc/alpha/beta in semver comparison
	currentVersion = strings.ReplaceAll(currentVersion, "-SNAPSHOT", "-snapshot")

	currentSemver, currentErr := semver.NewVersion(currentVersion)
	if currentErr != nil {
		log.Printf("Failed to parse current version string: %v", currentErr)
		return
	}

	isCurrentPrerelease := currentSemver.Prerelease() != ""

	// If current is a prerelease, check all releases; otherwise just check latest stable
	var repoURL string
	if isCurrentPrerelease {
		repoURL = "https://api.github.com/repos/owlcms/owlcms-controlpanel/releases"
	} else {
		repoURL = "https://api.github.com/repos/owlcms/owlcms-controlpanel/releases/latest"
	}
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

	type releaseInfo struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}

	var newerRelease *releaseInfo
	var newerSemver *semver.Version

	if isCurrentPrerelease {
		// Parse array of releases and find the newest one (stable or prerelease) that's newer than current
		var releases []releaseInfo
		if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
			log.Printf("Failed to parse update information: %v", err)
			return
		}

		for _, release := range releases {
			remoteVersion := release.TagName
			if len(remoteVersion) > 0 && remoteVersion[0] == 'v' {
				remoteVersion = remoteVersion[1:]
			}
			// Normalize SNAPSHOT to lowercase so it sorts after rc/alpha/beta in semver comparison
			remoteVersion = strings.ReplaceAll(remoteVersion, "-SNAPSHOT", "-snapshot")

			remoteSemver, remoteErr := semver.NewVersion(remoteVersion)
			if remoteErr != nil {
				continue
			}

			if remoteSemver.GreaterThan(currentSemver) {
				if newerSemver == nil || remoteSemver.GreaterThan(newerSemver) {
					newerSemver = remoteSemver
					releaseCopy := release
					newerRelease = &releaseCopy
				}
			}
		}
	} else {
		// Just check latest stable release
		var release releaseInfo
		if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
			log.Printf("Failed to parse update information: %v", err)
			return
		}

		remoteVersion := release.TagName
		if len(remoteVersion) > 0 && remoteVersion[0] == 'v' {
			remoteVersion = remoteVersion[1:]
		}
		// Normalize SNAPSHOT to lowercase so it sorts after rc/alpha/beta in semver comparison
		remoteVersion = strings.ReplaceAll(remoteVersion, "-SNAPSHOT", "-snapshot")

		remoteSemver, remoteErr := semver.NewVersion(remoteVersion)
		if remoteErr != nil {
			log.Printf("Failed to parse version strings for comparison: %v", remoteErr)
			return
		}

		if remoteSemver.GreaterThan(currentSemver) {
			newerSemver = remoteSemver
			newerRelease = &release
		}
	}

	// Check if we found a newer version
	if newerRelease != nil && newerSemver != nil {
		log.Printf("Update available: %s -> %s", currentSemver, newerSemver)

		parsedURL, err := url.Parse(newerRelease.HTMLURL)
		if err != nil {
			log.Printf("Failed to parse release URL: %v", err)
			return
		}
		link := widget.NewHyperlink("Release Notes and Installer", parsedURL)
		content := container.NewVBox(
			widget.NewLabel(fmt.Sprintf("A new version (%s) is available. You are currently using version %s.\nYou can simply download the new installer and install over the current version.", newerRelease.TagName, shared.GetLauncherVersion())),
			link,
		)
		dialog.ShowCustom("Update Available", "Close", content, win)
	} else {
		log.Println("No updates available - you are using the latest version")
		if showConfirmation {
			dialog.ShowInformation("No Updates", fmt.Sprintf("You are using the latest version (%s)", shared.GetLauncherVersion()), win)
		}
	}
}
