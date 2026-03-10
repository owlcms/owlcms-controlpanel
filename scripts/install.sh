#!/bin/bash
# Install or upgrade the owlcms Control Panel on headless Ubuntu (amd64)
# Usage:
#   git clone https://github.com/owlcms/owlcms-controlpanel
#   bash owlcms-controlpanel/scripts/install.sh [--version 3.3.0]

set -e

REPO="owlcms/owlcms-controlpanel"
ASSET_NAME="Linux_Control_Panel_Installer.deb"

# Parse optional --version flag
VERSION=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --version|-v)
            VERSION="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--version <tag>]"
            exit 1
            ;;
    esac
done

# Require curl
if ! command -v curl &>/dev/null; then
    echo "Installing curl..."
    sudo apt-get update -q && sudo apt-get install -y curl
fi

# Resolve the release tag
if [[ -z "$VERSION" ]]; then
    echo "Fetching latest release tag from GitHub..."
    RELEASE_JSON=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest")
    VERSION=$(echo "$RELEASE_JSON" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
    if [[ -z "$VERSION" ]]; then
        echo "Error: could not determine latest release version."
        exit 1
    fi
fi

echo "Installing owlcms Control Panel ${VERSION}..."

# Build download URL
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET_NAME}"

TMP_DEB=$(mktemp /tmp/owlcms-controlpanel-XXXXXX.deb)
trap 'rm -f "$TMP_DEB"' EXIT

echo "Downloading ${DOWNLOAD_URL} ..."
curl -fsSL -o "$TMP_DEB" "$DOWNLOAD_URL"

echo "Installing package..."
sudo apt-get install -y "$TMP_DEB"

echo ""
echo "Done. owlcms Control Panel ${VERSION} is installed."
echo "Run 'owlcms' or 'controlpanel' to start it."
