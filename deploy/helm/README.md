# Helm deployment

```sh
kubectl create secret generic ghbot \
  --from-literal=token=ghp_xxx --from-literal=webhook-secret=xxx
helm install bots ./deploy/helm/github-bot-store
```

Edit `values.yaml` to pick bots. Images must be built and pushed first —
the chart expects `ghcr.io/sanketpatel32/<bot>` images; build them with
`deploy/Dockerfile` or point the registry elsewhere.

**Replicas are pinned to 1 on purpose** — bots mutate GitHub state and
leaderless horizontal scaling causes duplicate actions. They restart fast
enough that HA comes from quick recovery, not redundancy.
