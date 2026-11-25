# branch-sweeper

Deletes the head branch automatically when a PR is merged. Keeps your branch list clean without relying on contributors clicking "delete branch".

## Setup

```bash
go build -o branch-sweeper ./bots/branch-sweeper
export GITHUB_TOKEN=... WEBHOOK_SECRET=...
./branch-sweeper   # listens on :8104
```

## Webhook

- URL: `http://your-server:8104/webhook`
- Events: **Pull requests**
- Permissions: `contents: write`

## Safety rails

- Never touches fork branches
- Never touches `main`, `master`, `develop`, `dev`, `release`, `staging`, `production`, or the repo's default branch
- Only fires on merged PRs, not merely closed ones
