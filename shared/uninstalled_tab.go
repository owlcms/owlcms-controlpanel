package shared

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// ShowUninstalledTabContent updates the provided `versionContainer` to show an
// explanation (loaded from embedded assets if present), an optional
// "Use latest pre-release" checkbox, and an Install button.
// `installAction` receives the checkbox state (true = pre-release requested).
// `backgroundInit` is executed in a new goroutine so the caller can prepare
// downloads or other async initialization while the UI stays responsive.
func ShowUninstalledTabContent(versionContainer *fyne.Container, assetPath string, installAction func(usePrerelease bool), backgroundInit func()) {
	expl := GetAssetContent(assetPath)
	if expl == "" {
		expl = assetPath
	}

	explLabel := widget.NewRichTextFromMarkdown(expl)

	usePrerelease := false
	prereleaseCheck := widget.NewCheck("Use latest pre-release", func(checked bool) {
		usePrerelease = checked
	})

	installBtn := widget.NewButton("Install", func() {
		log.Printf("Install button pressed for asset %s (prerelease=%v)", assetPath, usePrerelease)
		if installAction == nil {
			log.Printf("Install action is nil for asset %s", assetPath)
			// Provide user feedback instead of doing nothing
			if w := fyne.CurrentApp().Driver().AllWindows(); len(w) > 0 {
				dialog.ShowInformation("Install Unavailable", "This tab cannot install automatically. Use the Downloads area or the Files menu to install a version.", w[0])
			}
			return
		}
		installAction(usePrerelease)
	})

	// Replace the container contents and refresh
	if versionContainer != nil {
		versionContainer.Objects = []fyne.CanvasObject{container.NewVBox(explLabel, prereleaseCheck, installBtn)}
		versionContainer.Refresh()
	}

	if backgroundInit != nil {
		go backgroundInit()
	}
}
