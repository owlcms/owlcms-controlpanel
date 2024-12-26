package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"net/http"

	"owlcms-launcher/downloadUtils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type Release struct {
	Name string `json:"name"`
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
	return m.Theme.Color(name, variant)
}

func fetchReleases() ([]string, error) {
	url := "https://api.github.com/repos/owlcms/owlcms4-prerelease/releases"
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var releases []Release
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, fmt.Errorf("invalid response format: %w", err)
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}

	releaseNames := make([]string, 0, 10)
	for i, release := range releases {
		if i >= 10 {
			break
		}
		releaseNames = append(releaseNames, release.Name)
	}

	return releaseNames, nil
}

func main() {
	a := app.NewWithID("app.owlcms.owlcms-launcher")
	a.Settings().SetTheme(newMyTheme())
	w := a.NewWindow("OWLCMS Launcher")
	w.Resize(fyne.NewSize(600, 300)) // Larger initial window size

	progress := widget.NewProgressBarInfinite()
	loadingText := canvas.NewText("Fetching releases...", color.Black)
	loadingContainer := container.NewVBox(loadingText, progress)

	releaseLabel := widget.NewLabel("Select OWLCMS Release:")
	releaseDropdown := widget.NewSelect([]string{}, func(selected string) {
		urlPrefix := "https://github.com/owlcms/owlcms4-prerelease/releases/download"
		fileName := fmt.Sprintf("owlcms_%s.zip", selected)
		zipURL := fmt.Sprintf("%s/%s/%s", urlPrefix, selected, fileName)
		zipPath := fileName
		extractPath := selected // Use the release version as subdirectory

		dialog.ShowConfirm("Confirm Download",
			fmt.Sprintf("Do you want to download and install OWLCMS version %s?", selected),
			func(ok bool) {
				if !ok {
					return
				}

				// Show progress dialog
				progressDialog := dialog.NewCustom(
					"Installing OWLCMS",
					"Please wait...",
					widget.NewTextGridFromString("Downloading and extracting files..."),
					w)
				progressDialog.Show()

				// Download the ZIP file using downloadUtils
				err := downloadUtils.DownloadZip(zipURL, zipPath)
				if err != nil {
					progressDialog.Hide()
					dialog.ShowError(fmt.Errorf("download failed: %w", err), w)
					return
				}

				// Extract the ZIP file to version-specific subdirectory
				err = downloadUtils.ExtractZip(zipPath, extractPath)
				if err != nil {
					progressDialog.Hide()
					dialog.ShowError(fmt.Errorf("extraction failed: %w", err), w)
					return
				}

				// Hide progress dialog
				progressDialog.Hide()

				// Show success panel with installation details
				message := fmt.Sprintf(
					"Successfully installed OWLCMS version %s\n\n"+
						"Location: %s\n\n"+
						"The program files have been extracted to the above directory.",
					selected, extractPath)

				dialog.ShowInformation("Installation Complete", message, w)
			},
			w)
	})
	releaseDropdown.PlaceHolder = "Choose a release version"

	mainContent := container.NewVBox(
		widget.NewLabel("OWLCMS Launcher"),
		releaseLabel,
		releaseDropdown,
	)

	w.SetContent(loadingContainer)
	w.Resize(fyne.NewSize(800, 600))

	go func() {
		releases, err := fetchReleases()
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		releaseDropdown.Options = releases
		w.SetContent(mainContent)
		w.Canvas().Refresh(mainContent)
	}()

	w.ShowAndRun()
}
