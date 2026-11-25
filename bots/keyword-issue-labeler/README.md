# keyword-issue-labeler

Auto-labels new issues based on keywords in the title and body. Defaults cover `bug`, `feature`, `docs`, `question`, `performance`, `security` — override with a `labels.json` in the bot's working directory:

```json
{
  "bug": ["bug", "broken", "crash"],
  "area/ui": ["button", "css", "layout", "dark mode"]
}
```

## Setup

```bash
go build -o keyword-issue-labeler ./bots/keyword-issue-labeler
export GITHUB_TOKEN=... WEBHOOK_SECRET=...
./keyword-issue-labeler   # listens on :8114
```

## Webhook

- URL: `http://your-server:8114/webhook`
- Events: **Issues**
- Permissions: `issues: write`

## Notes

- Missing labels are created automatically (gray) — recolor them in repo settings.
- Substring matching, case-insensitive; re-runs on edit.
