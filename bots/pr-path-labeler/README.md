# pr-path-labeler

Labels PRs by touched directories — uses explicit `path-labels.json` mappings or the `area/<top-dir>` convention. Only applies labels that already exist in the repo; never removes human-applied labels.

| Env | Default | Purpose |
|---|---|---|
| `PORT` | 8121 | webhook port |

Events: pull_request · Permissions: pulls: read, issues: write
