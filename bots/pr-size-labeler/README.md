# pr-size-labeler

Labels every PR `size/XS` through `size/XL` by total lines changed (additions + deletions), and comments a gentle nudge on XL PRs suggesting a split.

| Size | Lines changed |
|------|---------------|
| XS | < 20 |
| S | < 100 |
| M | < 300 |
| L | < 800 |
| XL | 800+ |

## Setup

```bash
go build -o pr-size-labeler ./bots/pr-size-labeler
export GITHUB_TOKEN=... WEBHOOK_SECRET=...
./pr-size-labeler   # listens on :8101
```

## Webhook

- URL: `http://your-server:8101/webhook`
- Events: **Pull requests**
- Permissions: `pull-requests: write`, `issues: write` (labels live on issues)

## Notes

- Creates the `size/*` labels automatically on first use.
- Labels accumulate; GitHub shows the latest — the bot doesn't remove stale size labels (a PR rarely shrinks a tier after force-push, but if that bothers you, remove labels manually).
