#!/bin/sh
# reproduces docs/benchmarks.md numbers
set -e
echo "building all bots..."
total=0; count=0
for bot in bots/*/; do
  name=$(basename "$bot")
  CGO_ENABLED=0 go build -ldflags="-s -w" -o /tmp/bench-bin "./$bot" 2>/dev/null || continue
  sz=$(stat -c%s /tmp/bench-bin 2>/dev/null || stat -f%z /tmp/bench-bin)
  echo "$name: $(echo "$sz / 1048576" | bc -l | cut -c1-4)MB"
  total=$((total + sz)); count=$((count + 1))
done
echo "---"
echo "average binary: $(echo "$total / $count / 1048576" | bc -l | cut -c1-4)MB across $count executables"

echo "measuring idle RSS..."
GITHUB_TOKEN=dummy ALLOW_UNSIGNED_WEBHOOKS=true PORT=18999 /tmp/bench-bin >/dev/null 2>&1 &
pid=$!
sleep 1
grep VmRSS /proc/$pid/status 2>/dev/null || echo "(meminfo unavailable on this platform)"
kill $pid 2>/dev/null
rm -f /tmp/bench-bin
