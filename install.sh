#!/bin/bash
set -e

echo "Installing Chimera..."

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed. Please install Go 1.21+ first."
    echo "Visit: https://go.dev/doc/install"
    exit 1
fi

# Create temp directory
TMP_DIR=$(mktemp -d)
cd "$TMP_DIR"

# Clone and build
echo "Downloading source..."
git clone --depth 1 https://github.com/monojitgoswami69/chimera.git
cd chimera

echo "Building..."
go build -o chimera .

# Install to user's local bin
INSTALL_DIR="$HOME/.local/bin"
mkdir -p "$INSTALL_DIR"
mv chimera "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/chimera"

# Cleanup
cd ~
rm -rf "$TMP_DIR"

echo ""
echo "✓ Chimera installed successfully to $INSTALL_DIR/chimera"
echo ""
echo "Make sure $INSTALL_DIR is in your PATH:"
echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
echo ""
echo "Run 'chimera setup' to get started!"
