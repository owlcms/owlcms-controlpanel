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
	"github.com/magiconair/properties"
)

var (
	launcherVersion = "1.0.0"              // Default launcher version
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
		return "8091"
	}
	port, ok := environment.Get("FIRMATA_PORT")
	if !ok {
		return "8091"
	}
	return port
}

func InitEnv() {
	// Check for the presence of env.properties file in the owlcmsInstallDir
	props := properties.NewProperties()
	envFilePath := filepath.Join(owlcmsInstallDir, "env.properties")
	if _, err := os.Stat(envFilePath); os.IsNotExist(err) {
		// Create env.properties file with entry "FIRMATA_PORT=8080"
		props.Set("FIRMATA_PORT", "8080")
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

func checkForUpdates(win fyne.Window) {
	const repoURL = "https://api.github.com/repos/firmata/firmata-controlpanel/releases/latest"
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

	if release.TagName > launcherVersion {
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
		dialog.ShowInformation("No Updates", "You are using the latest version.", win)
	}
}
