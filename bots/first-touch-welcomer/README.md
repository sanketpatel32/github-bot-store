# first-touch-welcomer

Posts a friendly welcome comment the first time someone opens an issue or PR in your repo. Skips owners, members, and collaborators.

## Setup

```bash
go build -o first-touch-welcomer ./bots/first-touch-welcomer
export GITHUB_TOKEN=... WEBHOOK_SECRET=...
./first-touch-welcomer   # listens on :8102
```

## Webhook

- URL: `http://your-server:8102/webhook`
- Events: **Issues**, **Pull requests**
- Permissions: `issues: write`, `pull-requests: write`

## Notes

- First-timer detection uses the search API: if the user has at most one item (the one just opened) in this repo, they get welcomed.
