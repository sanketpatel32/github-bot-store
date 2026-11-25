# replay

Replays a captured webhook payload against a locally running bot.

```sh
# capture a payload from GitHub: repo settings -> webhooks -> recent deliveries -> redeliver panel
# sanitize it (strip anything sensitive), save as payload.json, then:
GITHUB_TOKEN=... ALLOW_UNSIGNED_WEBHOOKS=true ./pr-size-labeler &   # in one shell
go run ./tools/replay -event pull_request -target localhost:8101 payload.json
```

Signs requests with WEBHOOK_SECRET when set, exactly like GitHub — so signature verification logic is exercised too.
