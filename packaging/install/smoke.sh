#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_MODE="${1:-both}"

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

sudo_if_needed() {
  if [[ "${EUID}" -eq 0 ]]; then
    "$@"
  else
    sudo "$@"
  fi
}

check_file() {
  if [[ ! -f "$1" ]]; then
    echo "required file not found: $1" >&2
    exit 1
  fi
}

wait_for_url() {
  local url="$1"
  local label="$2"
  local attempts="${3:-20}"
  local delay="${4:-1}"

  for ((i = 1; i <= attempts; i++)); do
    if curl -fsS "${url}" >/dev/null 2>&1; then
      echo "${label} is ready: ${url}"
      return 0
    fi
    sleep "${delay}"
  done

  echo "timed out waiting for ${label}: ${url}" >&2
  exit 1
}

case "${SERVICE_MODE}" in
  both|control-plane|agent) ;;
  *)
    echo "usage: ./smoke.sh [both|control-plane|agent]" >&2
    exit 1
    ;;
esac

need curl
need systemctl
check_file "${ROOT_DIR}/install.sh"
check_file "${ROOT_DIR}/bin/scmctld"
check_file "${ROOT_DIR}/bin/scmctld-agent"
check_file "${ROOT_DIR}/bin/scmctl"
check_file "${ROOT_DIR}/etc/scm/scmctld.yaml.example"
check_file "${ROOT_DIR}/etc/scm/scmctld-agent.yaml.example"

sudo_if_needed "${ROOT_DIR}/install.sh"

cat <<'EOF'
Install completed. Before starting services, edit:
  /etc/scm/scmctld.yaml
  /etc/scm/scmctld-agent.yaml

Standalone demo values:
  scmctld database_path: /var/lib/scm/scmctld.db
  scmctld agent_auth_tokens:
    demo-host-1-agent: demo-host-1-token
  scmctld-agent control_plane_address: 127.0.0.1:8443
  scmctld-agent state_dir: /var/lib/scm/scmctld-agent/state
  scmctld-agent manifest_cache_dir: /var/lib/scm/scmctld-agent/manifests
  scmctld-agent host_id: demo-host-1
  scmctld-agent agent_id: demo-host-1-agent
  scmctld-agent auth_token: demo-host-1-token
  scmctld-agent poll_interval: 5s
  scmctld-agent run_timeout: 5m

Installed manifests:
  /usr/local/share/scm/examples/manifests/php-app-single-host.yaml
  /usr/local/share/scm/examples/manifests/php-app-two-hosts.yaml
EOF

if [[ "${SERVICE_MODE}" == "both" || "${SERVICE_MODE}" == "control-plane" ]]; then
  sudo_if_needed systemctl enable --now scmctld
  wait_for_url "http://127.0.0.1:8080" "control plane UI"
fi

if [[ "${SERVICE_MODE}" == "both" || "${SERVICE_MODE}" == "agent" ]]; then
  sudo_if_needed systemctl enable --now scmctld-agent
  wait_for_url "http://127.0.0.1:9108/healthz" "agent health"
fi

cat <<'EOF'

Optional packaging sanity check complete.

Recommended next steps:
  1. Edit /etc/scm/scmctld.yaml and /etc/scm/scmctld-agent.yaml
  2. sudo systemctl restart scmctld scmctld-agent
  3. curl http://127.0.0.1:9108/readyz
  4. scmctl validate -f /usr/local/share/scm/examples/manifests/php-app-single-host.yaml
  5. scmctl apply -f /usr/local/share/scm/examples/manifests/php-app-single-host.yaml --server 127.0.0.1:8443
EOF
