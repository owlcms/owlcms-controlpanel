#!/bin/bash
# Full clean test: scrub configs, ffmpeg, versions, rebuild everything, deploy, launch.
# Run from the owlcms-controlpanel directory.

set -e

PANEL_DIR="$HOME/.local/share/owlcms-controlpanel"
CAMERAS_DIR="$HOME/.local/share/owlcms-cameras"
REPLAYS_DIR="$HOME/.local/share/owlcms-replays"
REPLAYS_SRC="$HOME/git/replays"
PANEL_SRC="$(cd "$(dirname "$0")" && pwd)"

echo "=== 1. Scrubbing installed state ==="

echo "  Removing video_config (ffmpeg.toml, shared configs)..."
rm -rf "$PANEL_DIR/video_config/"

echo "  Removing downloaded ffmpeg..."
rm -rf "$PANEL_DIR/ffmpeg/"

echo "  Removing cameras versions..."
rm -rf "$CAMERAS_DIR/"

echo "  Removing replays versions..."
rm -rf "$REPLAYS_DIR/"

echo ""
echo "=== 2. Building replays binaries ==="
cd "$REPLAYS_SRC"

ARCH=$(uname -m)
case "$ARCH" in
    aarch64) SUFFIX="linux_arm64" ;;
    x86_64)  SUFFIX="linux_amd64" ;;
    *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

echo "  Building cameras_${SUFFIX}..."
go build -o "cameras_${SUFFIX}" ./cmd/cameras

echo "  Building replays_${SUFFIX}..."
go build -o "replays_${SUFFIX}" ./cmd/replays

echo ""
echo "=== 3. Building control panel ==="
cd "$PANEL_SRC"
go build -o controlpanel .

echo ""
echo "=== 4. Creating fake version dirs and deploying dev binaries ==="

# Use a dev version tag
DEV_VERSION="0.0.0-dev"

CAMERAS_VER_DIR="$CAMERAS_DIR/$DEV_VERSION"
REPLAYS_VER_DIR="$REPLAYS_DIR/$DEV_VERSION"

mkdir -p "$CAMERAS_VER_DIR"
mkdir -p "$REPLAYS_VER_DIR"

cp "$REPLAYS_SRC/cameras_${SUFFIX}" "$CAMERAS_VER_DIR/cameras_${SUFFIX}"
cp "$REPLAYS_SRC/replays_${SUFFIX}" "$REPLAYS_VER_DIR/replays_${SUFFIX}"

echo "  Deployed cameras to $CAMERAS_VER_DIR/"
echo "  Deployed replays to $REPLAYS_VER_DIR/"

echo ""
echo "=== 5. Launching control panel ==="
echo "  (ffmpeg download, config extraction, and launch will happen from the UI)"
echo ""
exec "$PANEL_SRC/controlpanel"
