# ADR 0002: Every bot runs as webhook server AND as GitHub Action

## Status: accepted

## Context

Users without servers can't self-host webhooks; users with servers shouldn't
need Actions minutes. Both audiences matter.

## Decision

The shared lib detects GITHUB_ACTIONS=true and switches Serve() to a
single-run mode reading the event payload from GITHUB_EVENT_PATH. Bot logic
is identical in both modes — the handler doesn't know the difference.

## Consequences

- Zero-server adoption path exists (fork, enable workflows, done)
- Cron bots need REPO set since no webhook payload tells them the repo
- CI smoke tests must clear GITHUB_ACTIONS or bots exit immediately (learned
  the hard way)
