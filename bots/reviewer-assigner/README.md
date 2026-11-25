# reviewer-assigner

Auto-assigns reviewers to PRs based on which paths changed — a lightweight, self-hosted alternative to CODEOWNERS auto-assignment.

## Setup

Create a `reviewers.json` in the bot's working directory:

```json
{
  "src/frontend/": ["alice"],
  "src/backend/": ["bob", "carol"],
  "docs/": ["dave"],
  "": ["fallback-dev"]
}
```

Keys are path prefixes — the **longest matching prefix** wins per file. The empty string `""` is the fallback for anything unmatched.

```bash
go build -o reviewer-assigner ./bots/reviewer-assigner
export GITHUB_TOKEN=... WEBHOOK_SECRET=...
./reviewer-assigner   # listens on :8106
```

## Webhook

- URL: `http://your-server:8106/webhook`
- Events: **Pull requests**
- Permissions: `pull-requests: write`

## Notes

- Never assigns the PR author as their own reviewer.
- Re-runs on new pushes, so late-added files still pull in the right people.
