package shared

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// CreateMenuButton creates a button that shows a popup menu when clicked
func CreateMenuButton(label string, menuItems []*fyne.MenuItem) *widget.Button {
	btn := widget.NewButton(label+" ▼", nil)
	btn.OnTapped = func() {
		menu := fyne.NewMenu("", menuItems...)
		canvas := fyne.CurrentApp().Driver().CanvasForObject(btn)
		pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(btn)
		pos.Y += btn.Size().Height // Position below the button
		widget.ShowPopUpMenuAtPosition(menu, canvas, pos)
	}
	return btn
}
