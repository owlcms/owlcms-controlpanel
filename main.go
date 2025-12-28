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
	"owlcms-launcher/tracker"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var owlcmsInstallDir = getInstallDir()

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
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting OWLCMS Launcher")
	a := app.NewWithID("app.owlcms.owlcms-launcher")
	a.Settings().SetTheme(newMyTheme())
	w := a.NewWindow("OWLCMS Control Panel")
	w.Resize(fyne.NewSize(900, 430))

	// Create tab contents - owlcms.CreateTab handles its own initialization
	owlcmsTabContent := owlcms.CreateTab(w, a)
	trackerTabContent := tracker.CreateTab(w)
	firmataTabContent := firmata.CreateTab(w)

	mainContent := container.NewAppTabs(
		container.NewTabItem("OWLCMS", owlcmsTabContent),
		container.NewTabItem("Tracker", trackerTabContent),
		container.NewTabItem("Firmata", firmataTabContent),
	)

	w.SetContent(mainContent)

	// Setup menus
	setupMenus(w)

	// Set up signal handling
	setupSignalHandling()

	// Setup cleanup on exit
	setupCleanupOnExit(w)

	// Run the application
	w.ShowAndRun()
}

// setupMenus sets up the application menu bar
func setupMenus(w fyne.Window) {
	fileMenu := fyne.NewMenu("File")
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
			dialog.ShowInformation("About", "OWLCMS Launcher version "+owlcms.GetLauncherVersion(), w)
		}),
	)
	menu := fyne.NewMainMenu(fileMenu, helpMenu)
	w.SetMainMenu(menu)
}

// setupCleanupOnExit sets up cleanup when the window is closed
func setupCleanupOnExit(w fyne.Window) {
	w.SetCloseIntercept(func() {
		// Check if any program is running
		if owlcms.IsRunning() || tracker.IsRunning() || firmata.IsRunning() {
			confirmDialog := dialog.NewConfirm(
				"Confirm Exit",
				"A program is running. Closing the window will stop it and may affect users.",
				func(confirm bool) {
					if !confirm {
						log.Println("Closing launcher - stopping all running processes")
						owlcms.StopRunningProcess(w)
						tracker.StopRunningProcess(w)
						firmata.StopRunningProcess(w)
						w.Close()
					}
				},
				w,
			)
			confirmDialog.SetConfirmText("Cancel")
			confirmDialog.SetDismissText("Stop and Exit")
			confirmDialog.Show()
		} else {
			w.Close()
		}
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
