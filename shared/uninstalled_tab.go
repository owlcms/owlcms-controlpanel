package shared

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
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
		if installAction != nil {
			installAction()
		}
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
