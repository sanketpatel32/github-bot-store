# GitHub Bot Store

A collection of small, self-hosted GitHub webhook bots written in Go. Each compiles to a **single static binary** (~5 MB, ~10 MB RAM) — built to run comfortably on the tiniest VPS, a Raspberry Pi, or a free-tier container.

**[Browse the Bot Store](https://sanketpatel32.github.io/github-bot-store/)** — full catalog with setup guides.

## The Bots

| Bot | What it does |
|-----|--------------|
| [pr-size-labeler](bots/pr-size-labeler) | Labels PRs size/XS through size/XL by lines changed, and nudges authors to split giant PRs. | webhook |
| [first-touch-welcomer](bots/first-touch-welcomer) | Posts a friendly welcome comment the first time someone opens an issue or PR. | webhook |
| [stale-bot](bots/stale-bot) | Marks issues idle 30 days as stale, closes them 7 days later. Any activity resets the clock. | cron |
| [branch-sweeper](bots/branch-sweeper) | Deletes the head branch automatically when a PR is merged. | webhook |
| [conventional-pr-check](bots/conventional-pr-check) | Commit status enforcing conventional-commit PR titles: feat:, fix:, chore: and friends. | webhook |
| [reviewer-assigner](bots/reviewer-assigner) | Auto-assigns reviewers by which paths changed — a self-hosted mini-CODEOWNERS. | webhook |
| [duplicate-issue-hunter](bots/duplicate-issue-hunter) | Comments possible duplicates when a new issue's title overlaps existing ones. | webhook |
| [todo-tracker](bots/todo-tracker) | Opens a tracking issue for every new TODO/FIXME comment pushed to the default branch. | webhook |
| [merge-conflict-detector](bots/merge-conflict-detector) | When the default branch moves, warns open PRs that now conflict. | webhook |
| [auto-merger](bots/auto-merger) | Squash-merges PRs labeled 'automerge' as soon as checks go green. | webhook |
| [repo-digest](bots/repo-digest) | Opens a weekly digest issue: merged PRs, issue stats, top contributors. | cron |
| [pr-template-checker](bots/pr-template-checker) | Nudges authors whose PR description is empty or missing template sections. | webhook |
| [release-drafter](bots/release-drafter) | Maintains a running draft release — every merged PR appends its title to the notes, grouped by type. | webhook |
| [keyword-issue-labeler](bots/keyword-issue-labeler) | Auto-labels new issues from keywords in the title and body — bug, feature, docs, question… | webhook |
| [ci-failure-commenter](bots/ci-failure-commenter) | When CI fails on a PR, comments which checks failed with links — no digging through the checks tab. | webhook |
| [pr-aging-bot](bots/pr-aging-bot) | Labels open PRs by wait time — fresh/week/stale/ancient — so review backlogs are visible at a glance. | cron |
| [review-reminder](bots/review-reminder) | Nudges requested reviewers when a PR waits 24h+ without review — one reminder each, no spam. | cron |
| [issue-triage-reminder](bots/issue-triage-reminder) | Flags issues that sit untriaged — no label, assignee, or milestone — with a needs-triage label that clears itself once triaged. | cron |
| [changelog-checker](bots/changelog-checker) | Commit status enforcing every PR updates the changelog — or explicitly opts out with a skip label. | webhook |
| [license-header-checker](bots/license-header-checker) | Fails PRs whose new source files lack a license header — one managed comment lists exactly which files. | webhook |
| [pr-path-labeler](bots/pr-path-labeler) | Labels PRs by which directories they touch — area/frontend, area/backend — using your repo's existing labels. | webhook |
| [contributor-milestones](bots/contributor-milestones) | Celebrates contributors — welcome comment on first merged PR, shout-outs at 5/10/25/50/100 merges. | webhook |
| [breaking-change-guard](bots/breaking-change-guard) | Blocks merges that declare breaking changes until a maintainer acknowledges with the breaking-change label. | webhook |
| [release-reminder](bots/release-reminder) | Opens a tracking issue when unreleased commits pile up — closes itself once you ship. | cron |
| [docs-link-checker](bots/docs-link-checker) | Scans your markdown for dead links — one self-updating issue lists them, closes itself when fixed. | cron |
| [issue-link-checker](bots/issue-link-checker) | Validates external links in new issues — one comment flags dead repro links before triage wastes time on them. | webhook |
| [issue-form-validator](bots/issue-form-validator) | Enforces your issue template — labels issues missing required sections, auto-clears when they fix it. | webhook |
| [dependency-update-grouper](bots/dependency-update-grouper) | Labels dependabot/renovate PRs patch/minor/major from semver in the title — batch the safe ones, review the rest. | webhook |
| [flaky-test-tracker](bots/flaky-test-tracker) | Catches flaky CI — checks that pass on retry with zero code changes — and files tracking issues. | webhook |
| [merge-queue-assistant](bots/merge-queue-assistant) | A minimal merge queue — label PRs queue/merge and the bot enforces FIFO order with queue/next promotion. | webhook |

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
