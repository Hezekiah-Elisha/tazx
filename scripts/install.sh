#!/usr/bin/env bash
echo "🚀 Installing Tazx (local build)..."

# Installing to local bin directory...
go build -o tazx

# Make the binary executable and move it to /usr/local/bin
chmod +x tazx

# Move the binary to /usr/local/bin (you may need sudo permissions)
sudo mv tazx /usr/local/bin/tazx

# Verify the installation
if command -v tazx &> /dev/null; then
    echo "✅ Tazx installed successfully!"
else
    echo "❌ Tazx installation failed. Please check for errors."
    exit 1
fi

echo "✅ Installed!"