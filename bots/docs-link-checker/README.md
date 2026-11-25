# docs-link-checker

Scans all markdown files for dead external links. One managed issue lists them, auto-updates, auto-closes when fixed.

| Env | Default | Purpose |
|---|---|---|
| `REPO` | — | owner/repo (required) |
| `BRANCH` | main | branch to scan |
| `PORT` | 8125 | health endpoint |
