package shared

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// CreateMenuButton creates a button that shows a popup menu when clicked
func CreateMenuButton(label string, menuItems []*fyne.MenuItem) *widget.Button {
	btn := widget.NewButton(label+" ▼", nil)
	btn.OnTapped = func() {
		menu := fyne.NewMenu("", menuItems...)
		app := fyne.CurrentApp()
		if app == nil || app.Driver() == nil {
			log.Printf("Cannot show %s menu: Fyne application driver is unavailable", label)
			return
		}

		driver := app.Driver()
		canvas := driver.CanvasForObject(btn)
		if canvas == nil {
			windows := driver.AllWindows()
			if len(windows) == 0 {
				log.Printf("Cannot show %s menu: no Fyne window is available", label)
				return
			}
			canvas = windows[0].Canvas()
		}

		pos := driver.AbsolutePositionForObject(btn)
		pos.Y += btn.Size().Height // Position below the button
		widget.ShowPopUpMenuAtPosition(menu, canvas, pos)
	}
	return btn
}
