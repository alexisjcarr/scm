# Setup

## Local development

1. Ensure Go 1.22+ is installed.
2. Run `./scripts/bootstrap.sh`.
3. Run `make test`.

## Local demo

1. Start the control plane:

```bash
go run ./cmd/scmctld -config ./configs/examples/scmctld.yaml
```

2. In another terminal, start the agent:

```bash
go run ./cmd/scmctld-agent -config ./configs/examples/scmctld-agent.yaml
```

3. Submit an apply:

```bash
go run ./cmd/scmctl apply -f ./examples/manifests/nginx.yaml --server 127.0.0.1:8443 --watch
```

4. Open `http://127.0.0.1:8080` to inspect the UI.

## Take-home demo topology

Use this flow when you want `scmctld` on your laptop and one `scmctld-agent` on each EC2 host.

1. Start the control plane locally with Docker Compose:

```bash
docker compose up --build scmctld
```

2. Expose `8443/tcp` from your laptop through a stable tunnel or relay that the EC2 hosts can reach.

3. Copy `configs/examples/scmctld-agent-remote.yaml` to each host as the starting point for `/etc/scm/scmctld-agent.yaml`, replacing `demo-tunnel.example.com:8443` with your reachable tunnel address.

4. Install and start `scmctld-agent` on each host under `systemd`.

5. Run `scmctl` from your laptop and submit manifests against the local control plane.

The tunnel is a demo reachability mechanism only. The product model is still an agent pull design where the control plane exposes gRPC and agents decide when to fetch work; it is not a password-based reach-into-hosts control path.

For the full bootstrap and PHP app rollout path, see `docs/takehome.md`.
