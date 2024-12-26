package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app" // Import for layout containers (VBox, HBox, etc.)
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func main() {
	a := app.NewWithID("app.owlcms.owlcms-launcher")
	w := a.NewWindow("owlcms launcher")

	// Create a label to display the click status
	clickLabel := widget.NewLabel("")

	// Create the button
	button := widget.NewButton("Click Me", func() {
		clickLabel.SetText("I was clicked!")
	})

	// Create a container to layout the label and button vertically
	content := container.NewVBox(
		button,
		clickLabel, // Add the label to the layout
	)

	w.SetContent(content) // Set the VBox as the window content

	w.Resize(fyne.NewSize(200, 100)) // Set a reasonable window size
	w.ShowAndRun()
}
