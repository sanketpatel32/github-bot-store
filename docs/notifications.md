# Outbound notifications

Any bot can mirror its important actions to Slack, Discord, or any webhook
endpoint. Set one env var:

```sh
NOTIFY_WEBHOOK_URL=https://hooks.slack.com/services/T.../B.../xxx
# or a discord webhook — both payload shapes are sent, both platforms accept
```

`ghbot.Notify(bot, message)` is best-effort: failures are logged, never
fatal. Currently wired:

- **auto-merger** — every successful merge
- **stale-bot** — every auto-close

Adding it to another bot is one line where the action happens. It's
deliberately opt-in per deployment — noisy bots are worse than quiet ones.
