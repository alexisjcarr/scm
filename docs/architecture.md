# Architecture

## Components

- `scmctl` loads a local YAML manifest, validates it locally, submits the compiled manifest to `scmctld`, and can stream apply events.
- `scmctld` stores agents, applies, work items, and event history in SQLite. It exposes gRPC for agents and the CLI, plus a small read-only HTTP UI.
- `scmctld-agent` registers itself, heartbeats periodically, fetches work only when idle, persists manifests locally, and reconciles package, file, and service resources.

## Domain boundaries

- `internal/manifest`: YAML DSL, validation, dependency graphing, and compiled transport shape.
- `internal/controlplane`: the facade layer for the control plane, with smaller subdomains for:
  - `inventory`: agent registration, heartbeats, and target resolution
  - `apply`: apply submission, apply read models, and apply status aggregation
  - `workqueue`: lease-based work claiming and work state transitions
- `internal/agent`: agent orchestration, with `internal/agent/runtime` owning local work persistence and idempotent resource reconciliation.
- `internal/platform`: shared config, logging, metrics, gRPC, clock, and version helpers.

## Work dispatch model

This MVP uses an agent pull model.

1. Agents register on startup.
2. Agents heartbeat to update liveness and current work.
3. Idle agents call `FetchWork`.
4. `scmctld` claims one pending work item transactionally in SQLite and returns a lease token.
5. The agent reports `running`, then `completed` or `failed`, along with event records.

The control plane does not push work directly to agents.

## Storage

- Control plane state: SQLite database with `agents`, `applies`, `work_items`, and `apply_events`.
- Agent local state: SQLite database with persisted work metadata.
- Agent manifest cache: JSON manifest payloads stored under the configured cache directory.

## UI

The control plane UI is server-rendered with Go templates and embedded into the daemon binary. It provides:

- agent inventory with current heartbeat freshness
- apply list
- apply detail page with per-host status and event history
