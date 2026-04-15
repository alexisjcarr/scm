# scm

`scm` is a small but maintainable host configuration management MVP written in Go.
It ships three binaries:

- `scmctl`: validates and submits manifests
- `scmctld`: control plane daemon
- `scmctld-agent`: host agent daemon

The repository is structured so an reader can explore a real codebase rather
than a toy script:

- domain boundaries for manifest parsing, control plane orchestration, and agent reconciliation
- explicit control-plane subdomains for inventory, applies, and work queue behavior
- gRPC transport between components
- SQLite-backed state with replaceable repository interfaces
- Prometheus metrics from both daemons
- server-rendered operational UI
- Ubuntu packaging and systemd install assets

## Quick start

1. `make test`
2. For a same-machine dev loop, start `scmctld` with the example config in `/Users/alexisjcarr/learning/scm/configs/examples/scmctld.yaml`
3. Start `scmctld-agent` with `/Users/alexisjcarr/learning/scm/configs/examples/scmctld-agent.yaml`
4. Run:

```bash
go run ./cmd/scmctl validate -f ./examples/manifests/nginx.yaml
go run ./cmd/scmctl apply -f ./examples/manifests/nginx.yaml --watch
```

For this project topology, run `scmctld` locally with Docker Compose and point remote remote agents at the exposed gRPC address or tunnel endpoint described in `docs/setup.md`.

See `/Users/alexisjcarr/learning/scm/docs` for architecture, DSL, install, development, and project deployment details.
