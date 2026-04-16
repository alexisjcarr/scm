#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

ensure_group() {
  local group_name="$1"
  if ! getent group "${group_name}" >/dev/null; then
    groupadd --system "${group_name}"
  fi
}

ensure_user() {
  local user_name="$1"
  local group_name="$2"
  local home_dir="$3"
  if ! id -u "${user_name}" >/dev/null 2>&1; then
    useradd --system --gid "${group_name}" --home "${home_dir}" --shell /usr/sbin/nologin "${user_name}"
  fi
}

ensure_group scmctld
ensure_group scmctld-agent
ensure_user scmctld scmctld /var/lib/scm
ensure_user scmctld-agent scmctld-agent /var/lib/scm/scmctld-agent

install -d /usr/local/bin /usr/local/libexec /etc/scm /var/lib/scm /var/lib/scm/scmctld-agent /var/lib/scm/scmctld-agent/state /var/lib/scm/scmctld-agent/manifests /usr/local/share/doc/scm /usr/local/share/scm/examples/manifests
install -m 0755 "${ROOT_DIR}/bin/scmctl" /usr/local/bin/scmctl
install -m 0755 "${ROOT_DIR}/bin/scmctld" /usr/local/bin/scmctld
install -m 0755 "${ROOT_DIR}/bin/scmctld-agent" /usr/local/bin/scmctld-agent
install -m 0755 "${ROOT_DIR}/install/scm-demo" /usr/local/bin/scm-demo
install -m 0755 "${ROOT_DIR}/install/scm-host-a-demo" /usr/local/bin/scm-host-a-demo
install -m 0755 "${ROOT_DIR}/install/scm-agent-fileop" /usr/local/libexec/scm-agent-fileop
install -m 0640 "${ROOT_DIR}/etc/scm/scmctld.yaml.example" /etc/scm/scmctld.yaml
install -m 0640 "${ROOT_DIR}/etc/scm/scmctld-agent.yaml.example" /etc/scm/scmctld-agent.yaml
install -m 0644 "${ROOT_DIR}/lib/systemd/system/scmctld.service" /etc/systemd/system/scmctld.service
install -m 0644 "${ROOT_DIR}/lib/systemd/system/scmctld-agent.service" /etc/systemd/system/scmctld-agent.service
install -d /etc/sudoers.d
install -m 0440 "${ROOT_DIR}/sudoers/scmctld-agent" /etc/sudoers.d/scmctld-agent
cp -R "${ROOT_DIR}/share/doc/scm/." /usr/local/share/doc/scm/
cp -R "${ROOT_DIR}/share/scm/examples/manifests/." /usr/local/share/scm/examples/manifests/
chown -R scmctld:scmctld /var/lib/scm
chown -R scmctld-agent:scmctld-agent /var/lib/scm/scmctld-agent
chown root:scmctld /etc/scm/scmctld.yaml
chown root:scmctld-agent /etc/scm/scmctld-agent.yaml
if command -v visudo >/dev/null 2>&1; then
  visudo -cf /etc/sudoers.d/scmctld-agent >/dev/null
fi
systemctl daemon-reload
echo "installed scm binaries, example manifests, sudoers drop-in, and systemd units"
