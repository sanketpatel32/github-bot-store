# repo-digest

Opens a weekly digest issue summarizing the last 7 days: merged PRs, new/closed issue counts, top contributors.

Cron-style like stale-bot — no webhook needed.

## Setup

```bash
go build -o repo-digest ./bots/repo-digest
export GITHUB_TOKEN=... REPO=owner/repo
./repo-digest   # listens on :8111 (health only)
```

## Configuration

| Env var | Default | Description |
|---------|---------|-------------|
| `REPO` | — (required) | `owner/repo` to summarize |

## Notes

- Posts the first digest immediately at startup, then every 7 days.
- Uses the `digest` label — create it if you want filtering.
