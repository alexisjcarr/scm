#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${1:-dev}"
DIST_DIR="${ROOT_DIR}/dist"
STAGE_DIR="${DIST_DIR}/stage"

rm -rf "${STAGE_DIR}"
mkdir -p "${DIST_DIR}"

build_arch() {
  local goos="$1"
  local goarch="$2"
  local out_dir="${STAGE_DIR}/scm"
  local archive="${DIST_DIR}/scm_${VERSION}_${goos}_${goarch}.tar.gz"

  rm -rf "${out_dir}"
  mkdir -p "${out_dir}/bin" "${out_dir}/etc/scm" "${out_dir}/install" "${out_dir}/lib/systemd/system" "${out_dir}/share/scm/examples/manifests" "${out_dir}/share/doc/scm/README.assets" "${out_dir}/sudoers"

  GOOS="${goos}" GOARCH="${goarch}" go build -o "${out_dir}/bin/scmctl" ./cmd/scmctl
  GOOS="${goos}" GOARCH="${goarch}" go build -o "${out_dir}/bin/scmctld" ./cmd/scmctld
  GOOS="${goos}" GOARCH="${goarch}" go build -o "${out_dir}/bin/scmctld-agent" ./cmd/scmctld-agent

  cp "${ROOT_DIR}/configs/examples/scmctld.yaml" "${out_dir}/etc/scm/scmctld.yaml.example"
  cp "${ROOT_DIR}/configs/examples/scmctld-agent.yaml" "${out_dir}/etc/scm/scmctld-agent.yaml.example"
  cp "${ROOT_DIR}/packaging/systemd/scmctld.service" "${out_dir}/lib/systemd/system/scmctld.service"
  cp "${ROOT_DIR}/packaging/systemd/scmctld-agent.service" "${out_dir}/lib/systemd/system/scmctld-agent.service"
  cp "${ROOT_DIR}/examples/manifests/nginx.yaml" "${out_dir}/share/scm/examples/manifests/nginx.yaml"
  cp "${ROOT_DIR}/examples/manifests/php-app-host-a.yaml" "${out_dir}/share/scm/examples/manifests/php-app-host-a.yaml"
  cp "${ROOT_DIR}/examples/manifests/php-app-two-hosts.yaml" "${out_dir}/share/scm/examples/manifests/php-app-two-hosts.yaml"
  cp "${ROOT_DIR}/README.md" "${out_dir}/share/doc/scm/README.md"
  cp "${ROOT_DIR}/README.assets/architecture-sequence.png" "${out_dir}/share/doc/scm/README.assets/architecture-sequence.png"
  cp "${ROOT_DIR}/README.assets/architecture-sequence.svg" "${out_dir}/share/doc/scm/README.assets/architecture-sequence.svg"
  cp "${ROOT_DIR}/README.assets/architecture-sequence.puml" "${out_dir}/share/doc/scm/README.assets/architecture-sequence.puml"
  cp "${ROOT_DIR}/packaging/install/install.sh" "${out_dir}/install.sh"
  cp "${ROOT_DIR}/packaging/install/smoke.sh" "${out_dir}/smoke.sh"
  cp "${ROOT_DIR}/packaging/install/scm-agent-fileop" "${out_dir}/install/scm-agent-fileop"
  cp "${ROOT_DIR}/packaging/install/scm-host-a-demo" "${out_dir}/install/scm-host-a-demo"
  cp "${ROOT_DIR}/packaging/sudoers/scmctld-agent" "${out_dir}/sudoers/scmctld-agent"

  chmod +x "${out_dir}/install/scm-agent-fileop"
  chmod +x "${out_dir}/install/scm-host-a-demo"
  chmod +x "${out_dir}/install.sh"
  chmod +x "${out_dir}/smoke.sh"
  tar -C "${STAGE_DIR}" -czf "${archive}" scm
}

build_arch linux amd64
build_arch linux arm64

(
  cd "${DIST_DIR}"
  shasum -a 256 scm_"${VERSION}"_linux_amd64.tar.gz scm_"${VERSION}"_linux_arm64.tar.gz > "scm_${VERSION}_checksums.txt"
)
