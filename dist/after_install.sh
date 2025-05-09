#!/bin/bash
if [[ -n "$SUDO_USER" ]]; then
  user_home=$(getent passwd "$SUDO_USER" | cut -d: -f6)
cat > $user_home/Desktop/owlcms <<EOF
[Desktop Entry]
Type=Link
Name=owlcms
Icon=owlcms
URL=/usr/local/share/applications/owlcms.desktop
EOF
  chmod +x $user_home/Desktop/owlcms
else
  if [[ -n "$PACKAGEKIT_CALLER_UID" ]]; then
    user_home=$(getent passwd "$PACKAGEKIT_CALLER_UID" | cut -d: -f6)
cat > $user_home/Desktop/owlcms.desktop <<EOF
[Desktop Entry]
Type=Link
Name=owlcms
Icon=owlcms
URL=/usr/local/share/applications/owlcms.desktop
EOF
  chmod +x $user_home/Desktop/owlcms
  else
    echo "No user found to create the desktop file" > /tmp/owlcms.log
    printenv >> /tmp/owlcms.log
  fi
fi
