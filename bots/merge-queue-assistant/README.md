# merge-queue-assistant

A minimal merge queue: label PRs `queue/merge` and the bot maintains FIFO order — the head gets `queue/next` plus a comment; everyone else waits. When the head merges or closes, the next PR is promoted automatically.

| Env | Default | Purpose |
|---|---|---|
| `PORT` | 8130 | webhook port |

Events: pull_request · Permissions: pulls: read/write, issues: write
