#!/bin/bash
set -e

echo "Building cortex..."
go build -o cortex ./cmd/cortex

# Install to ~/go/bin, which shadows ~/.local/bin on the default PATH.
# Installing only to ~/.local/bin leaves a stale ~/go/bin/cortex that
# the user keeps invoking without realising it.
mkdir -p ~/go/bin ~/.local/bin
mv -f cortex ~/go/bin/cortex
cp -f ~/go/bin/cortex ~/.local/bin/cortex

echo "Done! Installed to ~/go/bin/cortex and ~/.local/bin/cortex."