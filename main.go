package main

import (
	"image/color"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"owlcms-launcher/firmata"
	"owlcms-launcher/owlcms"
	"owlcms-launcher/owlcms/downloadutils"
	"owlcms-launcher/owlcms/javacheck"
	"owlcms-launcher/shared"
	"owlcms-launcher/tracker"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var owlcmsInstallDir = getInstallDir()

var exitInProgress bool

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
	switch downloadutils.GetGoos() {
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
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Llongfile)
	log.SetOutput(shared.NewLogPathShorteningWriter(os.Stderr))
	log.Println("Starting OWLCMS Launcher")
	a := app.NewWithID("app.owlcms.owlcms-launcher")
	a.Settings().SetTheme(newMyTheme())
	w := a.NewWindow("OWLCMS Control Panel")
	w.Resize(fyne.NewSize(900, 600))

	// Create tab contents - owlcms.CreateTab handles its own initialization
	owlcmsTabContent := owlcms.CreateTab(w, a)
	trackerTabContent := tracker.CreateTab(w)
	firmataTabContent := firmata.CreateTab(w)

	mainContent := container.NewAppTabs(
		container.NewTabItem("OWLCMS", owlcmsTabContent),
		container.NewTabItem("Tracker", trackerTabContent),
		container.NewTabItem("Arduino Devices", firmataTabContent),
	)

	w.SetContent(mainContent)

	// Setup menus
	setupMenus(w)

	// Show installed modules popup
	// Query the actual install directories from each package
	owlp := owlcms.GetInstallDir()
	tp := tracker.GetInstallDir()
	fp := firmata.GetInstallDir()

	mods := shared.DetectInstalledModules(owlp, tp, fp)

	owlcmsCheck := widget.NewCheck("OWLCMS", nil)
	owlcmsCheck.SetChecked(mods.OWLCMS)
	trackerCheck := widget.NewCheck("Tracker", nil)
	trackerCheck.SetChecked(mods.Tracker)
	firmataCheck := widget.NewCheck("Firmata", nil)
	firmataCheck.SetChecked(mods.Firmata)
	checks := container.NewVBox(owlcmsCheck, trackerCheck, firmataCheck)

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
	return owlcms.IsRunning() || tracker.IsRunning() || firmata.IsRunning()
}

func stopAllRunningProcesses(w fyne.Window) {
	owlcms.StopRunningProcess(w)
	tracker.StopRunningProcess(w)
	firmata.StopRunningProcess(w)
}

func requestExit(w fyne.Window) {
	if exitInProgress {
		return
	}

	running := anyProgramRunning()
	message := "Exit the Control Panel?"
	confirmText := "Exit"
	if running {
		message = "A program is running. Exiting will stop it and may affect users."
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

		// Let owlcms package handle process cleanup
		owlcms.HandleSignalCleanup()

		log.Println("Exiting Control Panel...")

		// Force exit with a slight delay to allow logs to be written
		time.AfterFunc(100*time.Millisecond, func() {
			os.Exit(0)
		})
	}()
}
