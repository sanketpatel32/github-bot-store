# auto-merger

Merges any PR labeled `automerge` as soon as its checks go green — squash-merges and moves on. (GitHub now has native auto-merge, but this works on repos/plans where it's unavailable and gives you a label-driven flow.)

## Setup

```bash
go build -o auto-merger ./bots/auto-merger
export GITHUB_TOKEN=... WEBHOOK_SECRET=...
./auto-merger   # listens on :8110
```

Create the `automerge` label in your repo.

## Webhook

- URL: `http://your-server:8110/webhook`
- Events: **Pull requests**, **Check runs**, **Check suites**, **Status**
- Permissions: `pull-requests: write`, `contents: write`, `checks: read`, `statuses: read`

## Notes

- Repos with no CI count as green immediately.
- Merge method is squash — edit `main.go` if you prefer merge commits.
