# Running bots on a Raspberry Pi (or any ARM board)

These bots are ideal for a Pi: each idles around ~10MB RAM, so a Pi Zero 2W
can run the whole store and a Pi 4 barely notices them.

## Option 1: prebuilt binaries (easiest)

Every release ships `linux/arm64` tarballs — a Pi 3/4/5 or Zero 2W in 64-bit
OS uses them directly:

```sh
curl -L https://github.com/sanketpatel32/github-bot-store/releases/latest/download/stale-bot-linux-arm64.tar.gz | tar xz
sha256sum -c checksums.txt --ignore-missing   # verify!
chmod +x stale-bot-linux-arm64
GITHUB_TOKEN=ghp_... REPO=owner/repo ./stale-bot-linux-arm64
```

32-bit ARM (older Pi OS): build from source — Go cross-compiles for you, even
on the Pi itself:

```sh
GOOS=linux GOARCH=arm GOARM=7 go build -o stale-bot ./bots/stale-bot
```

## Option 2: build off-device (recommended for Pi Zero)

Cross-compile on any laptop — no Go install needed on the Pi at all:

```sh
git clone https://github.com/sanketpatel32/github-bot-store
cd github-bot-store
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o stale-bot-arm64 ./bots/stale-bot
scp stale-bot-arm64 pi@raspberrypi.local:/home/pi/
```

## Running them properly

Use the systemd unit in `deploy/systemd-example.service` — one unit per bot.
Memory footprint check:

```sh
systemctl status stale-bot | grep Memory   # expect ~10-15MB
```

## Tips

- A dozen bots use less RAM than one Chromium tab — the Pi handles it fine.
- Use `SPOOL_DIR` on webhook bots so a power blip doesn't lose events, but
  put it on a real disk/endurance SD — spool writes are tiny but frequent.
- `BIND_ADDR=127.0.0.1` + the Caddy example if the Pi sits behind NAT with a
  tunnel (cloudflared or tailscale funnels work well — the bots don't care
  how traffic arrives).
