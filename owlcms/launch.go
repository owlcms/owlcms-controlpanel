package owlcms

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"controlpanel/owlcms/javacheck"
	"controlpanel/shared"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/Masterminds/semver/v3"
	"github.com/gofrs/flock"
	"github.com/magiconair/properties"
)

func configureTailLogLink(version, appDir string) {
	if tailLogLink == nil {
		return
	}

	logPath := filepath.Join(appDir, "logs", "owlcms.log")
	tailLogLink.SetText(fmt.Sprintf("Tail owlcms %s logs", version))
	tailLogLink.SetURL(nil)
	tailLogLink.OnTapped = func() {
		if err := shared.TailLogFile(logPath); err != nil {
			log.Printf("Failed to tail owlcms logs: %v", err)
			if statusLabel != nil {
				statusLabel.SetText("Failed to tail logs")
			}
		}
	}

	tailLogLink.Show()
}

var (
	controlPanelDir = shared.GetControlPanelInstallDir()
	lockFilePath    = filepath.Join(controlPanelDir, "java.lock")
	pidFilePath     = filepath.Join(controlPanelDir, "java.pid")
	javaPID         int
	lock            *flock.Flock
	currentProcess  *exec.Cmd
	currentVersion  string
	activeRuntime   *shared.RuntimeMetadata
)

func refreshRuntimePaths() {
	controlPanelDir = shared.GetControlPanelInstallDir()
	lockFilePath = filepath.Join(controlPanelDir, "java.lock")
	pidFilePath = filepath.Join(controlPanelDir, "java.pid")
}

func runtimeMetadataPath() string {
	return filepath.Join(controlPanelDir, "owlcms-run.json")
}

// RuntimeMetadataPath returns the path to the OWLCMS runtime metadata file.
func RuntimeMetadataPath() string {
	return runtimeMetadataPath()
}

func clearRuntimeState() {
	activeRuntime = nil
	if err := shared.ClearRuntimeMetadata(runtimeMetadataPath()); err != nil {
		log.Printf("Failed to clear OWLCMS runtime metadata: %v", err)
	}
}

func logRestartDecision(version string, pid int, waitErr error, retryCount, maxRestartRetries int) bool {
	restartable := shared.ShouldRestartProcess(waitErr)
	willRestart := !killedByUs && restartable && retryCount < maxRestartRetries

	details := []string{
		fmt.Sprintf("killedByUs=%t", killedByUs),
		fmt.Sprintf("retryCount=%d", retryCount),
		fmt.Sprintf("maxRestartRetries=%d", maxRestartRetries),
		fmt.Sprintf("waitErrNil=%t", waitErr == nil),
		fmt.Sprintf("restartable=%t", restartable),
		fmt.Sprintf("willRestart=%t", willRestart),
	}

	if waitErr == nil {
		details = append(details, "waitErr=<nil>")
	} else {
		details = append(details, fmt.Sprintf("waitErr=%v", waitErr))
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			details = append(details,
				"waitErrType=*exec.ExitError",
				fmt.Sprintf("exitCode=%d", exitErr.ExitCode()),
				fmt.Sprintf("sysType=%T", exitErr.Sys()),
			)
			if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				details = append(details,
					fmt.Sprintf("signaled=%t", ws.Signaled()),
					fmt.Sprintf("signal=%v", ws.Signal()),
					fmt.Sprintf("status=%d", ws.ExitStatus()),
				)
			}
		} else {
			details = append(details, fmt.Sprintf("waitErrType=%T", waitErr))
		}
	}

	log.Printf("OWLCMS restart decision for %s (PID: %d): %s", version, pid, strings.Join(details, ", "))
	return willRestart
}

type owlcmsLaunchParams struct {
	VersionDir string
	JarPath    string
	JavaPath   string
	TargetPort string
	Env        []string
}

const daemonMainClass = "app.owlcms.MainWrapper"
const embeddedMQTTEnv = "OWLCMS_ENABLEEMBEDDEDMQTT"

func shouldUseOwlcmsDaemonWrapper() bool {
	return shared.GetGoos() == "linux" && shared.IsRunAsDaemonEnabled() && !shared.IsRunningUnderSystemd()
}

func setEnvValue(env []string, key, value string) []string {
	return shared.UpsertEnv(env, key, value)
}

func removeEnvKey(env []string, key string) []string {
	prefix := key + "="
	filtered := env[:0]
	for _, entry := range env {
		if !strings.HasPrefix(entry, prefix) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func applyOwlcmsPropertiesToEnv(env []string, props *properties.Properties) []string {
	if props == nil {
		return env
	}

	skipKeys := map[string]struct{}{}
	if value, ok := props.Get(trackerConnectionEnv); ok && strings.TrimSpace(value) == "" {
		env = removeEnvKey(env, trackerConnectionEnv)
		skipKeys[trackerConnectionEnv] = struct{}{}
	}

	return shared.ApplyPropertiesToEnv(env, props, skipKeys)
}

// prepareOwlcmsLaunch resolves paths, verifies the jar exists, finds Java,
// loads the release environment, and builds the process env slice.
// Callers must ensure InitEnv() has been called before this.
func prepareOwlcmsLaunch(version string, embeddedMQTTOverride *bool) (*owlcmsLaunchParams, error) {
	if _, err := os.Stat(installDir); os.IsNotExist(err) {
		if err := shared.EnsureDir0755(installDir); err != nil {
			return nil, fmt.Errorf("creating owlcms directory: %w", err)
		}
	}

	versionDir := filepath.Join(installDir, version)
	jarPath := filepath.Join(versionDir, "owlcms.jar")
	if _, err := os.Stat(jarPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("owlcms.jar not found in %s", versionDir)
	}

	if err := EnsureReleaseEnvFromParent(version); err != nil {
		return nil, fmt.Errorf("failed to initialize env.properties: %w", err)
	}

	temurinVersion := GetTemurinVersionForRelease(version)
	localJava, err := javacheck.FindLocalJavaForVersion(temurinVersion)
	if err != nil {
		return nil, fmt.Errorf("java not found for %s: %w", temurinVersion, err)
	}

	mergedEnv, err := loadEnvironmentForReleaseProps(version)
	if err != nil {
		return nil, fmt.Errorf("failed to load environment: %w", err)
	}

	targetPort := GetPort()

	env := os.Environ()
	lv := shared.GetLauncherVersionSemver()
	env = shared.UpsertEnv(env, "OWLCMS_LAUNCHER", lv)
	env = shared.UpsertEnv(env, "OWLCMS_CONTROLPANEL", lv)
	for _, key := range mergedEnv.Keys() {
		value, _ := mergedEnv.Get(key)
		log.Printf("   %s=%s", key, value)
	}
	env = applyOwlcmsPropertiesToEnv(env, mergedEnv)
	if embeddedMQTTOverride != nil {
		value := "false"
		if *embeddedMQTTOverride {
			value = "true"
		}
		log.Printf("   %s=%s (command-line override)", embeddedMQTTEnv, value)
		env = setEnvValue(env, embeddedMQTTEnv, value)
	}

	return &owlcmsLaunchParams{
		VersionDir: versionDir,
		JarPath:    jarPath,
		JavaPath:   localJava,
		TargetPort: targetPort,
		Env:        env,
	}, nil
}

func buildOwlcmsCommand(params *owlcmsLaunchParams, useDaemonWrapper bool) *exec.Cmd {
	if useDaemonWrapper {
		return exec.Command(params.JavaPath, "-cp", params.JarPath, daemonMainClass)
	}
	return exec.Command(params.JavaPath, "-jar", filepath.Base(params.JarPath))
}

// recordOwlcmsStart writes the PID file and runtime metadata after a successful cmd.Start().
func recordOwlcmsStart(pid int, version, port string) *shared.RuntimeMetadata {
	if err := os.WriteFile(pidFilePath, []byte(fmt.Sprintf("%d\n", pid)), 0644); err != nil {
		log.Printf("Failed to write PID to PID file: %v\n", err)
	} else {
		log.Printf("Wrote PID %d to PID file %s\n", pid, pidFilePath)
	}

	SaveLastRunVersion(version)

	metadata, err := shared.WriteRuntimeMetadata(runtimeMetadataPath(), pid, version, port)
	if err != nil {
		log.Printf("Failed to write OWLCMS runtime metadata: %v", err)
		return nil
	}
	return metadata
}

// SaveLastRunVersion persists the version so that --owlcms previous can find it.
func SaveLastRunVersion(version string) {
	p := filepath.Join(installDir, "last-version.txt")
	if err := os.WriteFile(p, []byte(version), 0644); err != nil {
		log.Printf("Failed to save OWLCMS last-run version: %v", err)
	}
}

// GetLastRunVersion returns the previously launched OWLCMS version, or empty string.
func GetLastRunVersion() string {
	p := filepath.Join(installDir, "last-version.txt")
	data, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// LaunchDaemon starts OWLCMS headlessly (no UI) in daemon mode.
// Under systemd it stays in the foreground, waits on the process, and restarts
// on non-zero exit (same supervision as the interactive launcher).
// Otherwise it detaches the child and returns once the port is ready.
func LaunchDaemon(version string, enableEmbeddedMQTT bool) error {
	log.Printf("LaunchDaemon: starting OWLCMS %s headlessly (systemd=%v, INVOCATION_ID=%q)",
		version, shared.IsRunningUnderSystemd(), os.Getenv("INVOCATION_ID"))

	if err := InitEnv(); err != nil {
		return fmt.Errorf("failed to initialize environment: %w", err)
	}

	targetPort := GetPort()
	if shared.CheckPort(targetPort) == nil {
		log.Printf("LaunchDaemon: port %s is in use, attempting to free it...", targetPort)
		if err := shared.EnsurePortFree(targetPort); err != nil {
			return fmt.Errorf("port %s is in use and could not be freed: %w", targetPort, err)
		}
		if shared.CheckPort(targetPort) == nil {
			return fmt.Errorf("port %s is still in use after cleanup", targetPort)
		}
		log.Printf("LaunchDaemon: port %s successfully freed", targetPort)
	}

	params, err := prepareOwlcmsLaunch(version, &enableEmbeddedMQTT)
	if err != nil {
		return err
	}

	if shared.IsRunningUnderSystemd() {
		return launchDaemonForeground(version, params)
	}
	return launchDaemonDetached(version, params)
}

// launchDaemonForeground starts OWLCMS under systemd (or Docker).
// Restarts are handled by the external supervisor, so the Go process
// simply starts Java, waits for the port, blocks on cmd.Wait(), and
// returns a failure only for restartable exits. Deliberate external TERM/KILL
// stops are normalized to a clean return so the outer supervisor treats them
// as intentional stops.
func launchDaemonForeground(version string, params *owlcmsLaunchParams) error {
	cmd := buildOwlcmsCommand(params, false)
	shared.ConfigureNoConsoleWindow(cmd)
	cmd.Env = params.Env
	cmd.Dir = params.VersionDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("LaunchDaemon(systemd): command %v in %s", cmd.Args, params.VersionDir)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start OWLCMS %s: %w", version, err)
	}

	pid := cmd.Process.Pid
	activeRuntime = recordOwlcmsStart(pid, version, params.TargetPort)
	log.Printf("LaunchDaemon(systemd): OWLCMS %s (PID %d), waiting for port %s...", version, pid, params.TargetPort)

	// Wait for the port to come up before declaring success.
	deadline := time.Now().Add(60 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		if shared.CheckPort(params.TargetPort) == nil {
			ready = true
			break
		}
		if !shared.IsProcessRunning(pid) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if ready {
		log.Printf("LaunchDaemon(systemd): OWLCMS %s ready on port %s (PID %d)", version, params.TargetPort, pid)
		fmt.Printf("owlcms %s started successfully\n", version)
	}

	// Block until the process exits.
	waitErr := cmd.Wait()
	clearRuntimeState()

	if waitErr == nil {
		log.Printf("LaunchDaemon(systemd): OWLCMS %s exited normally (code 0)", version)
		return nil
	}

	restartable := shared.ShouldRestartProcess(waitErr)
	if !restartable {
		log.Printf("LaunchDaemon(systemd): OWLCMS %s (PID %d) exited intentionally: %v", version, pid, waitErr)
		return nil
	}

	log.Printf("LaunchDaemon(systemd): OWLCMS %s (PID %d) exited with restartable failure: %v", version, pid, waitErr)
	return waitErr
}

// launchDaemonDetached starts OWLCMS detached using MainWrapper and setsid.
// This is the original fire-and-forget behavior for interactive daemon mode
// ("Run as daemon" checkbox) where the Go process exits and the Java child
// survives.  Not used under systemd.
func launchDaemonDetached(version string, params *owlcmsLaunchParams) error {
	useDaemonWrapper := shouldUseOwlcmsDaemonWrapper()
	cmd := buildOwlcmsCommand(params, useDaemonWrapper)
	shared.ConfigureNoConsoleWindow(cmd)
	shared.ConfigureDetachedDaemonProcess(cmd, useDaemonWrapper)
	cmd.Env = params.Env
	cmd.Dir = params.VersionDir

	log.Printf("LaunchDaemon: command %v in %s", cmd.Args, params.VersionDir)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start OWLCMS %s: %w", version, err)
	}

	pid := cmd.Process.Pid
	activeRuntime = recordOwlcmsStart(pid, version, params.TargetPort)

	log.Printf("LaunchDaemon: OWLCMS %s (PID %d), waiting for port %s...", version, pid, params.TargetPort)
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if shared.CheckPort(params.TargetPort) == nil {
			log.Printf("LaunchDaemon: OWLCMS %s ready on port %s (PID %d)", version, params.TargetPort, pid)
			return nil
		}
		if !shared.IsProcessRunning(pid) {
			return fmt.Errorf("OWLCMS %s (PID %d) exited before becoming ready", version, pid)
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for OWLCMS %s on port %s", version, params.TargetPort)
}

var (
	startupLogMu       sync.Mutex
	startupLogStopCh   chan struct{}
	startupLogUpdating bool
	startupLogLastText string
	startupLogHeader   *widget.Label
)

func acquireJavaLock() (*flock.Flock, error) {
	pid, source, err := shared.ResolvePIDFromFileOrPort(pidFilePath, GetPort())
	if err != nil {
		return nil, err
	}
	if pid == 0 {
		return nil, nil
	}

	if source == "pid file" {
		log.Printf("Another instance of OWLCMS is already running with PID %d", pid)
		return nil, fmt.Errorf("another instance of OWLCMS is already running with PID %d", pid)
	}

	log.Printf("No running PID from PID file, stopping PID %d resolved from %s", pid, source)
	os.Remove(pidFilePath)
	os.Remove(lockFilePath)
	if err := shared.StopPIDFileOrPortProcess(pidFilePath, GetPort()); err != nil {
		log.Printf("Warning: failed to stop process after stale PID cleanup: %v", err)
	}

	return nil, nil
}

func releaseJavaLock() {
	log.Println("Released Java lock")
	if lock != nil {
		lock.Unlock()
		lock = nil
	}

	os.Remove(lockFilePath)
	os.Remove(pidFilePath)
}

func killLockingProcess() error {
	port := GetPort()
	if err := shared.StopPIDFileOrPortProcess(pidFilePath, port); err != nil {
		releaseJavaLock()
		return err
	}

	releaseJavaLock()
	log.Printf("Freed port %s\n", port)
	return nil
}

// launchOwlcms is the interactive (GUI) launcher.  It shares the same
// supervision contract as launchDaemonForeground — start, wait, restart
// on non-zero exit — but the wait/restart loop runs inside goroutines so
// the Fyne UI thread stays responsive.  See launchDaemonForeground for
// the synchronous headless equivalent used under systemd.
func launchOwlcms(version string, launchButton, stopBtn *widget.Button) error {
	currentVersion = version

	var err error
	lock, err = acquireJavaLock()
	if err != nil {
		goBackToMainScreen()
		return err
	}

	targetPort := GetPort()
	if shared.CheckPort(targetPort) == nil {
		log.Printf("Port %s is in use, attempting to free it...", targetPort)
		progressLabel := widget.NewLabel(fmt.Sprintf("Port %s is busy, stopping the process that is using it...", targetPort))
		progressDialog := dialog.NewCustom("Freeing Port", "", progressLabel, mainWindow)
		progressDialog.Show()

		go func() {
			err := shared.EnsurePortFree(targetPort)
			progressDialog.Hide()

			if err != nil {
				log.Printf("Cannot free port %s: %v", targetPort, err)
				dialog.ShowError(fmt.Errorf("cannot free port %s: %w", targetPort, err), mainWindow)
				goBackToMainScreen()
				return
			}
			if shared.CheckPort(targetPort) == nil {
				log.Printf("Port %s is still in use after cleanup", targetPort)
				dialog.ShowError(fmt.Errorf("port %s is still in use after cleanup", targetPort), mainWindow)
				goBackToMainScreen()
				return
			}

			log.Printf("Port %s successfully freed", targetPort)
			dialog.ShowInformation("Port Freed", fmt.Sprintf("Port %s has been freed. Launching OWLCMS %s...", targetPort, version), mainWindow)
			continueOwlcmsLaunch(version, targetPort, launchButton, stopBtn)
		}()
		return nil
	}

	continueOwlcmsLaunch(version, targetPort, launchButton, stopBtn)
	return nil
}

func continueOwlcmsLaunch(version, targetPort string, launchButton, stopBtn *widget.Button) {
	const maxRestartRetries = 3
	const restartDelay = 1 * time.Second

	statusLabel.SetText(fmt.Sprintf("Starting OWLCMS %s...", version))
	statusLabel.Refresh()
	statusLabel.Show()

	originalDir, err := os.Getwd()
	if err != nil {
		dialog.ShowError(fmt.Errorf("getting current directory: %w", err), mainWindow)
		goBackToMainScreen()
		return
	}

	params, err := prepareOwlcmsLaunch(version, nil)
	if err != nil {
		statusLabel.SetText(err.Error())
		launchButton.Show()
		goBackToMainScreen()
		releaseJavaLock()
		return
	}
	targetPort = params.TargetPort

	if err := os.Chdir(params.VersionDir); err != nil {
		launchButton.Show()
		dialog.ShowError(fmt.Errorf("changing to version directory: %w", err), mainWindow)
		return
	}
	defer os.Chdir(originalDir)

	var launchAttempt func(retryCount int)
	launchAttempt = func(retryCount int) {
		useDaemonWrapper := shouldUseOwlcmsDaemonWrapper()
		cmd := buildOwlcmsCommand(params, useDaemonWrapper)
		shared.ConfigureNoConsoleWindow(cmd)
		shared.ConfigureDetachedDaemonProcess(cmd, useDaemonWrapper)
		cmd.Env = params.Env
		cmd.Dir = params.VersionDir

		// Remove startup.log if it exists to ensure fresh log output
		// (Only versions >= 64.0.0-rc08 generate this file.)
		if owlcmsSupportsStartupLog(version) {
			startupLogPath := filepath.Join(params.VersionDir, "logs", "startup.log")
			if err := os.Remove(startupLogPath); err != nil && !os.IsNotExist(err) {
				log.Printf("Warning: Failed to remove old startup.log: %v", err)
			}
		}

		log.Printf("Starting OWLCMS %s with command: %v\n", version, cmd.Args)
		if err := cmd.Start(); err != nil {
			statusLabel.SetText(fmt.Sprintf("Failed to start OWLCMS %s", version))
			releaseJavaLock()
			launchButton.Show()
			goBackToMainScreen()
			log.Printf("Failed to start OWLCMS %s: %v\n", version, err)
			return
		}

		javaPID = cmd.Process.Pid
		activeRuntime = recordOwlcmsStart(javaPID, version, targetPort)

		log.Printf("Launching OWLCMS %s (PID: %d), waiting for port %s...\n", version, javaPID, targetPort)
		statusLabel.SetText(fmt.Sprintf("Starting OWLCMS %s (PID: %d), waiting for port %s.\nFull startup can take up to 30 seconds.", version, javaPID, targetPort))
		currentProcess = cmd
		stopBtn.SetText(fmt.Sprintf("Stop OWLCMS %s", version))
		stopBtn.Show()
		stopContainer.Show()
		downloadContainer.Hide()
		versionContainer.Hide()
		setOwlcmsTabModeRunning()

		appDir := filepath.Join(installDir, version)
		appDirLink.SetText(fmt.Sprintf("Open OWLCMS %s directory", version))
		appDirLink.SetURL(nil)
		appDirLink.OnTapped = func() {
			shared.OpenFileExplorer(appDir)
		}
		appDirLink.Show()
		configureTailLogLink(version, appDir)

		// Start monitoring for startup.log (when supported by the OWLCMS version)
		go monitorStartupLog(appDir, version)

		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		monitorChan := monitorProcess(done, targetPort)

		go func(retryCount int, pid int) {
			if err := <-monitorChan; err != nil {
				log.Printf("OWLCMS process %d failed to start properly: %v\n", pid, err)
				statusLabel.SetText(fmt.Sprintf("OWLCMS process %d failed to start properly", pid))
				stopBtn.Hide()
				stopContainer.Hide()
				launchButton.Show()
				currentProcess = nil
				clearRuntimeState()
				setOwlcmsTabMode(mainWindow)

				releaseJavaLock()
				return
			}

			log.Printf("OWLCMS process %d is ready (port %s responding)\n", pid, targetPort)
			statusLabel.SetText(fmt.Sprintf("OWLCMS running (PID: %d) on port %s", pid, targetPort))
			url := fmt.Sprintf("http://localhost:%s", targetPort)
			urlLink.SetURLFromString(url)
			urlLink.SetText("Open OWLCMS in a browser")
			urlLink.Show()

			// Close the startup log area now that OWLCMS is ready
			hideStartupLogArea()

			appDir := filepath.Join(installDir, version)
			appDirLink.SetText(fmt.Sprintf("Open OWLCMS %s directory", version))
			appDirLink.SetURL(nil)
			appDirLink.OnTapped = func() {
				shared.OpenFileExplorer(appDir)
			}
			appDirLink.Show()
			configureTailLogLink(version, appDir)

			if useDaemonWrapper {
				log.Printf("OWLCMS %s detached daemon is ready; reattaching UI to runtime metadata", version)
				currentProcess = nil
				if !reconnectOwlcmsRuntime() {
					if activeRuntime == nil {
						activeRuntime = &shared.RuntimeMetadata{
							PID:     pid,
							Version: version,
							Port:    targetPort,
						}
					}
					restoreOwlcmsRunningUI(version, targetPort, activeRuntime.PID)
				}
				return
			}

			err := <-done

			// Restart decision uses the same rules as Docker/systemd:
			//   exit 0                   → don't restart (clean shutdown)
			//   exit non-zero (e.g. 1)   → restart (database import, or unexpected error)
			//   SIGTERM / SIGINT          → don't restart (intentional stop by user)
			//   abnormal signal (SIGSEGV) → restart (JVM native crash)
			if logRestartDecision(version, pid, err, retryCount, maxRestartRetries) {
				attemptNum := retryCount + 1
				log.Printf("OWLCMS %s (PID: %d) exited unexpectedly (%v); restarting in %s (attempt %d/%d)\n", version, pid, err, restartDelay, attemptNum, maxRestartRetries)
				setOwlcmsTabModeRunning()
				currentProcess = nil
				stopBtn.Hide()
				stopContainer.Hide()
				launchButton.Hide()
				urlLink.Hide()
				appDirLink.Hide()
				if tailLogLink != nil {
					tailLogLink.Hide()
				}
				showStartupLogArea("Restarting OWLCMS")
				setStartupLogText("")
				time.Sleep(restartDelay)
				launchAttempt(retryCount + 1)
				return
			}

			if killedByUs {
				log.Printf("OWLCMS %s (PID: %d) was stopped by user\n", version, pid)
				statusLabel.SetText(fmt.Sprintf("OWLCMS %s (PID: %d) was stopped by user", version, pid))
			} else if err != nil {
				log.Printf("OWLCMS %s (PID: %d) terminated with error: %v\n", version, pid, err)
				statusLabel.SetText(fmt.Sprintf("OWLCMS %s (PID: %d) terminated with error", version, pid))
			} else {
				log.Printf("OWLCMS %s (PID: %d) exited normally\n", version, pid)
				statusLabel.SetText(fmt.Sprintf("OWLCMS %s (PID: %d) exited normally", version, pid))
			}

			currentProcess = nil
			killedByUs = false
			clearRuntimeState()
			stopBtn.Hide()
			stopContainer.Hide()
			launchButton.Show()
			setOwlcmsTabMode(mainWindow)

			urlLink.Hide()
			appDirLink.Hide()
			if tailLogLink != nil {
				tailLogLink.Hide()
			}
			hideStartupLogArea()
			releaseJavaLock()
		}(retryCount, javaPID)
	}

	launchAttempt(0)
}

// showStartupLogArea creates and shows the startup log text area
func showStartupLogArea(headerText string) {
	// Reset/initialize stop channel for this run
	startupLogMu.Lock()
	if startupLogStopCh != nil {
		select {
		case <-startupLogStopCh:
			// already closed
		default:
			close(startupLogStopCh)
		}
	}
	startupLogStopCh = make(chan struct{})
	stopCh := startupLogStopCh
	startupLogMu.Unlock()

	if startupLogText == nil {
		startupLogText = widget.NewMultiLineEntry()
		startupLogText.SetPlaceHolder("Waiting for startup log...")
		startupLogText.Wrapping = fyne.TextWrapWord
		startupLogText.OnChanged = func(s string) {
			// Keep it non-editable while remaining visually enabled.
			if startupLogUpdating {
				return
			}
			if s != startupLogLastText {
				startupLogUpdating = true
				startupLogText.SetText(startupLogLastText)
				startupLogUpdating = false
			}
		}
	}

	if startupLogContainer == nil {
		startupLogHeader = widget.NewLabel("Startup Progress:")

		// Use a Border layout so the text area expands to fill remaining space.
		// VBox would size children to MinSize and leave unused space below.
		header := container.NewVBox(
			widget.NewSeparator(),
			startupLogHeader,
		)
		scroller := container.NewScroll(startupLogText)
		startupLogContainer = container.NewBorder(header, nil, nil, nil, scroller)
	}

	if startupLogHeader != nil {
		if headerText == "" {
			headerText = "Startup Progress"
		}
		startupLogHeader.SetText(headerText)
		startupLogHeader.Refresh()
	}

	// Show within the running layout center so it can expand.
	if startupLogHost != nil {
		log.Printf("showStartupLogArea: Showing startup log container")
		startupLogHost.Objects = []fyne.CanvasObject{startupLogContainer}
		startupLogHost.Show()
		startupLogHost.Refresh()
	} else {
		log.Printf("showStartupLogArea: WARNING - startupLogHost is nil!")
	}

	// Auto-close after 60 seconds unless OWLCMS becomes ready first.
	go func(stopCh chan struct{}) {
		select {
		case <-time.After(60 * time.Second):
			hideStartupLogArea()
		case <-stopCh:
			return
		}
	}(stopCh)
}

// appendStartupLogText adds text to the startup log display
func appendStartupLogText(text string) {
	if startupLogText != nil {
		startupLogUpdating = true
		currentText := startupLogText.Text
		startupLogLastText = currentText + text
		startupLogText.SetText(startupLogLastText)
		// Move cursor to end so the latest text is visible
		startupLogText.CursorRow = len(strings.Split(startupLogLastText, "\n")) - 1
		startupLogUpdating = false
	}
}

// setStartupLogText sets the complete text in the startup log display
func setStartupLogText(text string) {
	if startupLogText != nil {
		startupLogUpdating = true
		startupLogLastText = text
		startupLogText.SetText(text)
		// Move cursor to end so the latest text is visible
		startupLogText.CursorRow = len(strings.Split(startupLogLastText, "\n")) - 1
		startupLogUpdating = false
	}
}

// hideStartupLogArea removes and hides the startup log display
func hideStartupLogArea() {
	startupLogMu.Lock()
	if startupLogStopCh != nil {
		select {
		case <-startupLogStopCh:
			// already closed
		default:
			close(startupLogStopCh)
		}
		startupLogStopCh = nil
	}
	startupLogMu.Unlock()

	if startupLogHost != nil {
		startupLogHost.Objects = nil
		startupLogHost.Hide()
		startupLogHost.Refresh()
	}
}

func owlcmsSupportsStartupLog(version string) bool {
	// Brutal normalization: ensure "-SNAPSHOT" prerelease compares correctly.
	// (ASCII sorting would treat 'S' differently; lowercase makes snapshot sort after rc.)
	normalized := strings.ReplaceAll(version, "-SNAPSHOT", "-snapshot")
	v, err := semver.NewVersion(normalized)
	if err != nil {
		log.Printf("Invalid OWLCMS version for semver comparison (%q): %v", version, err)
		return false
	}

	min, err := semver.NewVersion("64.0.0-rc08")
	if err != nil {
		// This should never happen; if it does, fail safe by disabling startup log.
		log.Printf("Internal error parsing minimum startup.log version: %v", err)
		return false
	}

	return !v.LessThan(min)
}

func monitorStartupLog(appDir, version string) {
	if !owlcmsSupportsStartupLog(version) {
		// Older OWLCMS versions don't generate logs/startup.log; don't poll for it.
		// Keep UI consistent: show an informational startup area while we wait for port readiness.
		showStartupLogArea("Startup Progress")
		setStartupLogText(fmt.Sprintf(
			"OWLCMS %s does not generate logs/startup.log.\nWaiting for OWLCMS to respond on port %s...\n",
			version,
			GetPort(),
		))
		return
	}

	startupLogPath := filepath.Join(appDir, "logs", "startup.log")
	log.Printf("Monitoring for startup.log at: %s\n", startupLogPath)

	// Show the startup log area immediately
	showStartupLogArea("Startup Progress")
	setStartupLogText("Waiting for startup.log...\n")

	// For testing: simulate dummy data
	// Toggle this to true only when doing UI-only testing.
	const startupLogTestMode = false
	if startupLogTestMode {
		log.Println("Testing mode: generating dummy startup log data")
		setStartupLogText("=== OWLCMS Test Startup Log ===\n")

		// Generate dummy data for up to 60 seconds, every 5 seconds,
		// or stop early if the startup log area is closed.
		startupLogMu.Lock()
		stopCh := startupLogStopCh
		startupLogMu.Unlock()
		go func() {
			for i := 0; i < 12; i++ {
				select {
				case <-stopCh:
					return
				default:
				}
				appendStartupLogText(fmt.Sprintf("[%02d:00] Initializing component %d...\n", i*5, i+1))
				select {
				case <-stopCh:
					return
				case <-time.After(5 * time.Second):
				}
			}
			appendStartupLogText("\n=== Startup dummy output complete (waiting for close/timeout) ===\n")
		}()
		return
	}

	// Real file monitoring
	// Check every 500ms for up to 60 seconds
	for i := 0; i < 120; i++ {
		if _, err := os.Stat(startupLogPath); err == nil {
			log.Printf("Found startup.log, starting to tail it\n")
			// Tail the file for 60 seconds
			go tailStartupLog(startupLogPath)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("startup.log not found after monitoring period\n")
	setStartupLogText("startup.log not found after monitoring period\n")
}

func tailStartupLog(logPath string) {
	log.Printf("Tailing startup log: %s\n", logPath)
	startupLogMu.Lock()
	stopCh := startupLogStopCh
	startupLogMu.Unlock()

	file, err := os.Open(logPath)
	if err != nil {
		log.Printf("Failed to open startup.log: %v\n", err)
		return
	}
	defer file.Close()

	// Read initial content
	content, err := os.ReadFile(logPath)
	if err == nil {
		setStartupLogText(string(content))
	}

	// Monitor for changes for 60 seconds (or until OWLCMS becomes ready)
	startTime := time.Now()
	lastSize := int64(0)
	if stat, err := file.Stat(); err == nil {
		lastSize = stat.Size()
	}

	foundReady := false
	for {
		select {
		case <-stopCh:
			return
		default:
		}

		if time.Since(startTime) > 60*time.Second {
			log.Println("Stopping startup log tail after 60 seconds")
			break
		}

		// Check for new content
		if stat, err := os.Stat(logPath); err == nil {
			if stat.Size() > lastSize {
				// Read the entire file again
				content, err := os.ReadFile(logPath)
				if err == nil {
					contentStr := string(content)
					setStartupLogText(contentStr)
					lastSize = stat.Size()

					// Check if OWLCMS is ready
					if !foundReady && (strings.Contains(contentStr, "OWLCMS Ready.") || strings.Contains(contentStr, "owlcms Ready.")) {
						foundReady = true
						log.Println("Found 'OWLCMS Ready.' in startup log")
					}
				}
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	// If we didn't find "OWLCMS Ready." after timeout, show error dialog
	if !foundReady {
		logsDir := filepath.Dir(logPath)
		go func() {
			dialog.ShowCustomConfirm("OWLCMS Startup Issue", "Open Logs Folder", "Close",
				widget.NewLabel("OWLCMS did not report ready status within 60 seconds.\nPlease check the logs and send them to owlcms@jflamy.dev if the issue persists."),
				func(ok bool) {
					if ok {
						shared.OpenFileExplorer(logsDir)
					}
				}, mainWindow)
		}()
		// Keep the log display visible so user can see what went wrong
	} else {
		// Close the startup log display only if startup was successful
		hideStartupLogArea()
	}
}
