#!/bin/bash
# Install bd-tui to ~/.local/bin

set -e

# Build if binary doesn't exist or is older than source
if [ ! -f ./bd-tui ] || [ ./cmd/bd-tui/main.go -nt ./bd-tui ]; then
    echo "Building bd-tui..."
    go build -o bd-tui ./cmd/bd-tui
fi

# Copy to ~/.local/bin
mkdir -p ~/.local/bin
cp ./bd-tui ~/.local/bin/bd-tui
echo "Installed bd-tui to ~/.local/bin/bd-tui"
