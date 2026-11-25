# Contributing

Thanks for helping build the smallest useful GitHub bots on the internet.

## Ground rules

- **Stdlib only.** Bots must compile with zero external dependencies — that's
  what keeps binaries at ~5MB and memory at ~10MB. The shared `ghbot` package
  is the only shared code.
- **Small commits.** One logical change per commit. Reference the issue you
  close in the message (`Closes #NN`).
- **gofmt or it didn't happen.** CI enforces gofmt, vet, build, and a boot
  smoke test for every bot.

## Adding a new bot

1. Copy an existing bot that's closest in shape (webhook vs cron).
2. Pick the next free port in the 8101–8199 range and register it in
   `docs/index.html` and the README table.
3. Each bot needs: `main.go` with a doc-comment header, a `README.md` with
   webhook events + permissions, and ideally unit tests for pure logic.
4. Wire it into `.github/workflows/ci.yml` smoke checks happen automatically —
   no extra CI config needed since it loops over `bots/*/`.
5. If it should run on this repo itself, add a job to the matching
   `bots-*.yml` workflow.

## Testing

```sh
go test -race ./...
go build ./...
```

For webhook bots, run locally with `ALLOW_UNSIGNED_WEBHOOKS=true` and curl
payloads at it.

## Reporting bugs

Open an issue with the bot name in the title. Include logs and the event type
that triggered it.
