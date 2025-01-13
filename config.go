package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/magiconair/properties"
)

var (
	launcherVersion = "1.0.0"              // Default launcher version
	buildVersion    = "_TAG_"              // Placeholder for build version
	environment     *properties.Properties // Global variable to hold the environment properties
)

func init() {
	if buildVersion != "_TAG_" {
		launcherVersion = buildVersion
	}
}

// GetPort returns the configured port from env.properties, defaulting to "8080"
func GetPort() string {
	if environment == nil {
		return "8080"
	}
	port, ok := environment.Get("PORT")
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
		// Create env.properties file with entry "PORT=8080"
		props.Set("PORT", "8080")
		file, err := os.Create(envFilePath)
		if err != nil {
			log.Fatalf("Failed to create env.properties file: %v", err)
		}
		defer file.Close()
		if _, err := props.Write(file, properties.UTF8); err != nil {
			log.Fatalf("Failed to write env.properties file: %v", err)
		}
		// Add commented-out entries
		if _, err := file.WriteString("# Add any environment variable you need.\n"); err != nil {
			log.Fatalf("Failed to write comment to env.properties file: %v", err)
		}
		file.WriteString("#OWLCMS_INITIALDATA=LARGEGROUP_DEMO\n")
		file.WriteString("#OWLCMS_MEMORYMODE=false\n")
		file.WriteString("#OWLCMS_RESETMODE=false\n")
	}

	// Load the properties into the global variable environment
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

	// Log the properties for debugging
	log.Printf("Loaded properties from %s:", envFilePath)
	for _, key := range environment.Keys() {
		value, _ := environment.Get(key)
		log.Printf("  %s = %s", key, value)
	}
}
