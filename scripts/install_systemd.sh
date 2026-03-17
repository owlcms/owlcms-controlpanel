#!/bin/bash

set -e

if ! command -v systemctl &>/dev/null; then
    echo "systemctl not found; nothing to install."
    exit 0
fi

script_dir=$(cd "$(dirname "$0")" && pwd)
user_unit_dir="${HOME}/.config/systemd/user"

mkdir -p "$user_unit_dir"

mapfile -t sample_units < <(
    find "$script_dir" -maxdepth 1 -type f -name 'controlpanel-*.service' -printf '%f\n' | sort
)

if [[ ${#sample_units[@]} -eq 0 ]]; then
    echo "No controlpanel-*.service files found in $script_dir"
    exit 1
fi

for unit in "${sample_units[@]}"; do
    echo "Installing $unit to $user_unit_dir ..."
    install -m 0644 "$script_dir/$unit" "$user_unit_dir/$unit"
done

echo "Reloading user systemd units..."
systemctl --user daemon-reload

for unit in "${sample_units[@]}"; do
    echo "Enabling and starting $unit ..."
    systemctl --user enable --now "$unit"
done

echo ""
echo "Installed user services:"
for unit in "${sample_units[@]}"; do
    systemctl --user status "$unit" --no-pager --lines=0 || true
done
echo ""
echo "If you want these user services to keep running after logout, enable lingering:"
echo "  loginctl enable-linger $USER"