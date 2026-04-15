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

5. Start the services:

```bash
sudo systemctl enable --now scmctld
sudo systemctl enable --now scmctld-agent
```

6. Use the installed CLI:

```bash
scmctl apply -f ./share/scm/examples/manifests/nginx.yaml --server 127.0.0.1:8443 --watch
```
