package main

import (
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"owlcms-launcher/javacheck"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

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
	return m.Theme.Color(name, variant)
}

func checkJava() error {
	return javacheck.CheckJava()
}

var (
	currentProcess   *exec.Cmd
	currentVersion   string // Add to track current version
	statusLabel      *widget.Label
	stopButton       *widget.Button
	versionContainer *fyne.Container
)

func launchOwlcms(version string, launchButton, stopButton *widget.Button, downloadGroup, versionContainer *fyne.Container) error {
	currentVersion = version // Store current version

	// Check if port 8080 is already in use
	if err := checkPort(); err == nil {
		return fmt.Errorf("OWLCMS is already running on port 8080")
	}

	statusLabel.SetText(fmt.Sprintf("Starting OWLCMS %s...", version))
	// Store current directory to restore it later
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	// Look for owlcms.jar in the version directory
	jarPath := filepath.Join(version, "owlcms.jar")
	if _, err := os.Stat(jarPath); os.IsNotExist(err) {
		launchButton.Show() // Show launch button again if start fails
		return fmt.Errorf("owlcms.jar not found in %s directory", version)
	}

	// Change to version directory
	if err := os.Chdir(version); err != nil {
		launchButton.Show() // Show launch button again if start fails
		return fmt.Errorf("changing to version directory: %w", err)
	}
	defer os.Chdir(originalDir)

	javaCmd := "java"
	localJava := filepath.Join(originalDir, "java17", "bin", "java")
	if runtime.GOOS == "windows" {
		localJava += ".exe"
	}
	if _, err := os.Stat(localJava); err == nil {
		javaCmd = localJava
	}

	cmd := exec.Command(javaCmd, "-jar", "owlcms.jar")
	if err := cmd.Start(); err != nil {
		statusLabel.SetText(fmt.Sprintf("Failed to start OWLCMS %s", version))
		launchButton.Show() // Show launch button again if start fails
		return fmt.Errorf("failed to start OWLCMS %s: %w", version, err)
	}

	fmt.Printf("Launching OWLCMS %s (PID: %d), waiting for port 8080...\n", version, cmd.Process.Pid)
	statusLabel.SetText(fmt.Sprintf("Starting OWLCMS %s (PID: %d), please wait...", version, cmd.Process.Pid))
	currentProcess = cmd
	stopButton.SetText(fmt.Sprintf("Stop OWLCMS %s", version))
	stopButton.Show()
	downloadGroup.Hide()
	versionContainer.Hide()

	// Monitor the process in background
	monitorChan := monitorProcess(cmd)

	// Wait for monitoring result in background
	go func() {
		if err := <-monitorChan; err != nil {
			fmt.Printf("OWLCMS process %d failed to start properly: %v\n", cmd.Process.Pid, err)
			statusLabel.SetText(fmt.Sprintf("OWLCMS process %d failed to start properly", cmd.Process.Pid))
			stopButton.Hide()
			launchButton.Show()
			currentProcess = nil
			downloadGroup.Show()
			versionContainer.Show()
			return
		}

		fmt.Printf("OWLCMS process %d is ready (port 8080 responding)\n", cmd.Process.Pid)
		statusLabel.SetText(fmt.Sprintf("OWLCMS running (PID: %d)", cmd.Process.Pid))

		// Process is stable, wait for it to end
		err := cmd.Wait()
		pid := cmd.Process.Pid

		if killedByUs {
			// If we killed it, just report normal termination
			fmt.Printf("OWLCMS %s (PID: %d) was stopped by user\n", version, pid)
			statusLabel.SetText(fmt.Sprintf("OWLCMS %s (PID: %d) was stopped by user", version, pid))
		} else if err != nil {
			// Only report error if it wasn't killed by us
			fmt.Printf("OWLCMS %s (PID: %d) terminated with error: %v\n", version, pid, err)
			statusLabel.SetText(fmt.Sprintf("OWLCMS %s (PID: %d) terminated with error", version, pid))
		} else {
			fmt.Printf("OWLCMS %s (PID: %d) exited normally\n", version, pid)
			statusLabel.SetText(fmt.Sprintf("OWLCMS %s (PID: %d) exited normally", version, pid))
		}

		currentProcess = nil
		killedByUs = false // Reset flag
		stopButton.Hide()
		launchButton.Show()
		downloadGroup.Show()
		versionContainer.Show()
	}()

	return nil
}

func main() {
	a := app.NewWithID("app.owlcms.owlcms-launcher")
	a.Settings().SetTheme(newMyTheme())
	w := a.NewWindow("OWLCMS Launcher")
	w.Resize(fyne.NewSize(600, 300)) // Larger initial window size

	progress := widget.NewProgressBarInfinite()
	loadingText := canvas.NewText("Fetching releases...", color.Black)
	loadingContainer := container.NewVBox(loadingText, progress)

	// Create stop button and status label
	stopButton = widget.NewButton("Stop", nil)
	statusLabel = widget.NewLabel("")
	statusLabel.Wrapping = fyne.TextWrapWord // Allow status messages to wrap

	// Create containers
	downloadGroup := container.NewVBox()
	versionContainer = container.NewVBox()
	stopContainer := container.NewVBox(stopButton, statusLabel)

	// Configure stop button behavior
	stopButton.OnTapped = func() {
		stopProcess(currentProcess, currentVersion, stopButton, downloadGroup, versionContainer, statusLabel, w)
	}
	stopButton.Hide()

	// Initialize version list
	versionList := createVersionList(w, stopButton, downloadGroup, versionContainer)

	// Create scroll container for version list with dynamic size
	versionScroll := container.NewVScroll(versionList)
	minHeight := 50 // minimum height
	rowHeight := 40 // approximate height per row
	numVersions := len(getAllInstalledVersions())
	if numVersions > 0 {
		// Set height based on number of versions, but cap at 4 rows
		height := minHeight + (rowHeight * min(numVersions, 4))
		versionScroll.SetMinSize(fyne.NewSize(400, float32(height)))
	} else {
		versionScroll.SetMinSize(fyne.NewSize(400, float32(minHeight)))
	}

	// Create more compact layout without padding
	versionContainer.Objects = []fyne.CanvasObject{
		widget.NewLabel("Installed Versions:"),
		versionScroll,
	}

	// Create release dropdown for downloads
	releaseDropdown := createReleaseDropdown(w, downloadGroup)

	mainContent := container.NewVBox(
		widget.NewLabelWithStyle("OWLCMS Launcher", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		versionContainer,
		stopContainer,
		container.NewHBox(
			downloadGroup,
			widget.NewLabel(""),
		),
	)

	w.SetContent(loadingContainer)
	w.Resize(fyne.NewSize(800, 600))

	go func() {
		releases, err := fetchReleases()
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		releaseDropdown.Options = releases // Set the available releases in dropdown
		// Show the main content with the populated version list
		w.SetContent(mainContent)
		w.Canvas().Refresh(mainContent)
	}()

	w.ShowAndRun()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
