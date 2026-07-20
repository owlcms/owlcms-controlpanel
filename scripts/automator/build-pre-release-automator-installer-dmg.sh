#!/usr/bin/env bash
# Build a local DMG containing the pre-release Automator installer application.

set -euo pipefail

if (($# > 1)); then
  echo "Usage: scripts/automator/build-pre-release-automator-installer-dmg.sh [output-dmg]" >&2
  exit 2
fi

for command in appdmg ditto sed; do
  if ! command -v "$command" >/dev/null 2>&1; then
    printf 'Required macOS command is unavailable: %s\n' "$command" >&2
    exit 1
  fi
done

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
template_app="$script_dir/Install controlpanel prerelease.app"
layout_template="$script_dir/install-pre-release-controlpanel.appdmg.json"
output_dmg=${1:-"$script_dir/macOS-controlpanel-prerelease.dmg"}

if [[ ! -d "$template_app/Contents" ]]; then
  printf 'Automator application is missing: %s\n' "$template_app" >&2
  exit 1
fi

if [[ ! -f "$layout_template" ]]; then
  printf 'DMG layout template is missing: %s\n' "$layout_template" >&2
  exit 1
fi

workspace=$(mktemp -d "${TMPDIR:-/tmp}/owlcms-pre-release-installer.XXXXXX")
trap 'rm -rf "$workspace"' EXIT

stage_dir="$workspace/stage"
launcher_name=$(basename "$template_app")
launcher_app="$stage_dir/$launcher_name"
layout_file="$workspace/install-pre-release-controlpanel.appdmg.json"

mkdir -p "$stage_dir"
ditto "$template_app" "$launcher_app"

# Re-sign ad hoc: editing Info.plist/resources invalidates the existing signature,
# which makes macOS refuse to launch the app ("damaged").
codesign --force --deep --sign - "$launcher_app"

sed "s|__APP_PATH__|$launcher_app|g" "$layout_template" > "$layout_file"

mkdir -p "$(dirname "$output_dmg")"
rm -f "$output_dmg"
appdmg "$layout_file" "$output_dmg"

printf 'Created %s\n' "$output_dmg"