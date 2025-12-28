package main

import (
	"owlcms-launcher/shared"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// createMenuButton wraps the shared version for backward compatibility
func createMenuButton(label string, menuItems []*fyne.MenuItem) *widget.Button {
	return shared.CreateMenuButton(label, menuItems)
}
