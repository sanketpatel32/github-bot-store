# ADR 0001: Standard library only, forever

## Status: accepted

## Context

Bots need to run on the smallest possible servers. Every dependency adds
binary size, supply-chain risk, and maintenance surface.

## Decision

No third-party Go dependencies. Ever. The GitHub REST API is JSON over
HTTPS — net/http handles it. HMAC webhook verification is crypto/hmac.
Nothing missing is worth a dependency.

## Consequences

- Binaries ~5.7MB, ~6MB RAM idle
- govulncheck almost never fires; SBOM is one line
- Some things are handwritten that libraries would give free (pagination,
  retry) — accepted cost, they're ~50 lines total in shared/ghbot
- GitHub App JWT signing is implemented manually with crypto/rsa (~40 lines)
