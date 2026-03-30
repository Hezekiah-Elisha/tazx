#!/usr/bin/env bash

set -e

echo "🚀 Installing Tazx..."

# Detect OS
OS="$(uname -s)"
ARCH="$(uname -m)"

# Normalize values
case "$OS" in
    Linux) OS="linux" ;;
    Darwin) OS="darwin" ;;
    *) echo "❌ Unsupported OS: $OS"; exit 1 ;;
esac

case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    arm64) ARCH="arm64" ;;
    *) echo "❌ Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Download binary
URL="https://github.com/Hezekiah-Elisha/tazx/releases/latest/download/tazx-${OS}-${ARCH}"

echo "⬇️ Downloading from $URL..."
curl -L -o tazx "$URL"

# Make executable
chmod +x tazx

# Move to /usr/local/bin
echo "📦 Installing to /usr/local/bin..."
sudo mv tazx /usr/local/bin/tazx

# Verify
echo "✅ Installed successfully!"
tazx --help || true

# Optional init
echo ""
read -p "Run 'tazx init' now? (y/n): " choice
if [[ "$choice" == "y" ]]; then
    tazx init
fi