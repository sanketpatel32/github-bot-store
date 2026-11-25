# Package installation

## The one-line installer (recommended)

```sh
curl -fsSL https://raw.githubusercontent.com/sanketpatel32/github-bot-store/main/install.sh | sh -s stale-bot
```

Downloads the latest release binary for your arch, **verifies the sha256
checksum against the release's checksums.txt**, and installs to
/usr/local/bin. Then `stale-bot doctor` validates your setup.

## Homebrew

A formula generator ships in the release workflow — each release emits
`homebrew-<bot>.rb` templates. To use them as a tap:

```sh
# one-time tap setup (repo owner)
git clone https://github.com/sanketpatel32/homebrew-ghbot
# copy the generated .rb files into Formula/ and push

# then users:
brew install sanketpatel32/ghbot/stale-bot
```

## Manual

Every release ships linux/amd64 and linux/arm64 tarballs plus cosign
signatures — see docs/verifying-releases.md for full verification.

## Why not apt/dnf?

A single static binary with no dependencies doesn't benefit from deb/rpm
packaging — there's nothing to integrate, no libraries to resolve. The
install script covers the same ground with fewer moving parts; systemd unit
examples live in deploy/.
