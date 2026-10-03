# finops

Polls pod CPU/memory usage (metrics-server), converts it to estimated USD using
configurable prices, and scales matching Deployments to 0 when the spend limit
for the budget window is reached. Restores them when a new window starts.

Config is env vars (see `internal/config`), set from `chart/values.yaml`.

    go mod tidy && go build ./...
    helm install finops ./chart --set target.namespace=... --set dryRun=false

Limitations (draft): spend is kept in memory (resets on restart), cost is
usage-based (not requests/PVC), scaling targets Deployments only, and
metrics-server must be installed.
