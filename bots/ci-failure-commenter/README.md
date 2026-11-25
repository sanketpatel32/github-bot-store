# ci-failure-commenter

When CI fails on a PR, comments exactly which checks failed with links to their logs — no more digging through the checks tab. Repeat failures **update the same comment** instead of spamming new ones.

## Setup

```bash
go build -o ci-failure-commenter ./bots/ci-failure-commenter
export GITHUB_TOKEN=... WEBHOOK_SECRET=...
./ci-failure-commenter   # listens on :8115
```

## Webhook

- URL: `http://your-server:8115/webhook`
- Events: **Check runs**, **Check suites**
- Permissions: `checks: read`, `issues: write`

## Notes

- Fires on `failure`, `timed_out`, and `action_required` conclusions.
- When the same PR later goes green the comment stays as history — edit `main.go` to delete it if you prefer silence.
