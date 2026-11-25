# release-reminder

Opens/updates a tracking issue when unreleased commits pile up past a threshold. Closes itself when you release or drop below threshold.

| Env | Default | Purpose |
|---|---|---|
| `REPO` | — | owner/repo (required) |
| `THRESHOLD_COMMITS` | 20 | unreleased commits that trigger the reminder |
| `PORT` | 8124 | health endpoint |
