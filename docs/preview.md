# Offline preview — try a bot without GitHub credentials

One command, zero tokens, zero network side effects:

```sh
tools/preview.sh bug-report-issue keyword-issue-labeler
tools/preview.sh large-pr pr-size-labeler
```

What happens:

1. **fakeapi** (`tools/fakeapi`) starts — a local stand-in for the GitHub
   REST + GraphQL API. It serves canned reads from the scenario's
   `responses.json` and records every mutation.
2. The bot starts with `GITHUB_API_URL` pointed at the fake — the same
   override GitHub Enterprise users rely on, so **no request can reach
   github.com** in preview mode. If the fake is down, calls simply fail.
3. The scenario webhook is replayed at the bot with `tools/replay`.
4. The transcript prints every mutation the bot would have made — labels,
   comments, statuses — with bodies, plus the reasoning visible in the
   bot's own logs.

## Scenarios

- `bug-report-issue` — an issue full of crash keywords;
  keyword-issue-labeler creates and applies the `bug` label
- `large-pr` — a PR with 8 files and ~1200 lines changed;
  pr-size-labeler applies `size/XL`

## Adding your own scenario

```
fixtures/<name>/payload.json     # the webhook payload, sanitized
fixtures/<name>/responses.json   # "METHOD /path" → canned JSON response
```

Then `tools/preview.sh <name> <bot>`. Unknown API reads return empty
arrays, so a bot that can't find its data simply decides no action is
needed — which is itself a valid preview outcome.
