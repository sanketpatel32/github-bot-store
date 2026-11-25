# pr-template-checker

Nudges authors when a PR description is empty or missing the sections your template expects.

## Setup

```bash
go build -o pr-template-checker ./bots/pr-template-checker
export GITHUB_TOKEN=... WEBHOOK_SECRET=...
./pr-template-checker   # listens on :8112
```

## Configuration

| Env var | Default | Description |
|---------|---------|-------------|
| `REQUIRED_SECTIONS` | `summary,testing` | Comma-separated keywords that must appear in the PR body (case-insensitive substring match) |

Example: `REQUIRED_SECTIONS="summary,test plan,screenshots"`

## Webhook

- URL: `http://your-server:8112/webhook`
- Events: **Pull requests**
- Permissions: `issues: write`

## Notes

- Re-checks on edit — fill in the sections and the bot goes quiet.
- Matches substrings, so `## Summary` satisfies `summary`.
