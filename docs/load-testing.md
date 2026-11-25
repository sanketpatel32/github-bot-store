# Load testing and resource budgets

Measured 2026-11-25 with `tools/loadtest.sh` against pr-size-labeler
(dummy token, so GitHub API calls fail fast with 401s — worst case for the
server since handlers still do full network roundtrips).

## Measured numbers

| Metric | Value |
|---|---|
| Webhook intake throughput | **~1370 req/sec** |
| Saturation behavior | 429 backpressure (GitHub retries) — never collapse |
| Memory under load | flat — bounded worker pool (8 workers, queue 64) |

## Regression budgets — file an issue if a bot exceeds these

| Resource | Budget |
|---|---|
| Idle RSS | 8MB |
| RSS under 500req burst | 25MB |
| Binary size | 8MB |
| Webhook ack latency p99 | 50ms |
| Intake throughput | 500 req/sec |

The 429s in load tests are a feature: when saturated, the bot tells GitHub
to retry later instead of OOMing. With SPOOL_DIR set, accepted events also
survive a crash, so the system as a whole doesn't lose work under load.

Reproduce: `tools/loadtest.sh [port] [requests] [concurrency]`.
