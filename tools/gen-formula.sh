#!/bin/sh
# gen-formula.sh <dist-dir> <tag> — emits homebrew-<bot>.rb formula stubs
# for every tarball in dist, with correct sha256s baked in.
set -e
dist="$1"
tag="$2"
for tb in "$dist"/*-linux-amd64.tar.gz; do
  base=$(basename "$tb" .tar.gz)          # e.g. stale-bot-linux-amd64
  name="${base%-linux-amd64}"             # e.g. stale-bot
  sha=$(sha256sum "$tb" | cut -d' ' -f1)
  classname=$(echo "$name" | sed -e 's/-/ /g' | awk '{for(i=1;i<=NF;i++) printf "%s%s", toupper(substr($i,1,1)), substr($i,2)}')
  cat > "$dist/homebrew-${name}.rb" <<RBEOF
class ${classname} < Formula
  desc "Tiny self-hosted GitHub automation bot — ${name}"
  homepage "https://github.com/sanketpatel32/github-bot-store"
  url "https://github.com/sanketpatel32/github-bot-store/releases/download/${tag}/${base}.tar.gz"
  sha256 "${sha}"
  license "MIT"

  def install
    bin.install "${base}" => "${name}"
  end

  test do
    system "#{bin}/${name}", "version"
  end
end
RBEOF
done
echo "generated formulas:"
ls "$dist"/homebrew-*.rb | wc -l
