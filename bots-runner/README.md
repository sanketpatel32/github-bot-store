# bots-runner

Optional all-in-one supervisor: run several bots as one managed process.

```sh
go build -o bots-runner ./bots-runner
./bots-runner stale-bot pr-size-labeler auto-merger
```

Features: restart-on-crash with exponential backoff (cap 60s), log prefixing,
single SIGTERM brings everything down. Bot binaries must sit next to the
runner or be in PATH. Not required — every bot still runs standalone.
