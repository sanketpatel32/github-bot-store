# issue-triage-reminder

Marks issues `needs-triage` when they sit unlabeled/unassigned past a grace period; removes the mark automatically once a maintainer triages them.

| Env | Default | Purpose |
|---|---|---|
| `REPO` | — | owner/repo (required) |
| `GRACE_HOURS` | 24 | grace period before marking |
| `PORT` | 8118 | health endpoint |
