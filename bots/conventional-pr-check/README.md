# conventional-pr-check

Posts a commit status (`conventional-pr-title`) that passes only when the PR title follows [conventional commits](https://www.conventionalcommits.org/):

```
feat: add login page
fix(api): handle nil token
chore(deps)!: drop support for node 14
```

## Setup

```bash
go build -o conventional-pr-check ./bots/conventional-pr-check
export GITHUB_TOKEN=... WEBHOOK_SECRET=...
./conventional-pr-check   # listens on :8105
```

## Webhook

- URL: `http://your-server:8105/webhook`
- Events: **Pull requests**
- Permissions: `statuses: write` (commit statuses), `pull-requests: write`

## Notes

- Reacts to open, edit, push, and reopen — fix the title and the status updates.
- Accepted types: `feat fix chore docs refactor test perf ci build style revert`, optional `(scope)` and `!`.
- Make it a required status check in branch protection to actually block merges.
