# license-header-checker

Fails a commit status when newly added source files lack a license header, with one managed comment listing offenders.

| Env | Default | Purpose |
|---|---|---|
| `HEADER_TEXT` | copyright | text the header must contain (case-insensitive) |
| `SOURCE_EXTENSIONS` | .go,.js,.ts,.py,.java,.c,.h,.cpp,.rs,.rb | extensions checked |
| `PORT` | 8120 | webhook port |

Events: pull_request · Permissions: statuses: write, pulls: read, issues: write
