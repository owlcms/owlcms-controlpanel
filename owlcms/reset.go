package owlcms

import (
	"owlcms-launcher/shared"

	"fyne.io/fyne/v2"
)

// resetToExplainMode clears download/update UI and shows the uninstalled explanation
// using the shared helper. Accepts the window so InstallDefault can show dialogs.
func resetToExplainMode(w fyne.Window) {
	if updateTitleContainer != nil {
		updateTitleContainer.Objects = []fyne.CanvasObject{}
		updateTitleContainer.Hide()
		updateTitleContainer.Refresh()
	}
	if downloadContainer != nil {
		downloadContainer.Objects = []fyne.CanvasObject{}
		downloadContainer.Hide()
		downloadContainer.Refresh()
	}
	if releaseNotesLink != nil {
		releaseNotesLink.Hide()
	}
	if installAvailableLink != nil {
		installAvailableLink.Hide()
	}
	if downloadButtonTitle != nil {
		downloadButtonTitle.Hide()
	}

	if statusLabel != nil {
		statusLabel.SetText("")
		statusLabel.Hide()
	}
	if stopContainer != nil {
		stopContainer.Hide()
	}
	if tailLogLink != nil {
		tailLogLink.Hide()
	}

	shared.ShowUninstalledTabContent(versionContainer, "asset/owlcms.md", func() { InstallDefault(w) }, nil)
	versionContainer.Show()
	versionContainer.Refresh()
}
