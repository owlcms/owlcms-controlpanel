package video

import (
	"log"
	"os"
	"path/filepath"

	"owlcms-launcher/shared"

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
		return filepath.Join(os.Getenv("APPDATA"), "owlcms-video")
	case "darwin":
		return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "owlcms-video")
	case "linux":
		return filepath.Join(os.Getenv("HOME"), ".local", "share", "owlcms-video")
	default:
		return "./owlcms-video"
	}
}

// GetInstallDir returns the video installation directory
func GetInstallDir() string {
	return getInstallDir()
}

// InitEnv creates or loads env.properties in the video install directory
func InitEnv() error {
	log.Println("Initializing video environment")
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
