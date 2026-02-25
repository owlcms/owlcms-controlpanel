package shared

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const videoConfigSubdir = "video_config"

func VideoConfigDir(versionDir string) string {
	_ = versionDir
	return filepath.Join(GetControlPanelInstallDir(), videoConfigSubdir)
}

func isDirEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return true
	}
	return len(entries) == 0
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
	return env
}

// ShouldRunVideoExtract determines whether launchers should run --extractConfig preflight.
// app must be "replays" or "cameras".
func ShouldRunVideoExtract(versionDir, app string) bool {
	configDir := VideoConfigDir(versionDir)
	switch app {
	case "replays":
		if fileMissing(filepath.Join(configDir, "config.toml")) || fileMissing(filepath.Join(configDir, "multicast.toml")) {
			return true
		}
	case "cameras":
		if fileMissing(filepath.Join(configDir, "cameras.toml")) {
			return true
		}
	}

	ffmpegDir := filepath.Join(VideoConfigDir(versionDir), "ffmpeg")
	return isDirEmpty(ffmpegDir)
}

// RunVideoExtractBootstrap runs the app once with --extractConfig and shared launcher env.
func RunVideoExtractBootstrap(exePath, versionDir string) error {
	cmd := exec.Command(exePath, "--extractConfig", "--configDir", VideoConfigDir(versionDir))
	cmd.Dir = versionDir
	cmd.Env = BuildVideoLaunchEnv(versionDir)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("extract bootstrap failed: %w (%s)", err, string(output))
	}

	return nil
}
