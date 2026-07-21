#!/usr/bin/env bash
# Download, sign, notarize, and staple the macOS DMGs from a controlpanel release.

set -euo pipefail

usage() {
	cat <<'EOF'
Usage: scripts/notarize-release-macos-dmgs.sh RELEASE_TAG [output-directory]

Downloads these release assets from owlcms/owlcms-controlpanel:
  macOS_Intel_OWLCMS.dmg
  macOS_OWLCMS.dmg

The downloaded files are signed, submitted to Apple notarization through the
controlpanel-notary keychain profile, stapled, and validated. The output
directory defaults to dist/notarized/RELEASE_TAG.

Optional environment variables:
  APPLE_SIGNING_IDENTITY          Developer ID identity to use
  APPLE_NOTARY_KEYCHAIN_PROFILE  notarytool keychain profile (default: controlpanel-notary)
EOF
}

if (($# < 1 || $# > 2)); then
	usage >&2
	exit 2
fi

for command in codesign gh mkdir security xcrun; do
	if ! command -v "$command" >/dev/null 2>&1; then
		printf 'Required command is unavailable: %s\n' "$command" >&2
		exit 1
	fi
done

release_tag=$1
repository='owlcms/owlcms-controlpanel'
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repository_dir=$(cd -- "$script_dir/.." && pwd)
output_dir=${2:-"$repository_dir/dist/notarized/$release_tag"}
signing_identity="${APPLE_SIGNING_IDENTITY:-Developer ID Application: Jean-François Lamy (YABVW9SA37)}"
notary_profile="${APPLE_NOTARY_KEYCHAIN_PROFILE:-controlpanel-notary}"
dmg_assets=(macOS_Intel_OWLCMS.dmg macOS_OWLCMS.dmg)

if ! security find-identity -v -p codesigning 2>/dev/null | grep -qF "$signing_identity"; then
	printf 'Developer ID signing identity is unavailable: %s\n' "$signing_identity" >&2
	exit 1
fi

mkdir -p "$output_dir"

for dmg in "${dmg_assets[@]}"; do
	if [[ -e "$output_dir/$dmg" ]]; then
		printf 'Output file already exists; choose another output directory or remove it: %s\n' "$output_dir/$dmg" >&2
		exit 1
	fi
done

printf 'Downloading macOS DMGs from release %s\n' "$release_tag"
gh release download "$release_tag" --repo "$repository" --dir "$output_dir" \
	--pattern 'macOS_Intel_OWLCMS.dmg' \
	--pattern 'macOS_OWLCMS.dmg'

for dmg in "${dmg_assets[@]}"; do
	dmg_path="$output_dir/$dmg"
	if [[ ! -f "$dmg_path" ]]; then
		printf 'Release asset was not downloaded: %s\n' "$dmg" >&2
		exit 1
	fi

	printf 'Signing %s\n' "$dmg_path"
	codesign --force --timestamp --sign "$signing_identity" "$dmg_path"
	codesign --verify --verbose=2 "$dmg_path"

	printf 'Submitting %s to Apple notarization\n' "$dmg"
	xcrun notarytool submit "$dmg_path" --keychain-profile "$notary_profile" --wait
	xcrun stapler staple "$dmg_path"
	xcrun stapler validate "$dmg_path"
	done

printf 'Notarized DMGs are available in %s\n' "$output_dir"