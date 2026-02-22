package cameras

import (
	"log"
	"os"
	"path/filepath"

	"controlpanel/shared"

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

func getInstallDir() string {
	switch shared.GetGoos() {
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "owlcms-cameras")
	case "darwin":
		return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "owlcms-cameras")
	case "linux":
		return filepath.Join(os.Getenv("HOME"), ".local", "share", "owlcms-cameras")
	default:
		return "./owlcms-cameras"
	}
}

// GetInstallDir returns the cameras installation directory
func GetInstallDir() string {
	return getInstallDir()
}

// InitEnv creates or loads env.properties in the cameras install directory
func InitEnv() error {
	log.Println("Initializing cameras environment")
	envFilePath := filepath.Join(installDir, "env.properties")
	if _, err := os.Stat(envFilePath); os.IsNotExist(err) {
		log.Printf("env.properties not found at %s, creating with defaults", envFilePath)

		if err := shared.EnsureDir0755(installDir); err != nil {
			return err
		}

		props := properties.NewProperties()
		props.Set("VIDEO_CONFIGDIR", installDir)

		file, err := os.Create(envFilePath)
		if err != nil {
			return err
		}
		defer file.Close()

		if _, err := props.Write(file, properties.UTF8); err != nil {
			return err
		}
		log.Printf("Created env.properties at %s", envFilePath)
	}

	props, err := properties.LoadFile(envFilePath, properties.UTF8)
	if err != nil {
		return err
	}
	environment = props
	return nil
}
