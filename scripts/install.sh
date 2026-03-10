#!/bin/bash
# Install or upgrade the owlcms Control Panel on headless Ubuntu (amd64 or arm64)
# Usage:
#   git clone https://github.com/owlcms/owlcms-controlpanel
#   bash owlcms-controlpanel/scripts/install.sh [--version 3.3.0]

set -e

REPO="owlcms/owlcms-controlpanel"

# Pick the right .deb for this architecture
ARCH=$(dpkg --print-architecture)
case "$ARCH" in
    amd64)  ASSET_NAME="Linux_Control_Panel_Installer.deb" ;;
    arm64)  ASSET_NAME="Raspberry_Pi_arm64_Control_Panel_Installer.deb" ;;
    *)      echo "Error: unsupported architecture: $ARCH"; exit 1 ;;
esac

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

# Require curl and jq
for cmd in curl jq; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "Installing $cmd..."
        sudo apt-get update -q && sudo apt-get install -y "$cmd"
    fi
done

# Resolve the release tag using /releases (includes pre-releases; /releases/latest skips them)
if [[ -z "$VERSION" ]]; then
    echo "Fetching latest release tag from GitHub..."
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases" | jq -r '.[0].tag_name')
    if [[ -z "$VERSION" || "$VERSION" == "null" ]]; then
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
