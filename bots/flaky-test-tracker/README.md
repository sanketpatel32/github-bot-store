# flaky-test-tracker

Detects flaky CI — checks that fail then pass on the same SHA — and opens a tracking issue after FLAKE_THRESHOLD flakes.

| Env | Default | Purpose |
|---|---|---|
| `FLAKE_THRESHOLD` | 3 | flakes before opening an issue |
| `PORT` | 8129 | webhook port |

Events: check_run · Permissions: checks: read, issues: write
