#!/bin/bash
set -e

echo "========================================="
echo " Installing SSH over WebSocket Gateway..."
echo "========================================="

# Detect OS and Arch
OS="$(uname -s)"
ARCH="$(uname -m)"

case "${OS}" in
    Linux*)     OS="linux";;
    Darwin*)    OS="darwin";;
    *)          echo "Unsupported OS: ${OS}"; exit 1;;
esac

case "${ARCH}" in
    x86_64)     ARCH="amd64";;
    arm64|aarch64) ARCH="arm64";;
    *)          echo "Unsupported architecture: ${ARCH}"; exit 1;;
esac

REPO="kelvinzer0/ssh-over-websocket"
API_URL="https://api.github.com/repos/$REPO/releases/latest"

echo "Fetching latest release information..."
# Get download URL for the specific OS and Arch
DOWNLOAD_URL=$(curl -s $API_URL | grep "browser_download_url" | grep "${OS}_${ARCH}" | grep "tar.gz" | cut -d '"' -f 4)

if [ -z "$DOWNLOAD_URL" ]; then
    echo "Error: Could not find a release for ${OS}_${ARCH}."
    echo "Please check https://github.com/$REPO/releases"
    exit 1
fi

echo "Downloading from: $DOWNLOAD_URL"
TMP_DIR=$(mktemp -d)
curl -sL "$DOWNLOAD_URL" -o "$TMP_DIR/ssh-gateway.tar.gz"

echo "Extracting..."
tar -xzf "$TMP_DIR/ssh-gateway.tar.gz" -C "$TMP_DIR"

INSTALL_DIR="/usr/local/bin"
echo "Installing to $INSTALL_DIR (requires sudo)..."
sudo mv "$TMP_DIR/ssh-gateway" "$INSTALL_DIR/ssh-gateway"
sudo chmod +x "$INSTALL_DIR/ssh-gateway"

rm -rf "$TMP_DIR"

echo ""
echo "Installation complete!"
echo "You can now run 'ssh-gateway' from anywhere."
echo ""
echo "To set it up as a system service, run:"
echo "  sudo ssh-gateway -install -bind :8080"
echo "========================================="
