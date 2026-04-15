# Take-home deployment guide

This repository is shaped for a take-home where you need to deploy a small PHP app to two EC2 hosts.

## Intended operating model

- `scmctld` runs separately from the managed hosts.
- `scmctld-agent` runs 1:1 under `systemd` on each host.
- `scmctl` runs from your operator machine.

The provided root passwords are useful for one-time bootstrap only. They are not the steady-state control model.

The control plane should not SSH or password into hosts to do work. The steady-state path is:

1. bootstrap the agent once with root
2. start the agent under `systemd`
3. submit desired state through `scmctl`
4. let each host agent pull and reconcile work

## Demo topology

- run `scmctld` locally with Docker Compose
- expose `8443/tcp` through a stable tunnel or relay the EC2 hosts can reach
- point each EC2 agent at that tunnel address
- run `scmctl` locally against the same control plane

This tunnel is only a demo reachability mechanism. It is not a product push channel.

## One-time bootstrap on each EC2 host

These steps assume Ubuntu and root access for initial setup.

Do not reboot either host during this flow.

1. Copy the unpacked release bundle to the host.
2. Install the agent assets:

```bash
sudo ./install.sh
```

3. Start from `/etc/scm/scmctld-agent.yaml` and set:

- `control_plane_address` to your reachable tunnel endpoint
- `host_id` to a stable host name such as `php-web-1` or `php-web-2`
- `agent_id` to a matching unique agent ID
- labels if you want inventory metadata

4. Start the agent:

```bash
sudo systemctl enable --now scmctld-agent
```

5. Verify local diagnostics on the host:

```bash
curl http://127.0.0.1:9108/healthz
curl http://127.0.0.1:9108/readyz
curl http://127.0.0.1:9108/status
```

The packaged unit runs `scmctld-agent` as the dedicated `scmctld-agent` service user. Privileged package, service, and managed file operations are mediated through a narrow `sudoers` drop-in rather than by running the whole agent as root.

## Deploy the PHP app

This repo includes `/Users/alexisjcarr/learning/scm/examples/manifests/php-app-two-hosts.yaml`, which assumes Ubuntu 24.04-style package and service names:

- `nginx`
- `php8.3-fpm`

Submit it from your laptop:

```bash
go run ./cmd/scmctl validate -f ./examples/manifests/php-app-two-hosts.yaml
go run ./cmd/scmctl apply -f ./examples/manifests/php-app-two-hosts.yaml --server 127.0.0.1:8443 --watch
```

The manifest:

- installs `nginx` and `php8.3-fpm`
- writes `/var/www/scm-php-demo/index.php`
- writes `/etc/nginx/sites-available/default`
- ensures `php8.3-fpm` and `nginx` are enabled and running
- serves the provided PHP application so `curl -sv "http://ADDRESS"` returns `200 OK` and includes `Hello, world!`
- uses `notifies` so PHP file changes restart `php8.3-fpm` and nginx config changes restart `nginx`

## Verification

On each host:

```bash
curl -sv "http://PUBLIC_IP"
systemctl status scmctld-agent --no-pager
```

On the control plane:

- open `http://127.0.0.1:8080`
- confirm both hosts are registered and `ready`
- inspect the apply detail page for per-host events
