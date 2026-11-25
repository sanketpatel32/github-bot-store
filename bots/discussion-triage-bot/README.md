# discussion-triage-bot

Sweeps repo discussions via GraphQL and maintains one self-updating issue listing questions unanswered past GRACE_DAYS.

| Env | Default | Purpose |
|---|---|---|
| `REPO` | — | owner/repo (required) |
| `GRACE_DAYS` | 3 | days before a silent discussion counts as stale |
| `SWEEP_INTERVAL_HOURS` | 24 | sweep cadence |
| `PORT` | 8131 | health endpoint |
