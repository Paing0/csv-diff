#!/usr/bin/env bash
set -euo pipefail

echo "Installing CSV Diff globally..."

# Build binary
go build -ldflags="-s -w" -o csv-diff ./cmd/csv-diff

# Determine install target (/usr/local/bin or ~/.local/bin)
if [ -d "/usr/local/bin" ] && [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
elif [ -d "$HOME/.local/bin" ]; then
    INSTALL_DIR="$HOME/.local/bin"
else
    INSTALL_DIR="$(go env GOPATH)/bin"
fi

mkdir -p "$INSTALL_DIR"
cp csv-diff "$INSTALL_DIR/csv-diff"
chmod +x "$INSTALL_DIR/csv-diff"

echo "Installed csv-diff to $INSTALL_DIR/csv-diff"
echo "You can now run 'csv-diff' from any terminal."
