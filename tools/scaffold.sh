#!/bin/sh
# scaffold a new bot: ./tools/scaffold.sh my-bot 8130 webhook|cron
set -e
name="$1"
port="$2"
kind="${3:-webhook}"
if [ -z "$name" ] || [ -z "$port" ]; then
  echo "usage: $0 <bot-name> <port> [webhook|cron]"
  exit 1
fi
if [ -d "bots/$name" ]; then
  echo "bots/$name already exists"
  exit 1
fi
mkdir -p "bots/$name"

if [ "$kind" = "cron" ]; then
cat > "bots/$name/main.go" <<EOF2
// $name — TODO: one-line description.
package main

import (
	"log"
	"os"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

func sweep(client *ghbot.Client, repo string) {
	// TODO: implement the periodic logic
	log.Printf("sweeping %s", repo)
}

func main() {
	cfg := ghbot.LoadConfig("$name", "$port")
	if ghbot.HandleCLIArgs("$name", cfg) {
		return
	}
	client := ghbot.NewClient(cfg.Token)
	repo := os.Getenv("REPO")
	if repo == "" {
		log.Fatal("REPO is not set (owner/repo)")
	}
	sweep(client, repo)
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		return
	}
	ghbot.ServeHealth(cfg, "$name")
}
EOF2
else
cat > "bots/$name/main.go" <<EOF2
// $name — TODO: one-line description.
package main

import (
	"encoding/json"
	"log"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

func main() {
	cfg := ghbot.LoadConfig("$name", "$port")
	if ghbot.HandleCLIArgs("$name", cfg) {
		return
	}
	client := ghbot.NewClient(cfg.Token)
	_ = client

	ghbot.Serve(cfg, "$name", func(event string, payload []byte) {
		// TODO: filter event types, parse payload, act
		var meta struct {
			Action string \`json:"action"\`
		}
		if err := json.Unmarshal(payload, &meta); err != nil {
			return
		}
		log.Printf("received %s/%s", event, meta.Action)
	})
}
EOF2
fi

cat > "bots/$name/README.md" <<EOF2
# $name

TODO: description.

| Env | Default | Purpose |
|---|---|---|
| \`PORT\` | $port | webhook/health port |

TODO: events and permissions.
EOF2

echo "scaffolded bots/$name — now:"
echo "  1. implement the TODOs in bots/$name/main.go"
echo "  2. add an entry to manifest.json and run: go run ./tools/gencatalog"
echo "  3. pick port $port is free? check manifest.json ports"
