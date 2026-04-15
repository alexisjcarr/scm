# Install

## Ubuntu evaluator flow

1. Build release artifacts:

```bash
./scripts/release.sh dev
```

2. Copy the desired tarball to the Ubuntu host and unpack it:

```bash
tar -xzf scm_dev_linux_amd64.tar.gz
cd scm
```

3. Install:

```bash
sudo ./install.sh
```

4. Review `/etc/scm/scmctld.yaml` and `/etc/scm/scmctld-agent.yaml`.

The packaged units run as dedicated service users:

- `scmctld`
- `scmctld-agent`

The installer also places a narrow sudoers drop-in so `scmctld-agent` can manage packages, services, and privileged file writes without running the entire daemon as root.

5. Start the services:

```bash
sudo systemctl enable --now scmctld
sudo systemctl enable --now scmctld-agent
```

6. Use the installed CLI:

```bash
scmctl apply -f ./share/scm/examples/manifests/nginx.yaml --server 127.0.0.1:8443 --watch
```

For the local-control-plane plus remote-EC2 demo flow, see `docs/takehome.md`.
