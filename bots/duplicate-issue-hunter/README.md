# duplicate-issue-hunter

When a new issue opens, searches existing issues by title keywords and comments with the closest matches — helps reporters find existing threads before maintainers triage.

## Setup

```bash
go build -o duplicate-issue-hunter ./bots/duplicate-issue-hunter
export GITHUB_TOKEN=... WEBHOOK_SECRET=...
./duplicate-issue-hunter   # listens on :8107
```

## Webhook

- URL: `http://your-server:8107/webhook`
- Events: **Issues**
- Permissions: `issues: write`

## Notes

- Only comments when 2+ keywords overlap an existing issue title — vague issues stay quiet.
- Ignores common stop words and short tokens.
