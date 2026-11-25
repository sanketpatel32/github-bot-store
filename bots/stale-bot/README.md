# stale-bot

Marks issues and PRs inactive for 30 days as `stale`, then closes them 7 days later if nothing happens. Any activity resets the clock.

This one is **cron-style**, not webhook-driven — it polls the repo on an interval.

## Setup

```bash
go build -o stale-bot ./bots/stale-bot
export GITHUB_TOKEN=... REPO=owner/repo
./stale-bot   # listens on :8103 (health only)
```

## Configuration

| Env var | Default | Description |
|---------|---------|-------------|
| `REPO` | — (required) | `owner/repo` to sweep |
| `STALE_DAYS` | 30 | Days idle before the stale label |
| `CLOSE_AFTER_DAYS` | 7 | Days after stale before closing |
| `SWEEP_INTERVAL_HOURS` | 12 | How often to check |

## Notes

- No webhook needed — just needs network access to api.github.com.
- Exempt pinned/high-priority items by keeping them active, or extend the bot to skip a label like `pinned`.
