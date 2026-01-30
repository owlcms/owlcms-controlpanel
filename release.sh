#!/bin/bash
export TAG=v3.0.5

# Check if tag already exists
if git rev-parse "${TAG}" >/dev/null 2>&1; then
    echo "❌ ERROR: Tag ${TAG} already exists!"
    echo "Please choose a different version number."
    exit 1
fi

BUILD_MAC=true
BUILD_WINDOWS=true
BUILD_RASPBERRY=true
BUILD_LINUX=true

# Pull the latest changes
git pull

# Check for uncommitted changes (excluding release.sh and ReleaseNotes.md)
# git status --porcelain (v1) uses two status columns, e.g. " M release.sh" or "M  file" or "?? file".
# The file path starts at column 4.
UNCOMMITTED=$(git status --porcelain | awk '{p=substr($0,4); if (p!="release.sh" && p!="ReleaseNotes.md") print $0;}')
if [ -n "$UNCOMMITTED" ]; then
    echo "❌ ERROR: You have uncommitted changes other than release.sh and ReleaseNotes.md:"
    echo "$UNCOMMITTED"
    echo ""
    echo "Please commit all changes before creating a release!"
    exit 1
fi

# Commit release.sh and ReleaseNotes.md if they have changes
if git status --porcelain | awk '{p=substr($0,4); if (p=="release.sh" || p=="ReleaseNotes.md") {found=1}} END{exit found?0:1}'; then
    git add release.sh ReleaseNotes.md
    git commit -m "Update release.sh and ReleaseNotes.md for $TAG"
    git push
fi

# Update the resource configuration
export DEB_TAG=${TAG#v}
dist/updateRc.sh ${DEB_TAG}

# Substitute the values in release.yaml
sed -i "s/BUILD_MAC: .*/BUILD_MAC: ${BUILD_MAC}/" .github/workflows/release.yaml
sed -i "s/BUILD_WINDOWS: .*/BUILD_WINDOWS: ${BUILD_WINDOWS}/" .github/workflows/release.yaml
sed -i "s/BUILD_RASPBERRY: .*/BUILD_RASPBERRY: ${BUILD_RASPBERRY}/" .github/workflows/release.yaml
sed -i "s/BUILD_LINUX: .*/BUILD_LINUX: ${BUILD_LINUX}/" .github/workflows/release.yaml

# Commit and push the changes
git commit -am "owlcms-launcher $TAG"
git push
git tag -a ${TAG} -m "owlcms-launcher $TAG"
git push origin --tags

# Find and watch the workflow progress
echo "Watching workflow progress..."
RUN_ID=""
for i in {1..12}; do
    RUN_ID=$(gh run list --workflow="Release owlcms-controlpanel" --limit=1 --json databaseId,headBranch --jq ".[] | select(.headBranch==\"${TAG}\") | .databaseId")
    if [ -n "$RUN_ID" ]; then
        break
    fi
    echo "Waiting for workflow run to appear (attempt $i/12)..."
    sleep 5
done

if [ -z "$RUN_ID" ]; then
    echo "ERROR: Could not find workflow run for tag ${TAG}"
    echo "Check manually: gh run list --workflow='Release owlcms-controlpanel'"
    exit 1
fi

echo "Found workflow run: $RUN_ID"
gh run watch "$RUN_ID"

# Check final status
if gh run view "$RUN_ID" --exit-status; then
    echo "✅ Release workflow completed successfully!"
else
    echo "❌ Release workflow failed. Check: gh run view $RUN_ID --log-failed"
    exit 1
fi