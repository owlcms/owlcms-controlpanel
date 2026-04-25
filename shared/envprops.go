package shared

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/magiconair/properties"
)

func runtimeGOARCH() string { return runtime.GOARCH }

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

// RemoveEnv removes any KEY=... entries with the given key from an
// environment slice.
func RemoveEnv(env []string, key string) []string {
	prefix := key + "="
	out := env[:0]
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		out = append(out, entry)
	}
	return out
}

// NormalizeChildProcessEnv adjusts a child-process environment to work
// around platform-specific quirks. On Windows, when the parent process is
// a non-native binary running under WoW emulation (e.g. an ARM64 controlpanel
// binary on an AMD64 host, or an x86 binary on an x64 host), Windows sets
// PROCESSOR_ARCHITECTURE to the emulated architecture and
// PROCESSOR_ARCHITEW6432 to the real OS architecture. Native libraries
// such as jSerialComm read PROCESSOR_ARCHITECTURE to decide which native
// DLL to extract, which then fails to load because the JVM child process
// itself is native (matches the real OS arch). We restore the OS-native
// values for child processes.
//
// Some packagers (e.g. fyne-cross via the NSIS-style installer) launch the
// app with a stub/wrapper that produces the same WoW-style env even when
// the final binary is native AMD64; we therefore also normalize whenever
// PROCESSOR_ARCHITECTURE disagrees with runtime.GOARCH of this process.
func NormalizeChildProcessEnv(env []string) []string {
	if runtime.GOOS != "windows" {
		return env
	}
	cur := os.Getenv("PROCESSOR_ARCHITECTURE")
	real := os.Getenv("PROCESSOR_ARCHITEW6432")
	log.Printf("NormalizeChildProcessEnv: PROCESSOR_ARCHITECTURE=%q PROCESSOR_ARCHITEW6432=%q runtime.GOARCH=%s", cur, real, runtimeGOARCH())

	target := real
	if target == "" {
		target = goarchToProcArch(runtimeGOARCH())
		if target == "" || strings.EqualFold(target, cur) {
			return env
		}
	}

	log.Printf("NormalizeChildProcessEnv: forcing PROCESSOR_ARCHITECTURE=%s (was %q), removing PROCESSOR_ARCHITEW6432", target, cur)
	env = UpsertEnv(env, "PROCESSOR_ARCHITECTURE", target)
	env = RemoveEnv(env, "PROCESSOR_ARCHITEW6432")
	return env
}

func goarchToProcArch(goarch string) string {
	switch goarch {
	case "amd64":
		return "AMD64"
	case "arm64":
		return "ARM64"
	case "386":
		return "x86"
	default:
		return ""
	}
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
