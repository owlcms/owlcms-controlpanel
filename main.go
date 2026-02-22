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

var owlcmsInstallDir = getInstallDir()

var exitInProgress bool
var controlPanelLogPath string

func init() {
	javacheck.InitJavaCheck(owlcmsInstallDir, owlcms.GetTemurinVersion)
}

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

func getInstallDir() string {
	switch shared.GetGoos() {
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "owlcms")
	case "darwin":
		return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "owlcms")
	case "linux":
		return filepath.Join(os.Getenv("HOME"), ".local", "share", "owlcms")
	default:
		return "./owlcms"
	}
}

func main() {
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
	log.Println("Starting OWLCMS Control Panel")
	a := app.NewWithID("app.owlcms.controlpanel")
	a.Settings().SetTheme(newMyTheme())
	w := a.NewWindow("OWLCMS Control Panel")
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

	running := anyProgramRunning()
	message := "Exit the Control Panel?"
	confirmText := "Exit"
	if running {
		message = "Running applications will be stopped. Exiting may affect users."
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
			if running {
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

	go func() {
		sig := <-sigChan
		log.Printf("Signal %v caught, cleaning up.\n", sig)

		// Start a hard exit timer in case cleanup hangs
		go func() {
			time.Sleep(5 * time.Second)
			log.Println("Cleanup timeout reached, forcing exit...")
			os.Exit(1)
		}()

		// Check if any programs are running and stop them forcefully
		if anyProgramRunning() {
			log.Println("Stopping all running processes due to signal...")
			stopAllRunningProcessesForSignal()
			log.Println("All processes stopped.")
		}

		log.Println("Exiting Control Panel...")
		os.Exit(0)
	}()
}
