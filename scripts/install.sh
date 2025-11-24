#!/bin/bash
# VeniceCode Installation Script

set -e

VERSION="${VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="venicecode"

echo "Installing VeniceCode..."

# Detect OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

# Map architecture names
case "$ARCH" in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo "Error: Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

# Construct download URL
if [ "$VERSION" = "latest" ]; then
    DOWNLOAD_URL="https://github.com/georgeglarson/venicecode/releases/latest/download/${BINARY_NAME}-${OS}-${ARCH}.tar.gz"
else
    DOWNLOAD_URL="https://github.com/georgeglarson/venicecode/releases/download/v${VERSION}/${BINARY_NAME}-${OS}-${ARCH}.tar.gz"
fi

echo "Downloading VeniceCode for ${OS}/${ARCH}..."
echo "URL: $DOWNLOAD_URL"

# Create temp directory
TMP_DIR="$(mktemp -d)"
trap "rm -rf $TMP_DIR" EXIT

# Download and extract
curl -L "$DOWNLOAD_URL" -o "$TMP_DIR/${BINARY_NAME}.tar.gz"
tar -xzf "$TMP_DIR/${BINARY_NAME}.tar.gz" -C "$TMP_DIR"

# Install binary
echo "Installing to $INSTALL_DIR..."
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP_DIR/${BINARY_NAME}-${OS}-${ARCH}" "$INSTALL_DIR/$BINARY_NAME"
else
    sudo mv "$TMP_DIR/${BINARY_NAME}-${OS}-${ARCH}" "$INSTALL_DIR/$BINARY_NAME"
fi

chmod +x "$INSTALL_DIR/$BINARY_NAME"

echo ""
echo "✓ VeniceCode installed successfully!"
echo ""
echo "Next steps:"
echo "  1. Get your Venice.ai API key: https://venice.ai/settings/api"
echo "  2. Set the API key: export VENICE_API_KEY=\"your-key-here\""
echo "  3. Run VeniceCode: venicecode"
echo ""
echo "Documentation: https://github.com/georgeglarson/venicecode"
