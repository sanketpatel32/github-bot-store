# review-reminder

Nudges requested reviewers when a PR waits too long — one reminder per reviewer per PR.

| Env | Default | Purpose |
|---|---|---|
| `REPO` | — | owner/repo (required) |
| `REMIND_AFTER_HOURS` | 24 | how long before nudging |
| `PORT` | 8117 | health endpoint |
