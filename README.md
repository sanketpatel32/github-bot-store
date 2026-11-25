# GitHub Bot Store

A collection of small, self-hosted GitHub webhook bots written in Go. Each compiles to a **single static binary** (~5 MB, ~10 MB RAM) — built to run comfortably on the tiniest VPS, a Raspberry Pi, or a free-tier container.

**[Browse the Bot Store](https://sanketpatel32.github.io/github-bot-store/)** — full catalog with setup guides.

## The Bots

| Bot | What it does |
|-----|--------------|
| [pr-size-labeler](bots/pr-size-labeler) | When someone opens a pull request, this bot looks at how much code changed and sticks a size label on it — XS for tiny fixes, XL for monsters. Giant PRs are hard to review, so it gently nudges the author to split them. | webhook |
| [first-touch-welcomer](bots/first-touch-welcomer) | The first time someone opens an issue or pull request in your project, this bot posts a friendly welcome comment. New contributors feel noticed instead of shouting into the void. | webhook |
| [stale-bot](bots/stale-bot) | Hunts down issues and pull requests nobody has touched in 30 days, marks them as stale, and closes them a week later if nobody responds. Any comment resets the clock. | cron |
| [branch-sweeper](bots/branch-sweeper) | Deletes branches that were merged months ago and are just cluttering up the branch list. It double-checks before deleting anything, and never touches protected branches. | webhook |
| [conventional-pr-check](bots/conventional-pr-check) | Makes sure every pull request title follows the conventional format like 'fix: crash on startup' or 'feat: dark mode'. Keeps your history readable and your changelogs automatic. | webhook |
| [reviewer-assigner](bots/reviewer-assigner) | Looks at which files a pull request touches and assigns the right reviewers automatically — your own mini-CODEOWNERS, no GitHub Team plan needed. | webhook |
| [duplicate-issue-hunter](bots/duplicate-issue-hunter) | When a new issue comes in, this bot searches existing ones and comments if it finds likely duplicates. Saves maintainers from answering the same bug report five times. | webhook |
| [todo-tracker](bots/todo-tracker) | Watches for new TODO and FIXME comments in the code and opens a tracking issue for each one, so they don't rot in the codebase forgotten. Closes the issue when the TODO is removed. | webhook |
| [merge-conflict-detector](bots/merge-conflict-detector) | Warns you the moment a pull request can no longer merge cleanly because the main branch moved on. Flips the warning to resolved when you fix it. | webhook |
| [auto-merger](bots/auto-merger) | Merges pull requests automatically once they're approved and all checks pass. No more sitting around waiting to click the green button. | webhook |
| [repo-digest](bots/repo-digest) | Posts a weekly summary issue: what got merged, what opened, what's still waiting. Like a newspaper for your repository. | cron |
| [pr-template-checker](bots/pr-template-checker) | Checks that pull request authors actually filled in the template instead of leaving the placeholder text. Reminds them politely if they didn't. | webhook |
| [release-drafter](bots/release-drafter) | Keeps a draft release up to date as pull requests merge, grouped by change type. When it's time to ship, your release notes are already written. | webhook |
| [keyword-issue-labeler](bots/keyword-issue-labeler) | Reads new issues and labels them based on what they mention — 'crash' gets the bug label, 'feature request' gets enhancement. Instant triage with zero effort. | webhook |
| [ci-failure-commenter](bots/ci-failure-commenter) | When CI fails on a pull request, this bot posts a compact comment explaining what broke, with links to the logs. Updates the same comment when it goes green again. | webhook |
| [pr-aging-bot](bots/pr-aging-bot) | Flags pull requests that have been sitting open too long without review. Old PRs die in silence; this makes them visible. | cron |
| [review-reminder](bots/review-reminder) | Nudges reviewers who were asked to review but haven't responded. Escalates politely as days pass. | cron |
| [issue-triage-reminder](bots/issue-triage-reminder) | Reminds maintainers about new issues that nobody has labeled or answered. Nothing slips through the cracks. | cron |
| [changelog-checker](bots/changelog-checker) | Reminds authors to update the changelog when their pull request changes user-facing behavior. No more forgotten entries at release time. | webhook |
| [license-header-checker](bots/license-header-checker) | Checks that new source files carry the project's license header. Keeps you compliant without manual review. | webhook |
| [pr-path-labeler](bots/pr-path-labeler) | Labels pull requests by which part of the codebase they touch — frontend, backend, docs — so you can filter work by area. | webhook |
| [contributor-milestones](bots/contributor-milestones) | Celebrates contributors when they hit milestones — first PR merged, tenth, fiftieth. Small celebrations keep people coming back. | webhook |
| [breaking-change-guard](bots/breaking-change-guard) | Flags pull requests that look like they break existing behavior, so they get extra scrutiny before merging. | webhook |
| [release-reminder](bots/release-reminder) | Nags you when lots of unreleased work has piled up on the main branch. A gentle push to actually ship. | cron |
| [docs-link-checker](bots/docs-link-checker) | Checks links in your documentation and README, and reports the dead ones. Nothing embarrasses like a 404 in the getting-started guide. | cron |
| [issue-link-checker](bots/issue-link-checker) | Verifies links people paste into issues actually work. Dead reproduction links get flagged automatically. | webhook |
| [issue-form-validator](bots/issue-form-validator) | Makes sure new issues follow your template — a bug report without steps to reproduce gets a friendly request to fill it in properly. | webhook |
| [dependency-update-grouper](bots/dependency-update-grouper) | Groups the flood of dependabot pull requests into batches so you review them once a week instead of every single day. | webhook |
| [flaky-test-tracker](bots/flaky-test-tracker) | Notices when the same test fails intermittently across runs and files a tracking issue. Flaky tests stop being invisible. | webhook |
| [merge-queue-assistant](bots/merge-queue-assistant) | Shows which pull request is next in line to merge, labeling the queue so everyone knows the order. | webhook |
| [discussion-triage-bot](bots/discussion-triage-bot) | Flags discussions that have been sitting unanswered. Questions from your community deserve replies. | cron |
| [project-board-sync](bots/project-board-sync) | Keeps your GitHub project board in sync automatically — new issues and PRs land on the board, status follows the work. | webhook |

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
