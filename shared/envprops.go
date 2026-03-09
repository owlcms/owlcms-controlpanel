package shared

import (
	"fmt"
	"os"
	"strings"

	"github.com/magiconair/properties"
)

func overlayPropertiesFromFile(dest *properties.Properties, envFilePath string) error {
	if strings.TrimSpace(envFilePath) == "" {
		return nil
	}

	content, err := os.ReadFile(envFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading env.properties %s: %w", envFilePath, err)
	}

	props := properties.NewProperties()
	if err := props.Load(content, properties.UTF8); err != nil {
		return fmt.Errorf("loading env.properties %s: %w", envFilePath, err)
	}

	for _, key := range props.Keys() {
		value, _ := props.Get(key)
		dest.Set(key, value)
	}

	return nil
}

// MergeEnvironmentProperties loads parent env.properties and overlays an
// optional version-specific env.properties on top of it.
func MergeEnvironmentProperties(parentEnvPath, releaseEnvPath string) (*properties.Properties, error) {
	merged := properties.NewProperties()
	if err := overlayPropertiesFromFile(merged, parentEnvPath); err != nil {
		return nil, err
	}
	if err := overlayPropertiesFromFile(merged, releaseEnvPath); err != nil {
		return nil, err
	}
	return merged, nil
}

// UpsertEnv sets or replaces a KEY=value entry in an environment slice.
func UpsertEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

// ApplyPropertiesToEnv merges property values into an environment slice.
func ApplyPropertiesToEnv(env []string, props *properties.Properties, skipKeys map[string]struct{}) []string {
	if props == nil {
		return env
	}

	for _, key := range props.Keys() {
		if _, skip := skipKeys[key]; skip {
			continue
		}
		value, _ := props.Get(key)
		env = UpsertEnv(env, key, value)
	}

	return env
}

// SavePropertyToFile upserts a single key=value into an env.properties file.
// If the file does not exist the write is silently skipped.
func SavePropertyToFile(envFilePath, key, value string) error {
	content, err := os.ReadFile(envFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing to update
		}
		return fmt.Errorf("reading %s: %w", envFilePath, err)
	}

	props := properties.NewProperties()
	if err := props.Load(content, properties.UTF8); err != nil {
		return fmt.Errorf("loading %s: %w", envFilePath, err)
	}

	props.Set(key, value)

	file, err := os.Create(envFilePath)
	if err != nil {
		return fmt.Errorf("opening %s for writing: %w", envFilePath, err)
	}
	defer file.Close()

	if _, err := props.Write(file, properties.UTF8); err != nil {
		return fmt.Errorf("writing %s: %w", envFilePath, err)
	}

	return nil
}
