package shared

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	env = append(env, fmt.Sprintf("VIDEO_CONFIGDIR=%s", configDir))
	env = append(env, fmt.Sprintf("VIDEO_CONTROLPANEL_DIR=%s", GetControlPanelInstallDir()))
	env = append(env, fmt.Sprintf("VIDEO_LAUNCHER=%s", GetLauncherVersionSemver()))
	env = append(env, fmt.Sprintf("OWLCMS_CONTROLPANEL=%s", GetLauncherVersionSemver()))

	// Export the shared FFmpeg path so child processes find it directly.
	if ffmpegPath := FindLocalFFmpeg(); ffmpegPath != "" {
		env = append(env, fmt.Sprintf("VIDEO_FFMPEG_PATH=%s", ffmpegPath))
		// For Linux shared builds, prepend the bundled lib/ to LD_LIBRARY_PATH.
		if GetGoos() == "linux" {
			libDir := filepath.Join(filepath.Dir(filepath.Dir(ffmpegPath)), "lib")
			if st, err := os.Stat(libDir); err == nil && st.IsDir() {
				if existing := os.Getenv("LD_LIBRARY_PATH"); existing != "" {
					env = append(env, fmt.Sprintf("LD_LIBRARY_PATH=%s:%s", libDir, existing))
				} else {
					env = append(env, fmt.Sprintf("LD_LIBRARY_PATH=%s", libDir))
				}
			}
		}
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
