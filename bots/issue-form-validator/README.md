# issue-form-validator

Validates issues against required template sections; labels invalid-issue and posts one managed comment, auto-clears when fixed.

| Env | Default | Purpose |
|---|---|---|
| `REQUIRED_SECTIONS` | what happened,steps to reproduce,expected | comma-separated sections |
| `PORT` | 8127 | webhook port |

Events: issues · Permissions: issues: write
