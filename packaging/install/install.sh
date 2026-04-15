#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

install -d /usr/local/bin /etc/scm /var/lib/scm /var/lib/scm/scmctld-agent /var/lib/scm/scmctld-agent/manifests /usr/local/share/doc/scm
install -m 0755 "${ROOT_DIR}/bin/scmctl" /usr/local/bin/scmctl
install -m 0755 "${ROOT_DIR}/bin/scmctld" /usr/local/bin/scmctld
install -m 0755 "${ROOT_DIR}/bin/scmctld-agent" /usr/local/bin/scmctld-agent
install -m 0644 "${ROOT_DIR}/etc/scm/scmctld.yaml.example" /etc/scm/scmctld.yaml
install -m 0644 "${ROOT_DIR}/etc/scm/scmctld-agent.yaml.example" /etc/scm/scmctld-agent.yaml
install -m 0644 "${ROOT_DIR}/lib/systemd/system/scmctld.service" /etc/systemd/system/scmctld.service
install -m 0644 "${ROOT_DIR}/lib/systemd/system/scmctld-agent.service" /etc/systemd/system/scmctld-agent.service
cp -R "${ROOT_DIR}/share/doc/scm/." /usr/local/share/doc/scm/
systemctl daemon-reload
echo "installed scm binaries, configs, and systemd units"
