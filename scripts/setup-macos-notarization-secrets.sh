#!/usr/bin/env bash
# Upload the secrets needed to sign and notarize macOS release DMGs in GitHub Actions.

set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  ./scripts/setup-macos-notarization-secrets.sh \
    /path/to/DeveloperIDApplication.p12 \
    "Developer ID Application: Your Name (TEAMID)" \
    [owner/repository]

The .p12 must contain a Developer ID Application certificate and its private key.
The Team ID is read from the (TEAMID) suffix of the signing identity. The script
prompts for the Apple ID used for the Developer Program and its app-specific
password. Generate the app-specific password at https://account.apple.com under
Sign-In and Security.

The target repository defaults to the current repository reported by gh.
EOF
}

if [[ $# -lt 2 || $# -gt 3 ]]; then
  usage >&2
  exit 2
fi

certificate_file=$1
signing_identity=$2
repository=${3:-}

if [[ ! -f "$certificate_file" ]]; then
  printf 'File not found: %s\n' "$certificate_file" >&2
  exit 2
fi

# Read the Team ID from the (TEAMID) suffix of the signing identity.
if [[ "$signing_identity" =~ \(([A-Z0-9]{10})\)[[:space:]]*$ ]]; then
  apple_team_id=${BASH_REMATCH[1]}
else
  echo "Could not read a 10-character Team ID from the signing identity." >&2
  echo "Expected a value ending in (TEAMID), e.g. 'Developer ID Application: Name (ABCDE12345)'." >&2
  exit 2
fi

if ! command -v gh >/dev/null 2>&1; then
  echo "The GitHub CLI (gh) is required." >&2
  exit 1
fi

if ! gh auth status >/dev/null 2>&1; then
  echo "GitHub CLI is not authenticated. Run: gh auth login" >&2
  exit 1
fi

if [[ -z "$repository" ]]; then
  repository=$(gh repo view --json nameWithOwner --jq '.nameWithOwner')
fi

read -r -s -p "Password used when exporting the .p12: " certificate_password
printf '\n'
if [[ -z "$certificate_password" ]]; then
  echo "The certificate export password cannot be empty." >&2
  exit 2
fi

read -r -p "Apple ID used for the Apple Developer Program: " apple_id
if [[ -z "$apple_id" ]]; then
  echo "The Apple ID cannot be empty." >&2
  exit 2
fi

printf 'Using Team ID %s from the signing identity.\n' "$apple_team_id"

read -r -s -p "Apple ID app-specific password: " apple_app_specific_password
printf '\n'
if [[ -z "$apple_app_specific_password" ]]; then
  echo "The Apple ID app-specific password cannot be empty." >&2
  exit 2
fi

keychain_password=$(openssl rand -base64 32)

set_secret() {
  local secret_name=$1
  gh secret set "$secret_name" --repo "$repository"
  printf 'Stored %s in %s\n' "$secret_name" "$repository"
}

base64 < "$certificate_file" | set_secret MACOS_CERTIFICATE_BASE64
printf '%s' "$certificate_password" | set_secret MACOS_CERTIFICATE_PASSWORD
printf '%s' "$keychain_password" | set_secret MACOS_KEYCHAIN_PASSWORD
printf '%s' "$apple_id" | set_secret APPLE_ID
printf '%s' "$apple_team_id" | set_secret APPLE_TEAM_ID
printf '%s' "$apple_app_specific_password" | set_secret APPLE_APP_SPECIFIC_PASSWORD
printf '%s' "$signing_identity" | gh variable set APPLE_SIGNING_IDENTITY --repo "$repository"
printf 'Stored APPLE_SIGNING_IDENTITY as a repository variable in %s\n' "$repository"

echo "Done. The GitHub Actions workflow can now import the signing certificate and notarize the DMGs."