# ADR 0003: Bot state lives in small JSON files, not a database

## Status: accepted

## Context

stale-bot, flaky-test-tracker and repo-digest need to remember things across
runs. A database contradicts the tiny-server philosophy.

## Decision

Simple JSON state files (stale-state.json, flaky-state.json) written
atomically enough for single-process use. In Actions mode they ride the
actions/cache.

## Consequences

- No external services required, ever
- Horizontal scaling of cron bots means state divergence — documented as
  "run cron bots as a singleton"; webhook bots are stateless and scale freely
