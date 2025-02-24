package dialog

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// NewDownloadDialog creates a consistent download dialog with a progress bar and cancel button
func NewDownloadDialog(title string, window fyne.Window, cancel chan bool) (*dialog.CustomDialog, *widget.ProgressBar, *widget.Label) {
	progressBar := widget.NewProgressBar()
	messageLabel := widget.NewLabel("Downloading...")

	cancelButton := widget.NewButton("Cancel", func() {
		close(cancel)
	})

	content := container.NewVBox(
		messageLabel,
		progressBar,
	)

	contentWithCancel := container.NewBorder(nil, container.NewPadded(cancelButton), nil, nil, content)

	d := dialog.NewCustom(
		title,
		"Please Wait...", // Set the dismiss button text to "Please Wait..."
		contentWithCancel,
		window)

	// Set a consistent width for the dialog
	d.Resize(fyne.NewSize(400, 200))

	return d, progressBar, messageLabel
}
