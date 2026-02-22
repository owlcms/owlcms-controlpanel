package tracker

import (
	"log"
	"os"
	"path/filepath"

	"controlpanel/shared"
	"controlpanel/tracker/downloadutils"

	"github.com/magiconair/properties"
)

var (
	launcherVersion = "1.0.0"
	buildVersion    = "_TAG_"
	environment     *properties.Properties
)

func initConfig() {
	if buildVersion != ("_" + "TAG" + "_") {
		launcherVersion = buildVersion
	}
}

// GetPort returns the configured port for tracker, defaulting to "8096"
func GetPort() string {
	if environment == nil {
		return "8096"
	}
	port, ok := environment.Get("TRACKER_PORT")
	if !ok {
		return "8096"
	}
	return port
}

// InitEnv initializes the tracker environment from env.properties
func InitEnv() error {
	log.Println("Initializing tracker environment")
	// Check for the presence of env.properties file in the installDir
	envFilePath := filepath.Join(installDir, "env.properties")
	if _, err := os.Stat(envFilePath); os.IsNotExist(err) {
		log.Printf("env.properties file not found at %s, creating with default values", envFilePath)

		// Ensure the directory exists before creating the file
		if err := shared.EnsureDir0755(installDir); err != nil {
			return err
		}

		// Create env.properties file with default entry using properties library
		props := properties.NewProperties()
		props.Set("TRACKER_PORT", "8096")

		file, err := os.Create(envFilePath)
		if err != nil {
			return err
		}
		defer file.Close()

		if _, err := props.Write(file, properties.UTF8); err != nil {
			return err
		}

		log.Printf("Successfully created env.properties file at %s", envFilePath)
	}

	// Load environment from env.properties
	props, err := properties.LoadFile(envFilePath, properties.UTF8)
	if err != nil {
		return err
	}
	environment = props

	// Log the properties for debugging
	log.Printf("Loaded properties from %s:", envFilePath)
	for _, key := range environment.Keys() {
		value, _ := environment.Get(key)
		log.Printf("  %s = %s", key, value)
	}

	return nil
}

func getInstallDir() string {
	switch downloadutils.GetGoos() {
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "owlcms-tracker")
	case "darwin":
		return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "owlcms-tracker")
	case "linux":
		return filepath.Join(os.Getenv("HOME"), ".local", "share", "owlcms-tracker")
	default:
		return "./owlcms-tracker"
	}
}

// GetInstallDir returns the installation directory used by the tracker package
func GetInstallDir() string {
	return getInstallDir()
}
