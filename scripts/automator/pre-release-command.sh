#!/bin/bash

LOG="/tmp/owlcms-prerelease-install.log"
echo "=== Install prerelease controlpanel started at $(date) ===" > "$LOG"

INSTALL_DIR="/Applications/owlcms"
DMG="/tmp/owlcms-controlpanel-prerelease.dmg"
RELEASES_JSON="/tmp/owlcms-controlpanel-releases.json"
RELEASES_URL="https://api.github.com/repos/owlcms/owlcms-controlpanel/releases?per_page=100"

osascript -e 'display notification "Finding the newest OWLCMS prerelease…" with title "Install prerelease controlpanel"'

echo "[1] Downloading release list from $RELEASES_URL" >> "$LOG"
curl --fail --location --retry 3 --output "$RELEASES_JSON" "$RELEASES_URL" >> "$LOG" 2>&1
if ! release_tag=$(osascript -l JavaScript -e '
function run(argv) {
    ObjC.import("Foundation");
    const text = ObjC.unwrap($.NSString.alloc.initWithContentsOfFileEncodingError($(argv[0]), $.NSUTF8StringEncoding, null));
    const release = JSON.parse(text).find(item => item.prerelease === true);
    if (!release) throw new Error("No prerelease");
    return release.tag_name;
}
' "$RELEASES_JSON" 2>> "$LOG"); then
    echo "ERROR: Could not find a GitHub prerelease." >> "$LOG"
    osascript -e 'display notification "Installation failed: no prerelease was found." with title "Install prerelease controlpanel"'
    exit 1
fi

LATEST_URL="https://github.com/owlcms/owlcms-controlpanel/releases/download/$release_tag/macOS_OWLCMS.dmg"
echo "[2] Downloading prerelease $release_tag from $LATEST_URL" >> "$LOG"
osascript -e "display notification \"Downloading prerelease $release_tag…\" with title \"Install prerelease controlpanel\""
curl --fail --location --retry 3 --output "$DMG" "$LATEST_URL" >> "$LOG" 2>&1

echo "[3] Creating install directory: $INSTALL_DIR" >> "$LOG"
mkdir -p "$INSTALL_DIR" 2>> "$LOG"

echo "[4] Mounting DMG…" >> "$LOG"
hdiutil attach "$DMG" >> "$LOG" 2>&1

MOUNT_POINT="/Volumes/OWLCMS Installer"
if [[ ! -d "$MOUNT_POINT" ]]; then
    echo "ERROR: Mount point not found: $MOUNT_POINT" >> "$LOG"
    osascript -e 'display notification "Installation failed: could not mount DMG." with title "Install prerelease controlpanel"'
    exit 1
fi

APP_IN_DMG="$MOUNT_POINT/owlcms.app"
if [[ ! -d "$APP_IN_DMG" ]]; then
    echo "ERROR: owlcms.app not found in DMG." >> "$LOG"
    osascript -e 'display notification "Installation failed: owlcms.app missing." with title "Install prerelease controlpanel"'
    hdiutil detach "$MOUNT_POINT" >> "$LOG" 2>&1
    exit 1
fi

echo "[5] Copying app to $INSTALL_DIR…" >> "$LOG"
cp -R "$APP_IN_DMG" "$INSTALL_DIR/" >> "$LOG" 2>&1

echo "[6] Unmounting DMG…" >> "$LOG"
hdiutil detach "$MOUNT_POINT" >> "$LOG" 2>&1

echo "[7] Clearing quarantine…" >> "$LOG"
xattr -rc "$INSTALL_DIR"/*.app >> "$LOG" 2>&1

echo "[8] Launching owlcms…" >> "$LOG"
open "$INSTALL_DIR"/*.app >> "$LOG" 2>&1

echo "=== Install prerelease controlpanel completed at $(date) ===" >> "$LOG"
osascript -e 'display notification "OWLCMS prerelease installation complete." with title "Install prerelease controlpanel"'

# Auto-eject the installer DMG this app was opened from, after this script exits.
INSTALLER_VOLUME="/Volumes/Install controlpanel prerelease"
if [ -d "$INSTALLER_VOLUME" ]; then
    echo "[9] Scheduling eject of $INSTALLER_VOLUME" >> "$LOG"
    nohup bash -c "sleep 3; hdiutil detach \"$INSTALLER_VOLUME\" -force" >> "$LOG" 2>&1 &
fi