# Permission matrix

What each bot needs. Smallest scope that works — don't grant more.

| Bot | Webhook events | Token permissions |
|---|---|---|
| pr-size-labeler | pull_request | Pull requests: read, Issues: write (labels live on issues API) |
| first-touch-welcomer | issues, pull_request | Issues: write, Pull requests: write |
| stale-bot | none (cron) | Issues: write, Pull requests: write |
| branch-sweeper | pull_request | Pull requests: read, Contents: write (branch deletion) |
| conventional-pr-check | pull_request | Commit statuses: write, Pull requests: read |
| reviewer-assigner | pull_request | Pull requests: write, Contents: read (CODEOWNERS) |
| duplicate-issue-hunter | issues | Issues: write |
| todo-tracker | push | Issues: write, Contents: read |
| merge-conflict-detector | pull_request, push | Pull requests: read/write (comments) |
| auto-merger | pull_request, check_suite | Pull requests: write, Checks: read, Contents: write |
| repo-digest | none (cron) | Issues: write, Pull requests: read |
| pr-template-checker | pull_request | Pull requests: read/write (comments) |
| release-drafter | push, pull_request | Contents: write (releases) |
| keyword-issue-labeler | issues | Issues: write |
| ci-failure-commenter | check_run, check_suite | Checks: read, Pull requests: write |

## Troubleshooting

**"Resource not accessible by integration"** — the GitHub Actions token is
missing a permission. Add it to the workflow's `permissions:` block.

**Bot receives webhook but does nothing** — check in order:
1. Bot logs mention "skipping bot-authored event"? The action that triggered
   it came from another bot — expected, see SKIP_BOT_EVENTS.
2. `ALLOWED_REPOS` set but doesn't include this repo?
3. Event type not in the bot's subscription list (table above)?
4. Signature valid? A mismatch returns 401 in delivery logs on GitHub.

**429 responses in webhook deliveries** — worker pool saturated under burst;
GitHub retries automatically, or scale the bot horizontally (dedupe and
spool make this safe).

**Rate limit errors** — installation tokens get 5k req/hr per installation.
DRY_RUN=true still consumes read quota; use ALLOWED_REPOS to reduce scan scope.
