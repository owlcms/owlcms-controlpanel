package main

import (
	"fmt"
	"image/color"
	"io"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"controlpanel/cameras"
	"controlpanel/firmata"
	"controlpanel/owlcms"
	"controlpanel/owlcms/javacheck"
	"controlpanel/replays"
	"controlpanel/shared"
	"controlpanel/tracker"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var exitInProgress bool
var controlPanelLogPath string

type myTheme struct {
	fyne.Theme
}

func newMyTheme() *myTheme {
	return &myTheme{Theme: theme.LightTheme()}
}

func (m myTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNameBackground {
		return color.White
	}
	if name == theme.ColorNameForeground {
		return color.Black
	}
	if name == theme.ColorNameShadow {
		return color.NRGBA{R: 0, G: 0, B: 0, A: 40}
	}
	if name == theme.ColorNameSuccess {
		// Much darker green for Firmata buttons
		return color.RGBA{R: 15, G: 80, B: 15, A: 255}
	}
	if name == theme.ColorNameError {
		// Much darker red for Firmata buttons
		return color.RGBA{R: 100, G: 10, B: 10, A: 255}
	}
	return m.Theme.Color(name, variant)
}

func getInstanceIdentity() (string, string) {
	instance := strings.TrimSpace(os.Getenv("CONTROLPANEL_INSTANCE"))
	if instance == "" {
		return "app.owlcms.controlpanel", "OWLCMS Control Panel"
	}

	return "app.owlcms.controlpanel." + instance, fmt.Sprintf("OWLCMS Control Panel (%s)", instance)
}

func main() {
	cliOptions := parseCLIOptions(os.Args[1:])
	owlcmsFlag, trackerFlag := parseDaemonFlags(os.Args[1:])
	if cliOptions.help {
		printUsage()
		return
	}
	if err := applyCLIInstanceOptions(cliOptions); err != nil {
		fmt.Fprintf(os.Stderr, "instance setup: %v\n", err)
		os.Exit(1)
	}
	if owlcmsFlag != "" || trackerFlag != "" {
		var err error
		owlcmsFlag, trackerFlag, err = maybeApplyImplicitInstanceForHeadless(cliOptions, owlcmsFlag, trackerFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "instance setup: %v\n", err)
			os.Exit(1)
		}
	}
	if cliOptions.init {
		initializedInstance := strings.TrimSpace(os.Getenv("CONTROLPANEL_INSTANCE"))
		if initializedInstance == "" {
			initializedInstance = strings.TrimSpace(cliOptions.instanceArg)
		}
		fmt.Printf("Initialized instance %q\n", initializedInstance)
		fmt.Printf("control panel dir: %s\n", shared.GetControlPanelInstallDir())
		fmt.Printf("owlcms dir:        %s\n", shared.GetOwlcmsInstallDir())
		fmt.Printf("tracker dir:       %s\n", shared.GetTrackerInstallDir())
		fmt.Printf("runtime dir:       %s\n", shared.GetRuntimeDir())
		return
	}

	javacheck.InitJavaCheck(shared.GetOwlcmsInstallDir(), owlcms.GetTemurinVersion)

	// Set up logging to file (and stderr if available)
	controlPanelDir := shared.GetControlPanelInstallDir()
	_ = shared.EnsureDir0755(controlPanelDir) // ignore error, can't log yet
	controlPanelLogPath = filepath.Join(controlPanelDir, "control-panel.log")
	logFile, err := os.OpenFile(controlPanelLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// Fall back to stderr only if file open fails
		log.SetOutput(shared.NewLogPathShorteningWriter(os.Stderr))
	} else {
		// On Windows GUI apps without console, stderr may be invalid
		// Test if stderr is usable by checking if Stat succeeds
		stderrUsable := false
		if fi, err := os.Stderr.Stat(); err == nil && fi != nil {
			stderrUsable = true
		}
		if stderrUsable {
			multiWriter := io.MultiWriter(os.Stderr, logFile)
			log.SetOutput(shared.NewLogPathShorteningWriter(multiWriter))
		} else {
			log.SetOutput(shared.NewLogPathShorteningWriter(logFile))
		}
	}
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Llongfile)
	log.Printf("Starting OWLCMS Control Panel %s", shared.GetLauncherVersion())

	if owlcmsFlag != "" || trackerFlag != "" {
		owlcmsStop := strings.EqualFold(owlcmsFlag, "stop")
		trackerStop := strings.EqualFold(trackerFlag, "stop")
		if owlcmsStop || trackerStop {
			stopHeadlessDaemons(owlcmsStop, trackerStop)
			return
		}
		runHeadlessDaemons(owlcmsFlag, trackerFlag)
		return
	}

	appID, windowTitle := getInstanceIdentity()
	a := app.NewWithID(appID)
	a.Settings().SetTheme(newMyTheme())
	w := a.NewWindow(windowTitle)
	w.Resize(fyne.NewSize(950, 600))

	// Create tab contents - owlcms.CreateTab handles its own initialization
	owlcmsTabContent := owlcms.CreateTab(w, a)
	trackerTabContent := tracker.CreateTab(w)
	firmataTabContent := firmata.CreateTab(w)

	tabs := []*container.TabItem{
		container.NewTabItem("OWLCMS", owlcmsTabContent),
		container.NewTabItem("Tracker", trackerTabContent),
		container.NewTabItem("Arduino Devices", firmataTabContent),
	}

	if shared.GetGoos() != "darwin" {
		camerasTabContent := cameras.CreateTab(w)
		replaysTabContent := replays.CreateTab(w)
		tabs = append(tabs,
			container.NewTabItem("Cameras", camerasTabContent),
			container.NewTabItem("Replays", replaysTabContent),
		)
	}

	mainContent := container.NewAppTabs(tabs...)

	// Refresh tracker version list when its tab is selected (to update OWLCMS version warning)
	mainContent.OnSelected = func(tab *container.TabItem) {
		if tab.Text == "Tracker" {
			tracker.OnTabSelected()
		}
	}

	w.SetContent(mainContent)

	// Setup menus
	setupMenus(w)

	// Show installed modules popup
	// Query the actual install directories from each package
	owlp := owlcms.GetInstallDir()
	tp := tracker.GetInstallDir()
	fp := firmata.GetInstallDir()
	vp := cameras.GetInstallDir()

	mods := shared.DetectInstalledModules(owlp, tp, fp, vp)

	owlcmsCheck := widget.NewCheck("OWLCMS", nil)
	owlcmsCheck.SetChecked(mods.OWLCMS)
	trackerCheck := widget.NewCheck("Tracker", nil)
	trackerCheck.SetChecked(mods.Tracker)
	firmataCheck := widget.NewCheck("Firmata", nil)
	firmataCheck.SetChecked(mods.Firmata)
	videoCheck := widget.NewCheck("Video", nil)
	videoCheck.SetChecked(mods.Video)
	checks := container.NewVBox(owlcmsCheck, trackerCheck, firmataCheck, videoCheck)

	// The initial installed-modules popup is kept in code for future use
	// but is not shown by default. To re-enable, call `dialog.ShowCustom(...)`.
	_ = checks // keep variable referenced

	// Set up signal handling
	setupSignalHandling()

	// Setup cleanup on exit
	setupCleanupOnExit(w)

	// Run the application
	w.ShowAndRun()
}

func anyProgramRunning() bool {
	owlcmsRunning := owlcms.IsRunning()
	trackerRunning := tracker.IsRunning()
	firmataRunning := firmata.IsRunning()
	camerasRunning := cameras.IsRunning()
	replaysRunning := replays.IsRunning()

	log.Printf("anyProgramRunning: OWLCMS=%v, Tracker=%v, Firmata=%v, Cameras=%v, Replays=%v", owlcmsRunning, trackerRunning, firmataRunning, camerasRunning, replaysRunning)

	return owlcmsRunning || trackerRunning || firmataRunning || camerasRunning || replaysRunning
}

func anyDaemonRunning() bool {
	return owlcms.IsRunning() || tracker.IsRunning()
}

func anyLocalProgramRunning() bool {
	return firmata.IsRunning() || cameras.IsRunning() || replays.IsRunning()
}

func daemonModeEnabled() bool {
	return shared.GetGoos() == "linux" && shared.IsRunAsDaemonEnabled()
}

func stopLocalRunningProcesses(w fyne.Window) {
	firmata.StopRunningProcess(w)
	cameras.StopRunningProcess(w)
	replays.StopRunningProcess(w)
}

func stopLocalRunningProcessesForSignal() {
	if firmata.IsRunning() {
		log.Println("Signal cleanup: forcefully stopping Firmata process")
		firmata.HandleSignalCleanup()
	}
	if cameras.IsRunning() {
		log.Println("Signal cleanup: forcefully stopping Cameras process")
		cameras.HandleSignalCleanup()
	}
	if replays.IsRunning() {
		log.Println("Signal cleanup: forcefully stopping Replays process")
		replays.HandleSignalCleanup()
	}
}

func stopAllRunningProcesses(w fyne.Window) {
	owlcms.StopRunningProcess(w)
	tracker.StopRunningProcess(w)
	firmata.StopRunningProcess(w)
	cameras.StopRunningProcess(w)
	replays.StopRunningProcess(w)
}

func stopAllRunningProcessesForSignal() {
	// For signal handling, use forceful termination (no UI, no delays)
	if owlcms.IsRunning() {
		log.Println("Signal cleanup: forcefully stopping OWLCMS process")
		owlcms.HandleSignalCleanup()
	}
	if tracker.IsRunning() {
		log.Println("Signal cleanup: forcefully stopping Tracker process")
		tracker.HandleSignalCleanup()
	}
	if firmata.IsRunning() {
		log.Println("Signal cleanup: forcefully stopping Firmata process")
		firmata.HandleSignalCleanup()
	}
	if cameras.IsRunning() {
		log.Println("Signal cleanup: forcefully stopping Cameras process")
		cameras.HandleSignalCleanup()
	}
	if replays.IsRunning() {
		log.Println("Signal cleanup: forcefully stopping Replays process")
		replays.HandleSignalCleanup()
	}
}

func cleanupJavaVersions(w fyne.Window) {
	dialog.ShowConfirm(
		"Cleanup Java Versions",
		"This will:\n• Scan env.properties files to find the highest required Java version\n• Download and install that version if not present\n• Remove all older Java versions from the control panel\n• Remove all legacy Java installations from owlcms and firmata directories\n\nContinue?",
		func(confirm bool) {
			if !confirm {
				return
			}

			// Create a status label for progress updates
			statusLabel := widget.NewLabel("Scanning for Java versions...")
			progressDialog := dialog.NewCustom("Cleaning Up Java", "Close", statusLabel, w)
			progressDialog.Show()

			// Run cleanup in goroutine to allow UI updates
			go func() {
				// Pass legacy directories for bundled cleanup, but scanning happens in control panel
				removed, err := shared.CleanupObsoleteJavaVersions(owlcms.GetInstallDir(), firmata.GetInstallDir(), statusLabel, w)
				progressDialog.Hide()

				if err != nil {
					dialog.ShowError(fmt.Errorf("cleanup failed: %w", err), w)
					return
				}

				if len(removed) == 0 {
					dialog.ShowInformation("Cleanup Complete", "No obsolete Java versions found.", w)
				} else {
					message := "Cleanup results:\n\n"
					for _, v := range removed {
						message += "• " + v + "\n"
					}
					dialog.ShowInformation("Cleanup Complete", message, w)
				}
			}()
		},
		w,
	)
}

func cleanupNodeVersions(w fyne.Window) {
	dialog.ShowConfirm(
		"Cleanup Node.js Versions",
		"This will:\n• Keep only the latest Node.js version in the control panel\n• Remove all older Node.js versions from the control panel\n• Remove all bundled Node.js from tracker version directories\n\nContinue?",
		func(confirm bool) {
			if !confirm {
				return
			}

			// Create a status label for progress updates
			statusLabel := widget.NewLabel("Scanning for Node.js versions...")
			progressDialog := dialog.NewCustom("Cleaning Up Node.js", "Close", statusLabel, w)
			progressDialog.Show()

			// Run cleanup in goroutine to allow UI updates
			go func() {
				removed, err := shared.CleanupObsoleteNodeVersions(statusLabel, w)
				progressDialog.Hide()

				if err != nil {
					dialog.ShowError(fmt.Errorf("cleanup failed: %w", err), w)
					return
				}

				if len(removed) == 0 {
					dialog.ShowInformation("Cleanup Complete", "No obsolete Node.js versions found.", w)
				} else {
					message := "Cleanup results:\n\n"
					for _, v := range removed {
						message += "• " + v + "\n"
					}
					dialog.ShowInformation("Cleanup Complete", message, w)
				}
			}()
		},
		w,
	)
}

func requestExit(w fyne.Window) {
	if exitInProgress {
		return
	}

	daemonRunning := anyDaemonRunning()
	localRunning := anyLocalProgramRunning()
	daemonMode := daemonModeEnabled()
	message := "Exit the Control Panel?"
	confirmText := "Exit"
	if daemonMode && daemonRunning && localRunning {
		message = "OWLCMS and Tracker will continue running in the background. Local applications will be stopped."
		confirmText = "Exit"
	} else if daemonMode && daemonRunning {
		message = "OWLCMS and Tracker will continue running in the background."
		confirmText = "Exit"
	} else if anyProgramRunning() {
		message = "Running applications will be stopped. Exiting may affect users."
		confirmText = "Stop and Exit"
	} else if localRunning {
		message = "Running local applications will be stopped. Exiting may affect users."
		confirmText = "Stop and Exit"
	}

	confirmDialog := dialog.NewConfirm(
		"Confirm Exit",
		message,
		func(confirm bool) {
			if !confirm {
				return
			}

			exitInProgress = true
			if daemonMode {
				if localRunning {
					log.Println("Exiting launcher - stopping local running processes")
					stopLocalRunningProcesses(w)
				}
				if daemonRunning {
					log.Println("Exiting launcher - leaving OWLCMS and Tracker running in background")
				}
			} else if anyProgramRunning() {
				log.Println("Exiting launcher - stopping all running processes")
				stopAllRunningProcesses(w)
			}

			// Avoid re-triggering our close intercept (especially when Quit is from the menu).
			w.SetCloseIntercept(nil)
			w.Close()
		},
		w,
	)
	confirmDialog.SetConfirmText(confirmText)
	confirmDialog.SetDismissText("Cancel")
	confirmDialog.Show()
}

// setupMenus sets up the application menu bar
func setupMenus(w fyne.Window) {
	// Use "Quit" on all platforms - Fyne checks for this exact label
	// and won't add its own duplicate if it finds one.
	fileMenu := fyne.NewMenu(
		"File",
		fyne.NewMenuItem("Open Control Panel Installation Directory", func() {
			controlPanelDir := shared.GetControlPanelInstallDir()
			if err := shared.OpenFile(controlPanelDir); err != nil {
				dialog.ShowError(fmt.Errorf("failed to open directory: %w", err), w)
			}
		}),
		fyne.NewMenuItem("Cleanup Obsolete Java Versions", func() {
			cleanupJavaVersions(w)
		}),
		fyne.NewMenuItem("Cleanup Obsolete Node.js Versions", func() {
			cleanupNodeVersions(w)
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Refresh", func() {
			owlcms.RefreshVersionList(w)
			tracker.RefreshVersionList(w)
			firmata.RefreshVersionList(w)
			cameras.RefreshVersionList(w)
			replays.RefreshVersionList(w)
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", func() {
			requestExit(w)
		}),
	)
	helpMenu := fyne.NewMenu("Help",
		fyne.NewMenuItem("Documentation", func() {
			linkURL, _ := url.Parse("https://owlcms.github.io/owlcms4-prerelease/#/LocalControlPanel")
			link := widget.NewHyperlink("Control Panel Documentation", linkURL)
			dialog.ShowCustom("Documentation", "Close", link, w)
		}),
		fyne.NewMenuItem("Show Control Panel Log", func() {
			if controlPanelLogPath == "" {
				dialog.ShowError(fmt.Errorf("log file path not available"), w)
				return
			}
			if _, err := os.Stat(controlPanelLogPath); os.IsNotExist(err) {
				dialog.ShowError(fmt.Errorf("log file does not exist: %s", controlPanelLogPath), w)
				return
			}
			if err := shared.OpenFile(controlPanelLogPath); err != nil {
				dialog.ShowError(fmt.Errorf("failed to open log file: %w", err), w)
			}
		}),
		fyne.NewMenuItem("Check for Updates", func() {
			// Show confirmation dialog when checking from menu
			owlcms.CheckForUpdates(w, true)
		}),
		fyne.NewMenuItem("About", func() {
			dialog.ShowInformation("About", "OWLCMS Launcher version "+shared.GetLauncherVersion(), w)
		}),
	)
	menu := fyne.NewMainMenu(fileMenu, helpMenu)
	w.SetMainMenu(menu)
}

// setupCleanupOnExit sets up cleanup when the window is closed
func setupCleanupOnExit(w fyne.Window) {
	w.SetCloseIntercept(func() {
		if exitInProgress {
			// Allow close to proceed without re-confirming.
			w.SetCloseIntercept(nil)
			w.Close()
			return
		}

		// Check if any program is running
		requestExit(w)
	})
}

// setupSignalHandling sets up OS signal handlers for graceful shutdown
func setupSignalHandling() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	if shared.GetGoos() == "linux" {
		signal.Notify(sigChan, syscall.SIGHUP)
	}

	go func() {
		sig := <-sigChan
		log.Printf("Signal %v caught, cleaning up.\n", sig)

		// Start a hard exit timer in case cleanup hangs
		go func() {
			time.Sleep(5 * time.Second)
			log.Println("Cleanup timeout reached, forcing exit...")
			os.Exit(1)
		}()

		if shared.IsRunningUnderSystemd() {
			// Under systemd the Go process owns the child — stop everything
			// so systemctl stop actually terminates OWLCMS/Tracker.
			log.Println("Running under systemd — stopping all processes due to signal...")
			stopAllRunningProcessesForSignal()
			log.Println("All processes stopped.")
		} else if daemonModeEnabled() {
			if anyLocalProgramRunning() {
				log.Println("Stopping local running processes due to signal while daemon mode is enabled...")
				stopLocalRunningProcessesForSignal()
			}
			if anyDaemonRunning() {
				log.Println("Daemon mode enabled - leaving OWLCMS and Tracker running after terminal signal")
			}
		} else if anyProgramRunning() {
			log.Println("Stopping all running processes due to signal...")
			stopAllRunningProcessesForSignal()
			log.Println("All processes stopped.")
		}

		log.Println("Exiting Control Panel...")
		os.Exit(0)
	}()
}

// parseDaemonFlags scans args for --owlcms [version|latest|stop] and --tracker [version|latest|stop].
// When a switch is present without a value, it defaults to "latest".
// Returns empty strings when the flags are absent.
func parseDaemonFlags(args []string) (owlcmsVersion, trackerVersion string) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--owlcms":
			owlcmsVersion = "latest"
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				owlcmsVersion = args[i]
			}
		case "--tracker":
			trackerVersion = "latest"
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				trackerVersion = args[i]
			}
		}
	}
	return
}

// resolveVersion turns "latest" into the highest semver-installed directory name,
// or validates that the given version directory exists.  installDir is the module's
// install root and allVersions is the semver-descending list from GetAllInstalledVersions.
func resolveVersion(label, requested string, allVersions []string, installDir string) (string, error) {
	if len(allVersions) == 0 {
		return "", fmt.Errorf("no installed %s versions found", label)
	}

	if strings.EqualFold(requested, "latest") {
		v := allVersions[0] // already sorted by semver descending
		log.Printf("Resolved %s 'latest' to %s", label, v)
		return v, nil
	}

	// Check the requested version exists as a directory
	dir := filepath.Join(installDir, requested)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return "", fmt.Errorf("%s version %q is not installed (directory %s not found)", label, requested, dir)
	}
	return requested, nil
}

// runHeadlessDaemons launches OWLCMS and/or Tracker in daemon mode without any UI,
// then exits.  This is intended for boot-time systemd/init usage.
func runHeadlessDaemons(owlcmsVersion, trackerVersion string) {
	// Force daemon mode on so the spawned processes are detached.
	_ = shared.SetRunAsDaemonEnabled(true)

	var failed bool

	if owlcmsVersion != "" {
		version, err := resolveVersion("owlcms", owlcmsVersion, owlcms.GetAllInstalledVersions(), owlcms.GetInstallDir())
		if err != nil {
			log.Printf("ERROR: %v", err)
			fmt.Fprintf(os.Stderr, "owlcms: %v\n", err)
			failed = true
		} else {
			// Check if already running
			if meta, running := shared.CheckDaemonRunning(owlcms.RuntimeMetadataPath()); running {
				log.Printf("OWLCMS %s is already running (PID %d, port %s)", meta.Version, meta.PID, meta.Port)
				fmt.Printf("owlcms %s already running (PID %d, port %s)\n", meta.Version, meta.PID, meta.Port)
			} else {
				if err := owlcms.LaunchDaemon(version); err != nil {
					log.Printf("ERROR: failed to launch owlcms %s: %v", version, err)
					fmt.Fprintf(os.Stderr, "owlcms %s: %v\n", version, err)
					failed = true
				} else {
					fmt.Printf("owlcms %s started successfully\n", version)
				}
			}
		}
	}

	if trackerVersion != "" {
		version, err := resolveVersion("tracker", trackerVersion, tracker.GetAllInstalledVersions(), tracker.GetInstallDir())
		if err != nil {
			log.Printf("ERROR: %v", err)
			fmt.Fprintf(os.Stderr, "tracker: %v\n", err)
			failed = true
		} else {
			if meta, running := shared.CheckDaemonRunning(tracker.RuntimeMetadataPath()); running {
				log.Printf("Tracker %s is already running (PID %d, port %s)", meta.Version, meta.PID, meta.Port)
				fmt.Printf("tracker %s already running (PID %d, port %s)\n", meta.Version, meta.PID, meta.Port)
			} else {
				if err := tracker.LaunchDaemon(version); err != nil {
					log.Printf("ERROR: failed to launch tracker %s: %v", version, err)
					fmt.Fprintf(os.Stderr, "tracker %s: %v\n", version, err)
					failed = true
				} else {
					fmt.Printf("tracker %s started successfully\n", version)
				}
			}
		}
	}

	if failed {
		os.Exit(1)
	}
}

// stopHeadlessDaemons stops running OWLCMS and/or Tracker daemons from the command line.
func stopHeadlessDaemons(stopOwlcms, stopTracker bool) {
	var failed bool

	if stopOwlcms {
		failed = stopOneDaemon("owlcms", owlcms.RuntimeMetadataPath()) || failed
	}

	if stopTracker {
		failed = stopOneDaemon("tracker", tracker.RuntimeMetadataPath()) || failed
	}

	if failed {
		os.Exit(1)
	}
}

// stopOneDaemon stops a single daemon identified by its runtime metadata file.
// Returns true on failure.
func stopOneDaemon(label, metadataPath string) bool {
	meta, running := shared.CheckDaemonRunning(metadataPath)
	if !running {
		log.Printf("%s is not running", label)
		fmt.Printf("%s is not running\n", label)
		// Clean up stale metadata if present
		_ = shared.ClearRuntimeMetadata(metadataPath)
		return false
	}

	log.Printf("Stopping %s %s (PID %d, port %s)...", label, meta.Version, meta.PID, meta.Port)
	fmt.Printf("Stopping %s %s (PID %d)...\n", label, meta.Version, meta.PID)

	if err := shared.GracefullyStopPID(meta.PID); err != nil {
		log.Printf("ERROR: failed to stop %s (PID %d): %v", label, meta.PID, err)
		fmt.Fprintf(os.Stderr, "%s: failed to stop PID %d: %v\n", label, meta.PID, err)
		return true
	}

	_ = shared.ClearRuntimeMetadata(metadataPath)
	log.Printf("%s %s (PID %d) stopped", label, meta.Version, meta.PID)
	fmt.Printf("%s %s stopped\n", label, meta.Version)
	return false
}
