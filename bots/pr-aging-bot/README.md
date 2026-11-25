# pr-aging-bot

Labels open PRs by wait time — `aging/fresh` (<7d), `aging/week`, `aging/stale` (14d+), `aging/ancient` (30d+). Makes review backlogs visible.

## Config

| Env | Default | Purpose |
|---|---|---|
| `REPO` | — | owner/repo to sweep (required) |
| `SKIP_DRAFTS` | true | skip draft PRs |
| `PORT` | 8116 | health endpoint |

Cron it or run as a scheduled GitHub Action.
