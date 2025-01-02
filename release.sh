#!/bin/bash
export TAG=1.2.0-alpha00
git pull

#fyne-cross windows -arch amd64 -app-id app.owlcms.owlcms-launcher -icon Icon.png -name owlcms
#fyne-cross linux -arch amd64 -app-id app.owlcms.owlcms-launcher -icon Icon.png -name owlcms
#fyne-cross linux -arch arm64 -app-id app.owlcms.owlcms-launcher -icon Icon.png -name owlcms

#cp fyne-cross/bin/linux-arm64/owlcms-launcher fyne-cross/bin/linux-arm64/owlcms-pi
#cp fyne-cross/bin/linux-amd64/owlcms-launcher fyne-cross/bin/linux-amd64/owlcms-linux

fpm -s tar -t deb -n owlcms-launcher -v $TAG -a arm64 --prefix / --after-install ./dist/after_install.sh --after-remove ./dist/after_remove.sh ./fyne-cross/dist/linux-arm64/owlcms.tar.xz 

#fpm -s tar -t deb -n owlcms-amd64 -v $TAG -a amd64 --prefix / --chdir ./fyne-cross/dist/linux-amd64 --after-install ./create_desktop.sh owlcms.tar.xz 

exit

git add --all
git commit -m "$TAG"
git push

gh release delete $TAG -y
gh release create $TAG --notes-file RELEASE.md -t "owlcms-launcher $TAG"
gh release upload $TAG fyne-cross/bin/linux-arm64/owlcms-pi
gh release upload $TAG fyne-cross/bin/linux-amd64/owlcms-linux
gh release upload $TAG fyne-cross/bin/windows-amd64/owlcms-windows.exe

git fetch --tags
