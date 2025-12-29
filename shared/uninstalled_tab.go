package shared

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// ShowUninstalledTabContent updates the provided `versionContainer` to show an
// explanation (loaded from embedded assets if present) and an Install button.
// `installAction` is called when the Install button is tapped. `backgroundInit`
// is executed in a new goroutine so the caller can prepare downloads or other
// async initialization while the UI stays responsive.
func ShowUninstalledTabContent(versionContainer *fyne.Container, assetPath string, installAction func(), backgroundInit func()) {
	expl := GetAssetContent(assetPath)
	if expl == "" {
		expl = assetPath
	}

	explLabel := widget.NewRichTextFromMarkdown(expl)
	installBtn := widget.NewButton("Install", func() {
		log.Printf("Install button pressed for asset %s", assetPath)
		if installAction == nil {
			log.Printf("Install action is nil for asset %s", assetPath)
			// Provide user feedback instead of doing nothing
			if w := fyne.CurrentApp().Driver().AllWindows(); len(w) > 0 {
				dialog.ShowInformation("Install Unavailable", "This tab cannot install automatically. Use the Downloads area or the Files menu to install a version.", w[0])
			}
			return
		}
		log.Printf("Calling installAction for asset %s", assetPath)
		installAction()
	})

	// Replace the container contents and refresh
	if versionContainer != nil {
		versionContainer.Objects = []fyne.CanvasObject{container.NewVBox(explLabel, installBtn)}
		versionContainer.Refresh()
	}

	if backgroundInit != nil {
		go backgroundInit()
	}
}
