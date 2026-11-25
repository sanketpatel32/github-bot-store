# merge-conflict-detector

When the default branch moves, checks every open PR and comments on the ones that now conflict — no more surprise red merge buttons.

## Setup

```bash
go build -o merge-conflict-detector ./bots/merge-conflict-detector
export GITHUB_TOKEN=... WEBHOOK_SECRET=...
./merge-conflict-detector   # listens on :8109
```

## Webhook

- URL: `http://your-server:8109/webhook`
- Events: **Push**
- Permissions: `pull-requests: read`, `issues: write` (PR comments are issue comments)

## Notes

- GitHub computes mergeability asynchronously; the bot retries once after 3s per PR.
- Only scans pushes to the default branch.
