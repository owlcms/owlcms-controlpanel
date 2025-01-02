#!/bin/bash
if [[ -n "$SUDO_USER" ]]; then
  user_home=$(getent passwd "$SUDO_USER" | cut -d: -f6)
  rm -f $user_home/Desktop/owlcms.desktop
else
  if [[ -n "$PACKAGEKIT_CALLER_UID" ]]; then
    user_home=$(getent passwd "$PACKAGEKIT_CALLER_UID" | cut -d: -f6)
    rm -f $user_home/Desktop/owlcms.desktop
  else
    echo "No user found to remove the desktop file" > /tmp/owlcms.log
    printenv >> /tmp/owlcms.log
  fi
fi