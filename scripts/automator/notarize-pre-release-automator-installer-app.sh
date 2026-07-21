#!/usr/bin/env bash
# Sign, notarize, and staple the prerelease Automator installer application locally.

set -euo pipefail

usage() {
	cat <<'EOF'
Usage: scripts/automator/notarize-pre-release-automator-installer-app.sh

Optional environment variables:
  APPLE_SIGNING_IDENTITY          Developer ID identity to use
  APPLE_NOTARY_KEYCHAIN_PROFILE  notarytool keychain profile (default: controlpanel-notary)
EOF
}

if (($# != 0)); then
	usage >&2
	exit 2
fi

for command in codesign ditto security xcrun; do
	if ! command -v "$command" >/dev/null 2>&1; then
		printf 'Required macOS command is unavailable: %s\n' "$command" >&2
		exit 1
	fi
done

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
app="$script_dir/Install controlpanel prerelease.app"
signing_identity="${APPLE_SIGNING_IDENTITY:-Developer ID Application: Jean-François Lamy (YABVW9SA37)}"
notary_profile="${APPLE_NOTARY_KEYCHAIN_PROFILE:-controlpanel-notary}"

if [[ ! -d "$app/Contents" ]]; then
	printf 'Automator application is missing: %s\n' "$app" >&2
	exit 1
fi

if ! security find-identity -v -p codesigning 2>/dev/null | grep -qF "$signing_identity"; then
	printf 'Developer ID signing identity is unavailable: %s\n' "$signing_identity" >&2
	exit 1
fi

workspace=$(mktemp -d "${TMPDIR:-/tmp}/owlcms-prerelease-automator-app-notarization.XXXXXX")
trap 'rm -rf "$workspace"' EXIT
archive="$workspace/$(basename "$app").zip"

codesign --force --deep --options runtime --timestamp --sign "$signing_identity" "$app"
codesign --verify --deep --strict --verbose=2 "$app"
ditto -c -k --keepParent "$app" "$archive"
xcrun notarytool submit "$archive" --keychain-profile "$notary_profile" --wait
xcrun stapler staple "$app"
xcrun stapler validate "$app"
codesign --verify --deep --strict --verbose=2 "$app"

printf 'Signed, notarized, and stapled %s\n' "$app"