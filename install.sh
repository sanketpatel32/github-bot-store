#!/bin/sh
# install.sh — install any bot from the store in one line:
#   curl -fsSL https://raw.githubusercontent.com/sanketpatel32/github-bot-store/main/install.sh | sh -s stale-bot
set -e

bot="$1"
if [ -z "$bot" ]; then
  echo "usage: $0 <bot-name> [install-dir]"
  echo "example: $0 stale-bot /usr/local/bin"
  exit 1
fi
dir="${2:-/usr/local/bin}"
arch=$(uname -m)
case "$arch" in
  x86_64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *) echo "unsupported arch: $arch"; exit 1 ;;
esac
os=$(uname -s | tr '[:upper:]' '[:lower:]')
[ "$os" = "linux" ] || { echo "only linux binaries are published; build from source on $os"; exit 1; }

base="https://github.com/sanketpatel32/github-bot-store/releases/latest/download"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "downloading $bot-linux-$arch..."
curl -fsSL "$base/$bot-linux-$arch.tar.gz" -o "$tmp/bot.tar.gz"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt"

echo "verifying checksum..."
(cd "$tmp" && grep "$bot-linux-$arch" checksums.txt | sha256sum -c -) || {
  echo "CHECKSUM MISMATCH — aborting"
  exit 1
}

tar -xzf "$tmp/bot.tar.gz" -C "$tmp"
bin=$(ls "$tmp" | grep "$bot-linux" | grep -v tar.gz | head -1)
echo "installing to $dir/$bot (may need sudo)..."
if [ -w "$dir" ]; then
  mv "$tmp/$bin" "$dir/$bot"
else
  sudo mv "$tmp/$bin" "$dir/$bot"
fi
chmod +x "$dir/$bot" 2>/dev/null || sudo chmod +x "$dir/$bot"
echo "installed: $dir/$bot"
echo "next: $bot doctor   # validates your configuration"
