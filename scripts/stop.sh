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

for unit in "${controlpanel_units[@]}"; do
    if systemctl --user is-active "$unit" >/dev/null 2>&1; then
        echo "Stopping user unit $unit ..."
        systemctl --user stop "$unit" || echo "Warning: failed to stop $unit"
    fi
done