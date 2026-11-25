# Support policy

## What we support

- **Latest `main`** — always deployable, CI-gated
- **Most recent tagged release** — binaries + checksums + SBOM

Anything older: upgrade. There's no LTS — the upgrade path is "download the
new binary", since there's no state migration beyond small JSON state files
that stay forward-compatible.

## Supply chain posture

- **Zero runtime dependencies.** Every bot compiles from Go stdlib only —
  the SBOM (`sbom.jsonl` in each release) is therefore trivially short and
  dependency-CVEs essentially can't exist in the binaries.
- **Checksums:** every release ships `checksums.txt` (sha256). Verify before
  deploying: `sha256sum -c checksums.txt --ignore-missing`.
- **CI scanning:** govulncheck runs on every push; dependabot watches
  workflow action versions.

## Go version support

Bots build with the Go version in `go.mod` (1.23+). Newer Go releases are
adopted after they've been out for a month.
