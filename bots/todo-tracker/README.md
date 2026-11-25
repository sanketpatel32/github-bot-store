# todo-tracker

Scans commits pushed to the default branch for newly added `TODO:` / `FIXME:` comments and opens a labeled tracking issue for each — so TODOs stop disappearing into the code.

## Setup

```bash
go build -o todo-tracker ./bots/todo-tracker
export GITHUB_TOKEN=... WEBHOOK_SECRET=...
./todo-tracker   # listens on :8108
```

## Webhook

- URL: `http://your-server:8108/webhook`
- Events: **Push**
- Permissions: `issues: write`, `contents: read`

## Notes

- Dedupes by a hash of file + text in the issue title — force-pushes and merges don't create duplicates.
- Only scans the default branch.
- Create a `todo` label for nicer filtering (or let it fail silently — labels are optional).
