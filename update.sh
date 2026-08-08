#!/bin/bash
set -e

echo "Building cortex..."
go build -o cortex ./cmd/cortex

echo "Replacing existing binary at ~/.local/bin/cortex..."
mkdir -p ~/.local/bin
mv cortex ~/.local/bin/cortex

echo "Done! You can now run 'cortex' from anywhere."
