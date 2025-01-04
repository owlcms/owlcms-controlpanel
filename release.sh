#!/bin/bash -x
export TAG=1.3.1
git pull
rm -f owlcms-launcher*

fyne-cross windows -arch amd64 -app-id app.owlcms.owlcms-launcher -icon Icon.png -name owlcms
fyne-cross linux -arch amd64 -app-id app.owlcms.owlcms-launcher -icon Icon.png -name owlcms
fyne-cross linux -arch arm64 -app-id app.owlcms.owlcms-launcher -icon Icon.png -name owlcms

cp fyne-cross/bin/linux-arm64/owlcms-launcher fyne-cross/bin/linux-arm64/owlcms-pi
cp fyne-cross/bin/linux-amd64/owlcms-launcher fyne-cross/bin/linux-amd64/owlcms-linux

fpm -s tar -t deb -n owlcms-launcher -v ${TAG} -a arm64 --prefix / --after-install ./dist/after_install.sh --after-remove ./dist/after_remove.sh ./fyne-cross/dist/linux-arm64/owlcms.tar.xz 
fpm -s tar -t deb -n owlcms-launcher -v ${TAG} -a amd64 --prefix / --after-install ./dist/after_install.sh --after-remove ./dist/after_remove.sh ./fyne-cross/dist/linux-amd64/owlcms.tar.xz
mv owlcms-launcher_${TAG}_arm64.deb owlcms-launcher_${TAG}_pi.deb


# gh requires data from the current repo
cp RELEASE.md /tmp
sed -i "s/_TAG_/${TAG}/g" ./RELEASE.md
cp /tmp/RELEASE.md .


git add --all
git commit -m "${TAG}"
git push

gh release delete ${TAG} -y
gh release create ${TAG} --notes-file ./RELEASE.md -t "owlcms-launcher ${TAG}"
gh release upload ${TAG} owlcms-launcher_${TAG}_pi.deb
gh release upload ${TAG} owlcms-launcher_${TAG}_amd64.deb
gh release upload ${TAG} fyne-cross/bin/linux-arm64/owlcms-pi
gh release upload ${TAG} fyne-cross/bin/linux-amd64/owlcms-linux
gh release upload ${TAG} fyne-cross/bin/windows-amd64/owlcms.exe

# git fetch --tags
