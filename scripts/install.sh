#!/bin/sh
set -eu

repo="atakang7/cortex"
install_dir="${CORTEX_INSTALL_DIR:-$HOME/.local/bin}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux|darwin) ;;
  *)
    echo "cortex: unsupported OS '$os'. On Windows, run this installer inside WSL2." >&2
    exit 1
    ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *)
    echo "cortex: unsupported architecture '$arch' (supported: amd64, arm64)." >&2
    exit 1
    ;;
esac

asset="cortex-${os}-${arch}.tar.gz"
url="https://github.com/${repo}/releases/latest/download/${asset}"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

mkdir -p "$install_dir"
echo "cortex: downloading latest ${os}/${arch} release..."
curl -fL --retry 3 --retry-delay 1 "$url" -o "$tmp/cortex.tar.gz"
tar -xzf "$tmp/cortex.tar.gz" -C "$tmp" cortex
install -m 0755 "$tmp/cortex" "$install_dir/cortex"

echo "cortex: installed to $install_dir/cortex"

case ":${PATH}:" in
  *":${install_dir}:"*) ;;
  *)
    echo "cortex: add $install_dir to PATH, for example:" >&2
    echo "  export PATH=\"$install_dir:\$PATH\"" >&2
    ;;
esac

"$install_dir/cortex" --version
