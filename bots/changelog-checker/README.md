# changelog-checker

Commit status that fails unless the PR touches CHANGELOG.md or carries the skip-changelog label.

| Env | Default | Purpose |
|---|---|---|
| `CHANGELOG_FILE` | CHANGELOG.md | file to look for |
| `SKIP_LABEL` | skip-changelog | label that bypasses the check |
| `PORT` | 8119 | webhook port |

Events: pull_request · Permissions: statuses: write, pulls: read
