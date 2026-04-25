# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
mise run dev          # Run locally with live reload (watchexec)
mise run go-run       # Run locally without reload
mise run go-run-demo  # Run without Kubernetes (demo mode, KUBECONFIG=)
mise run build        # Build binary to bin/gohome
mise run test         # go test -v ./...
mise run fmt          # go fmt ./...
mise run vet          # go vet ./...
mise run release      # Cut a CalVer release via annover (pushes tag → triggers CI)
prek run -a           # Run all pre-commit hooks across all files
```

Single test: `go test -v -run TestName ./internal/...`

## Architecture

GoHome is a Kubernetes homepage dashboard with a dual-server architecture: a local HTTP server (PORT, default 8080) and a Tailscale tsnet HTTPS server (TSNET_ADDR, default :443) share the same handler and metrics.

### Request flow

```
Browser/Tailscale → tsnet (:443) or local (:8080)
                  → resolveViewer (Tailscale identity via WhoIs or header)
                  → Prometheus middleware
                  → getData() → k8s.go (ingresses + configmap) + config.go (bookmarks)
                  → templates/index.html
```

### Key files

- `cmd/main.go` — entry point; reads env vars, starts both servers, initialises tsnet
- `internal/server.go` — HTTP handler, routing, Tailscale identity resolution, template rendering
- `internal/k8s.go` — Kubernetes client; ingress discovery and classification
- `internal/config.go` — ConfigMap-based bookmark parsing
- `templates/index.html` — single Go template; apps/services/bookmarks sections
- `static/style.css` — dark monospaced theme (JetBrains Mono, cyan/purple accents)

### Kubernetes integration

Reads ingresses across all namespaces and classifies them:
- `gohome.stringer.sh/app: "true"` → Apps section
- `gohome.stringer.sh/hide: "true"` → Hidden
- `gohome.stringer.sh/name: "..."` → Override display name
- `ingressClassName: tailscale` → hostname read from LoadBalancer status, tsnet/Funnel badge shown

Falls back to in-cluster config, then kubeconfig, then **demo mode** (hardcoded ingresses) if neither is available.

### Tailscale/tsnet

The tsnet server creates a persistent Tailscale node. `TS_STATE_DIR` must be a persistent volume in Kubernetes — without it each restart generates a new node with a numeric suffix. The k8s manifests mount a PVC at this path.

Viewer identity is resolved two ways: via the `Tailscale-User-Login` header (when behind Tailscale Serve/proxy) or via `LocalClient.WhoIs(remoteAddr)` when connecting directly through tsnet.

### Versioning

CalVer (`YYYY.N`). `mise run release` calls `annover bump` which creates the tag and GitHub release. The tag push triggers `.github/workflows/release.yml`, which builds multi-arch binaries and Docker image, uploads assets, then uses annover deploy to update the GitOps repo (`joeds13/turingpi_talos_cluster`, `gohome/kustomization.yaml`). Version is injected at build time via `-ldflags "-X main.Version=..."`.

## Environment variables

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | Local HTTP listener |
| `NAMESPACE` | `default` | K8s namespace to watch |
| `CONFIG_MAP_NAME` | `gohome-config` | ConfigMap for bookmarks/title |
| `TSNET_HOSTNAME` | `gohome` | Tailscale node name |
| `TSNET_ADDR` | `:443` | tsnet listener address |
| `TS_STATE_DIR` | — | Persistent tsnet state directory |
| `TS_AUTHKEY` | — | Tailscale auth key for headless operation |
| `PAGE_TITLE` | — | Override page title (highest priority) |
