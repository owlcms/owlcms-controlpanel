package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/Masterminds/semver/v3"
)

var versionList *widget.List

func getAllInstalledVersions() []string {
	owlcmsDir := owlcmsInstallDir
	entries, err := os.ReadDir(owlcmsDir)
	if err != nil {
		return nil
	}

	versionPattern := regexp.MustCompile(`^\d+\.\d+\.\d+(?:-(?:rc|alpha|beta)(?:\d+)?)?$`)
	var versions []*semver.Version
	for _, entry := range entries {
		if entry.IsDir() && versionPattern.MatchString(entry.Name()) {
			v, err := semver.NewVersion(entry.Name())
			if err == nil {
				versions = append(versions, v)
			}
		}
	}

	sort.Sort(sort.Reverse(semver.Collection(versions)))

	var versionStrings []string
	for _, v := range versions {
		versionStrings = append(versionStrings, v.String())
	}

	return versionStrings
}

func findLatestInstalled() string {
	owlcmsDir := owlcmsInstallDir
	entries, err := os.ReadDir(owlcmsDir)
	if err != nil {
		return ""
	}

	versionPattern := regexp.MustCompile(`^\d+\.\d+\.\d+(?:-(?:rc|alpha|beta)(?:\d+)?)?$`)
	var versions []*semver.Version
	for _, entry := range entries {
		if entry.IsDir() && versionPattern.MatchString(entry.Name()) {
			v, err := semver.NewVersion(entry.Name())
			if err == nil {
				versions = append(versions, v)
			}
		}
	}

	if len(versions) == 0 {
		return ""
	}

	sort.Sort(sort.Reverse(semver.Collection(versions)))
	return versions[0].String()
}

func createVersionList(w fyne.Window, stopButton *widget.Button, downloadGroup, versionContainer *fyne.Container) *widget.List {
	versions := getAllInstalledVersions()

	versionList = widget.NewList(
		func() int { return len(versions) },
		func() fyne.CanvasObject {
			label := widget.NewLabel("Template")
			launchButton := widget.NewButton("Launch", nil)
			filesButton := widget.NewButton("Files", nil)
			removeButton := widget.NewButton("Remove", nil)
			launchButton.Resize(fyne.NewSize(80, 25))
			removeButton.Resize(fyne.NewSize(80, 25))
			filesButton.Resize(fyne.NewSize(80, 25))
			launchButton.Importance = widget.HighImportance // Make the launch button important
			return container.NewBorder(nil, nil, nil, container.NewHBox(launchButton, filesButton, removeButton), label)
		},
		func(index widget.ListItemID, item fyne.CanvasObject) {
			cont := item.(*fyne.Container)
			label := cont.Objects[0].(*widget.Label)
			buttons := cont.Objects[1].(*fyne.Container)
			launchButton := buttons.Objects[0].(*widget.Button)
			removeButton := buttons.Objects[2].(*widget.Button)
			filesButton := buttons.Objects[1].(*widget.Button)

			version := versions[index]
			label.SetText(version)
			launchButton.SetText("Launch")
			launchButton.OnTapped = func() {
				if currentProcess != nil {
					dialog.ShowError(fmt.Errorf("OWLCMS is already running"), w)
					return
				}

				fmt.Printf("Launching version %s\n", version)
				if err := checkJava(statusLabel, downloadGroup); err != nil {
					dialog.ShowError(fmt.Errorf("java check/installation failed: %w", err), w)
					return
				}

				if err := launchOwlcms(version, launchButton, stopButton, downloadGroup, versionContainer); err != nil {
					dialog.ShowError(err, w)
					return
				}
			}

			removeButton.SetText("Remove")
			removeButton.OnTapped = func() {
				dialog.ShowConfirm("Confirm Remove",
					fmt.Sprintf("Do you want to remove OWLCMS version %s?", version),
					func(ok bool) {
						if !ok {
							return
						}

						err := os.RemoveAll(filepath.Join(owlcmsInstallDir, version))
						if err != nil {
							dialog.ShowError(fmt.Errorf("failed to remove OWLCMS %s: %w", version, err), w)
							return
						}

						// Recompute the version list
						recomputeVersionList(w, downloadGroup)

						// Check if a more recent version is available
						checkForNewerVersion()
					},
					w)
			}

			filesButton.SetText("Files")
			filesButton.OnTapped = func() {
				versionDir := filepath.Join(owlcmsInstallDir, version)
				if err := openFileExplorer(versionDir); err != nil {
					dialog.ShowError(fmt.Errorf("failed to open file explorer for %s: %w", versionDir, err), w)
				}
			}
		},
	)

	versionList.OnSelected = func(id widget.ListItemID) {
		if id < len(versions) {
			fmt.Printf("Selected version: %s\n", versions[id])
		}
	}

	if len(versions) > 0 {
		versionList.Select(0)
	}

	if latest := findLatestInstalled(); latest != "" {
		for i, v := range versions {
			if v == latest {
				versionList.Select(i)
				break
			}
		}
	}

	// Log the versions being added
	fmt.Println("Versions being added to the version list:")
	for _, version := range versions {
		fmt.Println(version)
	}

	return versionList
}

func recomputeVersionList(w fyne.Window, downloadGroup *fyne.Container) {
	// Reinitialize the version list
	fmt.Println("Reinitializing version list")
	versionContainer.Objects = nil // Clear the container
	newVersionList := createVersionList(w, stopButton, downloadGroup, versionContainer)

	// Update the scroll container's size
	numVersions := len(getAllInstalledVersions())
	versionScroll := container.NewVScroll(newVersionList)
	versionScroll.SetMinSize(fyne.NewSize(400, computeVersionScrollHeight(numVersions)))

	versionLabel := widget.NewLabel("Installed Versions:")
	if numVersions == 0 {
		versionLabel.Hide()
		versionScroll.Hide()
		versionContainer.Hide()
	} else {
		versionLabel.Show()
		versionScroll.Show()
		versionContainer.Show()
	}
	versionContainer.Add(versionLabel)
	versionContainer.Add(versionScroll)

	fmt.Println("Version list reinitialized")
}
