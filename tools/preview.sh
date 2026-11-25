#!/bin/sh
# preview.sh — try a bot offline against a canned scenario, no GitHub
# credentials needed. Every API call goes to the local fake; nothing can
# reach github.com because GITHUB_API_URL is overridden for the bot process.
#
# usage: tools/preview.sh <scenario> <bot>
#   tools/preview.sh bug-report-issue keyword-issue-labeler
#   tools/preview.sh large-pr pr-size-labeler
set -e
scenario="${1:-bug-report-issue}"
bot="${2:-keyword-issue-labeler}"
root=$(cd "$(dirname "$0")/.." && pwd)

if [ ! -d "$root/fixtures/$scenario" ]; then
  echo "unknown scenario — available:"
  ls "$root/fixtures"
  exit 1
fi

echo "== starting fake github api"
FIXTURES="$root/fixtures/$scenario" PORT=9911 go run "$root/tools/fakeapi" &
FAKE=$!
sleep 1

echo "== starting $bot (all API traffic redirected to the fake)"
cd "$root"
GITHUB_TOKEN=preview-token \
GITHUB_API_URL=http://localhost:9911 \
ALLOW_UNSIGNED_WEBHOOKS=true \
PORT=18101 \
REPO=example/demo \
go run "./bots/$bot" &
BOTPID=$!
sleep 2

echo "== replaying scenario webhook"
event="issues"
case "$bot" in
  pr-*|auto-merger|reviewer-assigner|conventional-pr-check|changelog-checker|license-header-checker|breaking-change-guard|merge-queue-assistant|pr-template-checker) event="pull_request" ;;
esac
go run ./tools/replay -event "$event" -target localhost:18101 "fixtures/$scenario/payload.json"
sleep 2

echo ""
echo "== transcript: what the bot would have done to github.com"
curl -s http://localhost:9911/transcript

kill $FAKE $BOTPID 2>/dev/null
echo ""
echo "== preview complete — nothing touched the real github.com"
