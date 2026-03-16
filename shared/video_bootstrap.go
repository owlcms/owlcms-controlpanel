package shared

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const videoConfigSubdir = "video_config"

// VideoConfigDir returns the shared ffmpeg config directory under the control panel.
// Layout: <controlpanel>/video_config/ffmpeg/ffmpeg.toml
func VideoConfigDir(versionDir string) string {
	_ = versionDir
	return filepath.Join(GetControlPanelInstallDir(), videoConfigSubdir, "ffmpeg")
}

func fileMissing(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return true
	}
	return false
}

// BuildVideoLaunchEnv builds the shared env passed to replays/cameras child processes.
func BuildVideoLaunchEnv(versionDir string) []string {
	configDir := VideoConfigDir(versionDir)
	env := os.Environ()
	env = UpsertEnv(env, "VIDEO_CONFIGDIR", configDir)
	env = UpsertEnv(env, "VIDEO_CONTROLPANEL_DIR", GetControlPanelInstallDir())
	env = UpsertEnv(env, "VIDEO_LAUNCHER", GetLauncherVersionSemver())
	env = UpsertEnv(env, "OWLCMS_CONTROLPANEL", GetLauncherVersionSemver())

	// Export the shared FFmpeg path so child processes find it directly.
	if ffmpegPath := FindLocalFFmpeg(); ffmpegPath != "" {
		env = UpsertEnv(env, "VIDEO_FFMPEG_PATH", ffmpegPath)
		// For Linux shared builds, prepend the bundled lib/ to LD_LIBRARY_PATH.
		if GetGoos() == "linux" {
			libDir := filepath.Join(filepath.Dir(filepath.Dir(ffmpegPath)), "lib")
			if st, err := os.Stat(libDir); err == nil && st.IsDir() {
				if existing := os.Getenv("LD_LIBRARY_PATH"); existing != "" {
					env = UpsertEnv(env, "LD_LIBRARY_PATH", fmt.Sprintf("%s:%s", libDir, existing))
				} else {
					env = UpsertEnv(env, "LD_LIBRARY_PATH", libDir)
				}
			}
		}
	}

	parentEnvPath := filepath.Join(filepath.Dir(versionDir), "env.properties")
	releaseEnvPath := filepath.Join(versionDir, "env.properties")
	if props, err := MergeEnvironmentProperties(parentEnvPath, releaseEnvPath); err == nil {
		env = ApplyPropertiesToEnv(env, props, nil)
	}

	return env
}

// ShouldRunVideoExtract determines whether launchers should run --extractConfig preflight.
// app must be "replays" or "cameras".
// FFmpeg availability is handled separately by EnsureFFmpegPrerequisite.
// Checks for the shared ffmpeg.toml in video_config and per-instance config.toml
// in the version directory.
func ShouldRunVideoExtract(versionDir, app string) bool {
	sharedDir := VideoConfigDir(versionDir)
	// Shared encoder config
	if fileMissing(filepath.Join(sharedDir, "ffmpeg.toml")) {
		return true
	}
	// Per-instance config in the version directory
	return fileMissing(filepath.Join(versionDir, "config.toml"))
}

// RunVideoExtractBootstrap runs the app once with --extractConfig and shared launcher env.
// --configDir points to the version directory for per-instance configs;
// ffmpeg.toml is written to the shared VIDEO_CONFIGDIR set in the environment.
func RunVideoExtractBootstrap(exePath, versionDir string) error {
	cmd := exec.Command(exePath, "--extractConfig", "--configDir", versionDir)
	cmd.Dir = versionDir
	cmd.Env = BuildVideoLaunchEnv(versionDir)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("extract bootstrap failed: %w (%s)", err, string(output))
	}

	return nil
}

// ReadTopLevelTOMLValue reads a simple top-level TOML key without pulling in a full TOML parser.
func ReadTopLevelTOMLValue(filePath, key string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.TrimSpace(parts[0]) != key {
			continue
		}

		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"'`)
		return value, nil
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", nil
}
