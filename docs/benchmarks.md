# Benchmarks — binary size and memory

Measured 2026-11-25 on linux/amd64, Go 1.23, `-ldflags="-s -w"`, CGO disabled.
Reproduce with `./bench.sh` in the repo root.

## Results

| Metric | Value |
|---|---|
| Binary size (stripped, all 29 executables) | **~5.7MB each** |
| Idle memory (RSS, webhook server up) | **~5.6MB** |
| Build time (single bot, warm cache) | ~1-2s |
| Dependencies | **zero** — Go stdlib only |

What this means in practice: the entire store — every bot running
simultaneously — fits in roughly **170MB of RAM**. A $4/month VPS or a
Raspberry Pi Zero 2W runs all of them without breaking a sweat.

## Methodology

```sh
# binary size
CGO_ENABLED=0 go build -ldflags="-s -w" -o bot ./bots/stale-bot
ls -la bot

# idle memory — boot the server, read RSS after warmup
GITHUB_TOKEN=x ALLOW_UNSIGNED_WEBHOOKS=true ./bot &
sleep 1
grep VmRSS /proc/$!/status
```

## Why so small

- **stdlib only** — no dependency tree compiled in, no runtime assets
- static single binary — no interpreter, no VM, no container runtime needed
- stripped symbols (`-s -w`) — debug info isn't useful in production here
- bounded worker pool — memory doesn't grow with load; excess load returns
  429 for GitHub to retry rather than queuing unboundedly

If you measure a bot exceeding ~8MB RSS at idle, that's a regression —
please file an issue.
