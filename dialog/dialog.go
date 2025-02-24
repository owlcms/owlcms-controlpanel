package dialog

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// NewDownloadDialog creates a consistent download dialog with a progress bar and cancel button
func NewDownloadDialog(title string, window fyne.Window, cancel chan bool) (dialog.Dialog, *widget.ProgressBar) {
	progressBar := widget.NewProgressBar()

	content := container.NewVBox(progressBar)
	progressDialog := dialog.NewCustom(title, "Cancel", content, window)
	progressDialog.SetOnClosed(func() {
		close(cancel)
	})

	// Set a consistent width for the dialog
	progressDialog.Resize(fyne.NewSize(400, 200))

	return progressDialog, progressBar
}
