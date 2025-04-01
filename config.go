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
	launcherVersion = "2.3.0"              // Default launcher version
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

func InitEnv() {
	// Check for the presence of env.properties file in the owlcmsInstallDir
	props := properties.NewProperties()
	envFilePath := filepath.Join(owlcmsInstallDir, "env.properties")
	if _, err := os.Stat(envFilePath); os.IsNotExist(err) {
		// Create env.properties file with entry "OWLCMS_PORT=8080"
		props.Set("OWLCMS_PORT", "8080")
		file, err := os.Create(envFilePath)
		if err != nil {
			log.Fatalf("Failed to create env.properties file: %v", err)
		}
		defer file.Close()
		if _, err := props.Write(file, properties.UTF8); err != nil {
			log.Fatalf("Failed to write env.properties file: %v", err)
		}
		// Add commented-out entries
		rawString := `# Add any environment variable you need. (remove the leading # to uncomment)
#OWLCMS_INITIALDATA=LARGEGROUP_DEMO
#OWLCMS_RESETMODE=true
#OWLCMS_MEMORYMODE=true

# this overrides all the feature toggles in the database (remove the leading # to uncomment)
#OWLCMS_FEATURESWITCHES=interimScores

# java options can be set with this variable (remove the leading # to uncomment)
#JAVA_OPTIONS=-Xmx512m -Xmx512m`

		if _, err := file.WriteString(rawString); err != nil {
			log.Fatalf("Failed to write comment to env.properties file: %v", err)
		}
	}

	// Load the properties into the global variable environment
	loadProperties(envFilePath)
}

func loadProperties(envFilePath string) {
	environment = properties.NewProperties()
	file, err := os.Open(envFilePath)
	if err != nil {
		log.Fatalf("Failed to open env.properties file: %v", err)
	}
	defer file.Close()
	content, err := os.ReadFile(envFilePath)
	if err != nil {
		log.Fatalf("Failed to read env.properties file: %v", err)
	}
	if err := environment.Load(content, properties.UTF8); err != nil {
		log.Fatalf("Failed to load env.properties file: %v", err)
	}

	// // Log the properties for debugging
	// log.Printf("Loaded properties from %s:", envFilePath)
	// for _, key := range environment.Keys() {
	// 	value, _ := environment.Get(key)
	// 	log.Printf("  %s = %s", key, value)
	// }
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
		log.Println("No updates available - you are using the latest version")
		// Only show confirmation when explicitly requested
		if showConfirmation {
			dialog.ShowInformation("No Updates", fmt.Sprintf("You are running the most recent version (v%s).", currentVersion), win)
		}
	}
}
