# Verifying releases

Every release artifact is checksummed AND keyless-signed with
[cosign](https://docs.sigstore.dev) using GitHub Actions' OIDC identity —
no keys to manage, signatures tied to this repo's CI.

## Verify a download

```sh
# 1. checksums
curl -LO https://github.com/sanketpatel32/github-bot-store/releases/latest/download/checksums.txt
sha256sum -c checksums.txt --ignore-missing

# 2. cosign signature (proves the binary came from this repo's CI)
cosign verify-blob stale-bot-linux-amd64.tar.gz \
  --signature stale-bot-linux-amd64.tar.gz.sig \
  --certificate stale-bot-linux-amd64.tar.gz.pem \
  --certificate-identity-regexp "github-bot-store" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com"
```

Build provenance: artifacts are built ONLY by `.github/workflows/release.yml`
on immutable tags — the workflow is in the repo, so the exact build recipe
is auditable end to end.
