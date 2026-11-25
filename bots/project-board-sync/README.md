# project-board-sync

Auto-adds new issues/PRs to a Projects v2 board and moves them: assigned → In Progress, closed/merged → Done.

| Env | Default | Purpose |
|---|---|---|
| `PROJECT_NUMBER` | — | projects v2 board number (required) |
| `PROJECT_OWNER` | — | user/org login owning the project (required) |
| `PORT` | 8132 | webhook port |

Events: issues, pull_request · Permissions: issues/PRs read, projects write
