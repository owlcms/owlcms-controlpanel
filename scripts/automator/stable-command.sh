#!/bin/bash

LOG="/tmp/owlcms-install.log"
echo "=== Install OWLCMS controlpanel started at $(date) ===" > "$LOG"

# Start notification
osascript -e 'display notification "Installing OWLCMS controlpanel…" with title "Install OWLCMS controlpanel"'

INSTALL_DIR="/Applications/owlcms"
DMG="/tmp/owlcms-controlpanel-latest.dmg"
LATEST_URL="https://github.com/owlcms/owlcms-controlpanel/releases/latest/download/macOS_OWLCMS.dmg"

echo "[1] Creating install directory: $INSTALL_DIR" >> "$LOG"
mkdir -p "$INSTALL_DIR" 2>>"$LOG"

echo "[2] Downloading DMG from $LATEST_URL" >> "$LOG"
curl -L -o "$DMG" "$LATEST_URL" >>"$LOG" 2>&1
echo "Download exit code: $?" >> "$LOG"

echo "[3] Mounting DMG…" >> "$LOG"
hdiutil attach "$DMG" >>"$LOG" 2>&1

# Your DMG volume name (as shown in Finder)
MOUNT_POINT="/Volumes/OWLCMS Installer"
echo "Mount point: $MOUNT_POINT" >> "$LOG"

if [ ! -d "$MOUNT_POINT" ]; then
    echo "ERROR: Mount point not found: $MOUNT_POINT" >> "$LOG"
    osascript -e 'display notification "Installation failed: could not mount DMG." with title "Install OWLCMS controlpanel"'
    exit 1
fi

echo "[4] Locating owlcms.app inside DMG…" >> "$LOG"
APP_IN_DMG="$MOUNT_POINT/owlcms.app"
echo "App path: $APP_IN_DMG" >> "$LOG"

if [ ! -d "$APP_IN_DMG" ]; then
    echo "ERROR: owlcms.app not found in DMG." >> "$LOG"
    osascript -e 'display notification "Installation failed: owlcms.app missing." with title "Install OWLCMS controlpanel"'
    hdiutil detach "$MOUNT_POINT" >>"$LOG" 2>&1
    exit 1
fi

echo "[5] Copying app to $INSTALL_DIR…" >> "$LOG"
cp -R "$APP_IN_DMG" "$INSTALL_DIR/" >>"$LOG" 2>&1
echo "Copy exit code: $?" >> "$LOG"

echo "[6] Unmounting DMG…" >> "$LOG"
hdiutil detach "$MOUNT_POINT" >>"$LOG" 2>&1

echo "[7] Clearing quarantine…" >> "$LOG"
xattr -rc "$INSTALL_DIR"/*.app >>"$LOG" 2>&1

echo "[8] Launching owlcms…" >> "$LOG"
open "$INSTALL_DIR"/*.app >>"$LOG" 2>&1

echo "=== Install OWLCMS controlpanel completed at $(date) ===" >> "$LOG"

# Final notification
osascript -e 'display notification "OWLCMS controlpanel installation complete." with title "Install OWLCMS controlpanel"'

# Auto-eject the installer DMG this app was opened from, after this script exits.
INSTALLER_VOLUME="/Volumes/Install controlpanel"
if [ -d "$INSTALLER_VOLUME" ]; then
    echo "[9] Scheduling eject of $INSTALLER_VOLUME" >> "$LOG"
    nohup bash -c "sleep 3; hdiutil detach \"$INSTALLER_VOLUME\" -force" >> "$LOG" 2>&1 &
fi
