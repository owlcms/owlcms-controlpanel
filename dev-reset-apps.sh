#!/bin/bash
# Apps-only dev helper: reset app versions, build cameras/replays, deploy dev binaries.
# Leaves control panel files and binary untouched.

set -e

if [ -n "$APPDATA" ]; then
	CAMERAS_DIR="$APPDATA/owlcms-cameras"
	REPLAYS_DIR="$APPDATA/owlcms-replays"
	REPLAYS_SRC="/c/Dev/git/replays"
else
	CAMERAS_DIR="$HOME/.local/share/owlcms-cameras"
	REPLAYS_DIR="$HOME/.local/share/owlcms-replays"
	REPLAYS_SRC="$HOME/git/replays"
fi

DEV_VERSION="0.0.0-dev"

echo "=== 1. Resetting app installed state ==="

echo "  Removing cameras versions..."
rm -rf "$CAMERAS_DIR/"

echo "  Removing replays versions..."
rm -rf "$REPLAYS_DIR/"

echo ""
echo "=== 2. Building app binaries ==="
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
echo "=== 3. Deploying app binaries ==="
CAMERAS_VER_DIR="$CAMERAS_DIR/$DEV_VERSION"
REPLAYS_VER_DIR="$REPLAYS_DIR/$DEV_VERSION"

mkdir -p "$CAMERAS_VER_DIR"
mkdir -p "$REPLAYS_VER_DIR"

cp "$REPLAYS_SRC/cameras_${SUFFIX}" "$CAMERAS_VER_DIR/cameras_${SUFFIX}"
cp "$REPLAYS_SRC/replays_${SUFFIX}" "$REPLAYS_VER_DIR/replays_${SUFFIX}"

echo "  Deployed cameras to $CAMERAS_VER_DIR/"
echo "  Deployed replays to $REPLAYS_VER_DIR/"

echo ""
echo "Apps-only reset/build/deploy complete."
