# Security Policy

## Reporting a vulnerability

Email sanpatel323@gmail.com or open a private security advisory on GitHub —
please do not open public issues for vulnerabilities.

## Scope notes

These bots handle GitHub webhooks and tokens. In particular:

- Always set `WEBHOOK_SECRET` in production; the bots refuse to serve webhooks
  without it unless `ALLOW_UNSIGNED_WEBHOOKS=true` is explicitly set.
- Tokens are read from the environment only and are never logged.
- The webhook server enforces a 1MB payload limit and strict timeouts.
- Binaries ship with no runtime dependencies; verify downloads with the
  `checksums.txt` published on each release.

## Supported versions

Latest `main` and the most recent tagged release only.
