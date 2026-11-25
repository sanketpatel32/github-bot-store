# breaking-change-guard

Detects conventional-commit breaking markers (`feat!:`, `BREAKING CHANGE:`) and blocks merge until a maintainer applies the breaking-change label.

| Env | Default | Purpose |
|---|---|---|
| `PORT` | 8123 | webhook port |

Events: pull_request · Permissions: statuses: write, pulls: read, issues: write
