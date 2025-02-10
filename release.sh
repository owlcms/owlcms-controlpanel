#!/bin/bash -x
export TAG=v0.9.0
export REMOTE=firmata-controlpanel
git tag -d ${TAG}
git push ${REMOTE} --delete ${TAG}
gh release delete ${TAG} --yes

BUILD_MAC=false
BUILD_WINDOWS=true
BUILD_RASPBERRY=true
BUILD_LINUX=true

# Pull the latest changes
git pull

# Update the resource configuration
export DEB_TAG=${TAG#v}
dist/updateRc.sh ${DEB_TAG}

# Substitute the values in release.yaml
sed -i "s/BUILD_MAC: .*/BUILD_MAC: ${BUILD_MAC}/" .github/workflows/release.yaml
sed -i "s/BUILD_WINDOWS: .*/BUILD_WINDOWS: ${BUILD_WINDOWS}/" .github/workflows/release.yaml
sed -i "s/BUILD_RASPBERRY: .*/BUILD_RASPBERRY: ${BUILD_RASPBERRY}/" .github/workflows/release.yaml
sed -i "s/BUILD_LINUX: .*/BUILD_LINUX: ${BUILD_LINUX}/" .github/workflows/release.yaml

# Commit and push the changes
git commit -am "firmata-launcher $TAG"
git push ${REMOTE} HEAD:main
git tag -a ${TAG} -m "firmata-launcher $TAG"
git push ${REMOTE} --tags