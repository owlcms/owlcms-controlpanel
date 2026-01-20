package shared

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// CreateUpdateNotification creates a notification message box with a warning icon,
// message text, and install/release notes links
func CreateUpdateNotification(versionType, releaseVersion string, installLink, releaseNotesLink *widget.Hyperlink) fyne.CanvasObject {
	messageLabel := widget.NewLabel(fmt.Sprintf("A more recent %s version %s is available.", versionType, releaseVersion))
	messageLabel.TextStyle = fyne.TextStyle{Bold: true, Underline: true}
	warningIcon := canvas.NewText("⚠️", color.Black)
	warningIcon.TextSize = 20
	messageBox := container.NewHBox(warningIcon, messageLabel, installLink, releaseNotesLink)
	return messageBox
}
