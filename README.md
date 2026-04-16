# Simple Config Manager (scm)

`scm` is a small host configuration management MVP written in Go. It has three binaries:

- `scmctl`: validates and submits manifests
- `scmctld`: control plane daemon
- `scmctld-agent`: per-host reconciliation agent

The project is intentionally shaped like a real service rather than a one-off script: clear domain boundaries, gRPC between components, durable control-plane state, a bounded host-local checkpoint model, a read-only operational UI, Prometheus metrics, and Ubuntu packaging/systemd assets.

## What I Built

The MVP supports:

- host targeting by explicit host ID and exact-match label selectors
- declarative `package`, `file`, and `service` resources
- dependency ordering via `requires`
- change-triggered follow-up behavior via `notifies`
- idempotent reconciliation on the host
- agent-pull work distribution with lease-based claiming
- a small control-plane UI for inventory, apply status, and event history

For this project, I was able to do the following on two separate Ubuntu hosts:

1. `scmctl` submitted a manifest
2. `scmctld` created and assigned work
3. `scmctld-agent` reconciled the host
4. the host served `Hello, world!` over HTTP

## Reviewer Guide

If you only want the fastest path to a working demo, use the packaged Ubuntu flow in [Installation](#installation) and then run `scm-demo`.

If you want to inspect the code locally first:

1. run the unit tests with `make test`
2. start `scmctld` and `scmctld-agent` with the example configs
3. submit one of the example manifests with `scmctl`
4. use the control-plane UI at `http://127.0.0.1:8080` to inspect inventory and apply state

## Architecture

### Operating model

- `scmctl` is the operator-facing submit and validation tool
- `scmctld` is the control plane and system of record
- `scmctld-agent` runs 1:1 on managed hosts and performs reconciliation locally
- SQLite is used for control-plane MVP persistence
- the agent keeps only a bounded JSON checkpoint on disk for crash recovery; the control plane remains the canonical history and journald is the host-local execution log

This is an agent-pull design. The control plane does not SSH into hosts or use stored passwords to perform changes.

### Intended topology

![Architecture sequence diagram](README.assets/architecture-sequence.svg)

PlantUML source: [README.assets/architecture-sequence.puml](README.assets/architecture-sequence.puml)

The intended steady-state deployment model is:

- `scmctld` deployed separately from managed hosts
- durable system state shared by the control plane
- `scmctld-agent` deployed 1:1 on managed hosts
- all components living on the same trusted company network or VPC

I intentionally designed this as a control plane plus pull-based agents rather than a per-host shell script. A simple script-per-box model can be fine for a toy environment, but it does not scale to a large fleet. I wanted the architecture to still make sense in an environment with thousands of hosts, with a central system of record, durable apply history, and per-host reconciliation.

In the project environment, I only had host-level access. I did not have control over network security groups and I did not have a reliable view of the surrounding network topology. I confirmed that the control plane was healthy, bound on `*:8443`, and not blocked by host-local firewalls, while cross-host traffic still timed out over both public and private addresses. That pointed to network policy or routing outside the hosts themselves.

Because of that constraint, I validated the full control-plane-plus-agent path by deploying the full system independently on each host. That was a demo fallback, not the intended production architecture.

### Key implementation choices

- `internal/manifest`: DSL parsing, validation, graph construction, compile step
- `internal/controlplane`: inventory, apply lifecycle, and work queue concerns
- `internal/agent`: registration, polling, reconciliation, and host execution
- `internal/platform`: shared config, logging, metrics, gRPC, clock, version helpers
- `requires` edges are compiled into a DAG and executed in topologically sorted order so reconciliation order is explicit, deterministic, and explainable

Work dispatch is lease-based:

1. agent registers and heartbeats
2. idle agent calls `FetchWork`
3. control plane claims one pending item transactionally in SQLite
4. agent reconciles locally
5. agent reports events and terminal state back to the control plane

## Manifest DSL

Manifests are YAML and can target hosts explicitly or by selector.

```yaml
apiVersion: scm/v1
kind: Manifest
metadata:
  name: php-app-single-host
target:
  hosts:
    - demo-host-1
resources:
  - id: nginx_pkg
    type: package
    name: nginx
    state: installed
  - id: app_index
    type: file
    path: /var/www/scm-php-demo/index.php
    content: |
      <?php
      header("Content-Type: text/plain");
      echo "Hello, world!\n";
      ?>
    mode: "0644"
    owner: www-data
    group: www-data
    state: present
    notifies:
      - php_fpm_svc
  - id: nginx_svc
    type: service
    name: nginx
    state: running
    enabled: true
```

Supported resource types:

- `package`
  - `name`
  - `state: installed|absent`
- `file`
  - `path`, `content`, `mode`
  - optional `owner`, `group`
  - `state: present|absent`
- `service`
  - `name`
  - `state: running|stopped`
  - optional `enabled`

Relationship behavior:

- `requires` defines DAG ordering and is topologically sorted before execution
- `notifies` revisits downstream service resources if an upstream resource changed

Validation guarantees:

- resource IDs are unique
- `requires` and `notifies` references must exist
- `notifies` can only target service resources
- dependency cycles are rejected

Canonical examples:

- [examples/manifests/nginx.yaml](examples/manifests/nginx.yaml)
- [examples/manifests/php-app-single-host.yaml](examples/manifests/php-app-single-host.yaml)
- [examples/manifests/php-app-two-hosts.yaml](examples/manifests/php-app-two-hosts.yaml)

## Installation

### Local prerequisites

For local development:

- Go 1.23+
- `make`
- a Unix-like environment with standard shell tools

For the packaged host demo:

- Ubuntu
- `systemd`
- `sudo`
- `apt` / `dpkg`

I did not optimize the primary demo path around Docker because package installation, service management, sudo policy, and host-local reconciliation are central to the problem. For this project, native Ubuntu packaging is a better fit than containerizing away the interesting parts.

## Run and Test

### Local dev loop

Build and test:

```bash
make build
make test
```

Start the control plane and agent with example configs:

```bash
go run ./cmd/scmctld -config ./configs/examples/scmctld.yaml
go run ./cmd/scmctld-agent -config ./configs/examples/scmctld-agent.yaml
```

Validate and submit a manifest:

```bash
go run ./cmd/scmctl validate -f ./examples/manifests/nginx.yaml
go run ./cmd/scmctl apply -f ./examples/manifests/nginx.yaml --server 127.0.0.1:8443
```

Use `http://127.0.0.1:8080`, the apply detail page, or `scmctl --watch` during local testing. The example configs are biased toward the packaged Ubuntu path under `/var/lib/scm/...`; for repo-local experimentation you can either override the paths or use the compose/dev config under `configs/dev`.

### Packaged Ubuntu demo

Build a release bundle:

```bash
./scripts/release.sh dev
```

On Ubuntu:

```bash
tar -xzf scm_dev_linux_amd64.tar.gz
cd scm
sudo ./smoke.sh
```

The quickest successful evaluator path is the standalone packaged demo:

- run `scmctld` and `scmctld-agent` on the same host
- point the agent at `127.0.0.1:8443`
- use the single-host PHP manifest

Required config values:

`/etc/scm/scmctld.yaml`

```yaml
grpc_listen_address: ":8443"
http_listen_address: ":8080"
database_path: "/var/lib/scm/scmctld.db"
log_level: "info"
log_json: false
lease_duration: 2m
```

`/etc/scm/scmctld-agent.yaml`

```yaml
control_plane_address: "127.0.0.1:8443"
state_dir: "/var/lib/scm/scmctld-agent/state"
manifest_cache_dir: "/var/lib/scm/scmctld-agent/manifests"
metrics_listen_address: ":9108"
host_id: "demo-host-1"
agent_id: "demo-host-1-agent"
labels:
  role: "web"
  env: "demo"
log_level: "info"
log_json: false
poll_interval: 5s
run_timeout: 5m
```

The installed helper path is:

```bash
sudo ./smoke.sh
scm-demo
```

What those helpers do:

- install and start the packaged daemons
- point you at the expected config values
- validate local health checks
- submit the single-host PHP manifest
- print the apply detail URL and verification commands

I used this same standalone deployment pattern on two separate hosts. Each host ran its own control plane and agent locally, and each successfully converged the PHP app to `Hello, world!`.

Progress view options:

- control plane apply detail page: `http://127.0.0.1:8080/applies/<apply_id>`
- `scmctl --watch`
- agent execution logs: `journalctl -u scmctld-agent -f -o cat`

### Verification

Local verification:

```bash
curl -sv http://127.0.0.1/
systemctl status scmctld --no-pager
systemctl status scmctld-agent --no-pager
```

If public ingress is available:

```bash
curl -sv http://PUBLIC_IP/
```

Expected result:

- `200 OK`
- response body includes `Hello, world!`

### CI and automated checks

The repository includes:

- `make test` -> `./scripts/test.sh`
- GitHub Actions CI at [.github/workflows/test.yml](.github/workflows/test.yml)
- GitHub Actions packaged artifacts at [.github/workflows/artifacts.yml](.github/workflows/artifacts.yml)

The release bundle includes:

- all three binaries
- example configs
- systemd units
- example manifests
- `install.sh`
- `smoke.sh`
- `scm-demo`

Steady-state daemons run as dedicated service users:

- `scmctld`
- `scmctld-agent`

The agent uses a narrow sudoers policy for package, service, and privileged file operations instead of running the entire daemon as root.

## Third-Party Tools and Libraries

Primary third-party dependencies:

- `google.golang.org/grpc`: gRPC transport between `scmctl`, `scmctld`, and `scmctld-agent`
- `gopkg.in/yaml.v3`: manifest and config parsing
- `github.com/prometheus/client_golang`: metrics instrumentation and Prometheus exposition
- `modernc.org/sqlite`: embedded SQLite driver for the control-plane persistence layer

System tools the agent intentionally relies on:

- `apt-get` / `dpkg` for package reconciliation
- `systemctl` for service reconciliation
- `sudo` for narrowly scoped privileged operations
- `journald` for host-local execution logs

I did not use third-party hosted APIs. The system is self-contained aside from the OS package/service manager on Ubuntu hosts.

## Major Design Choices

### Agent-pull instead of SSH/push

I chose an agent-pull model so the control plane can remain a scheduler and source of truth rather than a process that stores host credentials and reaches into machines over SSH. Each host runs its own agent, pulls work, and reconciles local state.

### SQLite only in the control plane

The control plane needs durable state for:

- registered agents
- applies
- work items
- event history

SQLite was a good MVP tradeoff there: durable, simple, and sufficient for a small operational UI and recoverable work queue without adding more infrastructure during the project.

The agent keeps only a bounded JSON checkpoint on disk for crash recovery. That keeps the host-side persistence model simple while leaving the control plane as the canonical audit/history store.

### Native Ubuntu packaging instead of a Docker-first demo

This project manages packages, files, and services on Linux hosts. Because `apt`, `systemd`, `sudo`, and journald are part of the actual behavior being demonstrated, I treated native Ubuntu packaging and systemd units as the primary install path rather than hiding that behavior behind a container.

### Explicit dependency graph in the DSL

The manifest DSL supports `requires` and `notifies` so the executor can model ordering and change-triggered follow-up behavior intentionally. `requires` becomes a DAG and is topologically sorted before reconciliation, which gives predictable and explainable execution order instead of relying on file order.

## Tradeoffs and Known Limitations

### What I prioritized

- clean control-plane / agent separation
- explicit domain boundaries
- idempotent host reconciliation
- operational visibility via UI, logs, and metrics
- Ubuntu-friendly packaging and systemd integration

### Known limitations

- the intended shared-control-plane, multi-host topology was blocked by environment/network controls I did not have access to change
- bootstrap and debugging still involve more manual root work than I would want long term
- SQLite is appropriate for the control plane MVP but not the final production persistence story
- the agent-local checkpoint model is intentionally minimal; with more time I would add explicit retention/cleanup and restart-resume semantics around it

### With more time

- separate poll cadence and run timeout is now fixed, but I would further harden the agent execution and progress-reporting path
- improve installer and bootstrap ergonomics so the demo path requires less manual host editing
- add a cleaner production deployment story for a separately hosted control plane in a shared VPC/network
- broaden validation and end-to-end testing around real distro/package differences
