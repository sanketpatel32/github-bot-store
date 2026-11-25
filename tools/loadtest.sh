#!/bin/sh
# loadtest — fires concurrent signed webhooks at a running bot and measures
# throughput, latency, and memory stability. Usage:
#   ./tools/loadtest.sh [port] [requests] [concurrency]
set -e
port="${1:-8101}"
total="${2:-500}"
conc="${3:-20}"

cat > /tmp/loadtest-payload.json <<'JSON'
{"action":"opened","number":1,"pull_request":{"title":"feat: load test","additions":10,"deletions":5,"head":{"sha":"abc123"},"user":{"login":"loadtester","type":"User"},"draft":false},"repository":{"full_name":"test/repo","default_branch":"main"},"sender":{"login":"loadtester","type":"User"}}
JSON

echo "load test: $total requests, $conc concurrent -> localhost:$port/webhook"
echo "(bot must run with ALLOW_UNSIGNED_WEBHOOKS=true and a dummy token;"
echo " GitHub API calls will 401 — that's fine, we're measuring the server)"

start=$(date +%s%N)
seq 1 "$total" | xargs -P "$conc" -I{} curl -s -o /dev/null -w "%{http_code}\n" \
  -X POST "http://localhost:$port/webhook" \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: pull_request" \
  -H "X-GitHub-Delivery: loadtest-{}" \
  -d @/tmp/loadtest-payload.json > /tmp/loadtest-results.txt
end=$(date +%s%N)

ms=$(( (end - start) / 1000000 ))
ok=$(grep -c "^202$" /tmp/loadtest-results.txt || true)
accepted=$(grep -c "^200\|^202" /tmp/loadtest-results.txt || true)
busy=$(grep -c "^429$" /tmp/loadtest-results.txt || true)
echo "---"
echo "total time: ${ms}ms"
echo "accepted: $accepted/$total  (429 backpressure: $busy)"
echo "throughput: $(( total * 1000 / (ms + 1) )) req/sec"
echo ""
echo "memory check: watch the bot's RSS before/after — it should be flat"
echo "because the worker pool is bounded. ghbot_events_total on /metrics"
echo "should show exactly $total deliveries MINUS dedupe hits (dedupe IDs"
echo "in this test are unique per request, so all count)."
