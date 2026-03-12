#!/bin/bash
# Install or upgrade the owlcms Control Panel on headless Ubuntu (amd64 or arm64)
# Usage:
#   git clone https://github.com/owlcms/owlcms-controlpanel
#   bash owlcms-controlpanel/scripts/install.sh [--prerelease|--release] [--version 3.3.0]

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

REPO="owlcms/owlcms-controlpanel"

# Pick the right .deb for this architecture
ARCH=$(dpkg --print-architecture)
case "$ARCH" in
    amd64)  ASSET_NAME="Linux_Control_Panel_Installer.deb" ;;
    arm64)  ASSET_NAME="Raspberry_Pi_arm64_Control_Panel_Installer.deb" ;;
    *)      echo "Error: unsupported architecture: $ARCH"; exit 1 ;;
esac

# Parse optional selection flags
VERSION=""
INSTALL_KIND="prerelease"
while [[ $# -gt 0 ]]; do
    case "$1" in
        --prerelease)
            INSTALL_KIND="prerelease"
            shift
            ;;
        --release)
            INSTALL_KIND="release"
            shift
            ;;
        --version|-v)
            VERSION="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--prerelease|--release] [--version <tag>]"
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

# Resolve the release tag. Default selection is the latest prerelease.
if [[ -z "$VERSION" ]]; then
    if [[ "$INSTALL_KIND" == "release" ]]; then
        echo "Fetching latest stable release tag from GitHub..."
        VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | jq -r '.tag_name')
    else
        echo "Fetching latest prerelease tag from GitHub..."
        VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases" | jq -r '[.[] | select(.prerelease == true)][0].tag_name')
    fi
    if [[ -z "$VERSION" || "$VERSION" == "null" ]]; then
        echo "Error: could not determine latest ${INSTALL_KIND} version."
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

"${SCRIPT_DIR}/restart.sh"

echo ""
echo "Done. owlcms Control Panel ${VERSION} is installed."
echo "Run 'owlcms' or 'controlpanel' to start it."
