# GitHub Bot Store

A collection of small, self-hosted GitHub webhook bots written in Go. Each compiles to a **single static binary** (~5 MB, ~10 MB RAM) — built to run comfortably on the tiniest VPS, a Raspberry Pi, or a free-tier container.

**[Browse the Bot Store](https://sanketpatel32.github.io/github-bot-store/)** — full catalog with setup guides.

## The Bots

| Bot | What it does |
|-----|--------------|
| [pr-size-labeler](bots/pr-size-labeler) | Labels PRs XS–XL by lines changed |
| [first-touch-welcomer](bots/first-touch-welcomer) | Welcomes first-time contributors |
| [stale-bot](bots/stale-bot) | Marks and closes stale issues/PRs |
| [branch-sweeper](bots/branch-sweeper) | Deletes branches after PR merge |
| [conventional-pr-check](bots/conventional-pr-check) | Status check enforcing conventional PR titles |
| [reviewer-assigner](bots/reviewer-assigner) | Auto-assigns reviewers by changed paths |
| [duplicate-issue-hunter](bots/duplicate-issue-hunter) | Flags possible duplicate issues |
| [todo-tracker](bots/todo-tracker) | Opens issues for new TODO comments in pushed code |
| [merge-conflict-detector](bots/merge-conflict-detector) | Warns when a PR becomes unmergeable |
| [auto-merger](bots/auto-merger) | Merges labeled PRs once checks go green |
| [repo-digest](bots/repo-digest) | Weekly digest issue of repo activity |
| [pr-template-checker](bots/pr-template-checker) | Nudges PRs missing template sections |
| [release-drafter](bots/release-drafter) | Running draft release from merged PRs |
| [keyword-issue-labeler](bots/keyword-issue-labeler) | Auto-labels issues by content keywords |
| [ci-failure-commenter](bots/ci-failure-commenter) | Comments failing checks with log links |
| [pr-aging-bot](bots/pr-aging-bot) | Labels PRs by wait time — fresh/week/stale/ancient | cron |
| [review-reminder](bots/review-reminder) | Nudges reviewers when PRs wait too long | cron |
| [issue-triage-reminder](bots/issue-triage-reminder) | Flags untriaged issues with a self-clearing label | cron |
| [changelog-checker](bots/changelog-checker) | Commit status enforcing changelog updates | webhook |
| [license-header-checker](bots/license-header-checker) | Enforces license headers on new files | webhook |
| [pr-path-labeler](bots/pr-path-labeler) | Labels PRs by touched directories | webhook |
| [contributor-milestones](bots/contributor-milestones) | Celebrates merged-PR milestones | webhook |
| [breaking-change-guard](bots/breaking-change-guard) | Requires acknowledgment of breaking changes | webhook |
| [release-reminder](bots/release-reminder) | Reminds you when unreleased commits pile up | cron |
| [docs-link-checker](bots/docs-link-checker) | Finds dead links in markdown docs | cron |

## Automations

- **CI** (`.github/workflows/ci.yml`) — gofmt, vet, builds all 15 bots, and smoke-tests each binary's `/healthz` on every push
- **Release** (`.github/workflows/release.yml`) — push a `v*` tag and every bot is cross-compiled for linux/amd64 + arm64 and attached to the release, so your server never needs Go installed

## Why Go?

- **Tiny footprint** — static binaries, no runtime, no dependency tree on the server
- **Low RAM** — each bot idles at ~10 MB; run all 12 in under 150 MB
- **One file to deploy** — `scp` the binary, run it, done

## Quick Start

```bash
# 1. Build any bot (Go 1.22+)
go build -o pr-size-labeler ./bots/pr-size-labeler

# 2. Configure
cp .env.example .env   # add your token + webhook secret
export $(grep -v '^#' .env | xargs)

# 3. Run
./pr-size-labeler     # listens on its default port
```

Then point a GitHub webhook (repo → Settings → Webhooks, or a GitHub App) at
`http://your-server:<port>/webhook` with content type `application/json`.

## Getting a Token

A classic PAT with `repo` scope works for most bots:
GitHub → Settings → Developer settings → Personal access tokens. Each bot's README
lists the exact permissions/events it needs.

## Repository Layout

```
bots/            one folder per bot, each a standalone main.go
shared/ghbot/    shared webhook + GitHub API helpers (stdlib only)
docs/            the GitHub Pages site
.env.example     template for token, secret, ports
```
