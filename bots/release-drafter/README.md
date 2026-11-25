# release-drafter

Maintains a running **draft GitHub release**: every merged PR appends its title to the notes, grouped under headings by conventional-commit prefix (Features, Bug Fixes, Performance…). When you're ready to ship, just publish the draft.

## Setup

```bash
go build -o release-drafter ./bots/release-drafter
export GITHUB_TOKEN=... WEBHOOK_SECRET=...
./release-drafter   # listens on :8113
```

## Webhook

- URL: `http://your-server:8113/webhook`
- Events: **Pull requests**
- Permissions: `contents: write`, `pull-requests: read`

## Notes

- PRs without a conventional prefix land in "Other" — pair with conventional-pr-check.
- Idempotent: re-delivered webhooks don't duplicate entries.
- Combine with [release automation](https://github.com/sanketpatel32/github-bot-store/actions) — tag the draft and binaries build themselves.
