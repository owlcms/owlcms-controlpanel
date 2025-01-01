#!/bin/bash
export TAG=1.1.0
git pull

fyne-cross windows -arch amd64 -app-id app.owlcms.owlcms-launcher -icon Icon.png -name owlcms-windows
fyne-cross linux -arch amd64 -app-id app.owlcms.owlcms-launcher -icon Icon.png -name owlcms-linux
fyne-cross linux -arch arm64 -app-id app.owlcms.owlcms-launcher -icon Icon.png -name owlcms-pi

cp fyne-cross/bin/linux-arm64/owlcms-launcher fyne-cross/bin/linux-arm64/owlcms-pi
cp fyne-cross/bin/linux-amd64/owlcms-launcher fyne-cross/bin/linux-amd64/owlcms-linux

git add --all
git commit -m "$TAG"
git push

gh release delete $TAG -y
gh release create $TAG --notes-file RELEASE.md -t "owlcms-launcher $TAG"
gh release upload $TAG fyne-cross/bin/linux-arm64/owlcms-pi
gh release upload $TAG fyne-cross/bin/linux-amd64/owlcms-linux
gh release upload $TAG fyne-cross/bin/windows-amd64/owlcms-windows.exe

git fetch --tags
