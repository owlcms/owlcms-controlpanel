#!/bin/bash

set -e

if ! command -v systemctl &>/dev/null; then
    exit 0
fi

mapfile -t controlpanel_units < <(
    systemctl --user list-unit-files --type=service --no-legend 2>/dev/null \
        | awk '{print $1}' \
        | grep '^controlpanel-.*\.service$' || true
)

if [[ ${#controlpanel_units[@]} -eq 0 ]]; then
    exit 0
fi

echo "Reloading user systemd units..."
systemctl --user daemon-reload || true

for unit in "${controlpanel_units[@]}"; do
    if systemctl --user is-enabled "$unit" >/dev/null 2>&1 || systemctl --user is-active "$unit" >/dev/null 2>&1; then
        echo "Restarting user unit $unit ..."
        systemctl --user restart "$unit" || echo "Warning: failed to restart $unit"
    fi
done