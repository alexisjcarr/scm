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
  mkdir -p "${out_dir}/bin" "${out_dir}/etc/scm" "${out_dir}/lib/systemd/system" "${out_dir}/share/scm/examples/manifests" "${out_dir}/share/doc/scm"

  GOOS="${goos}" GOARCH="${goarch}" go build -o "${out_dir}/bin/scmctl" ./cmd/scmctl
  GOOS="${goos}" GOARCH="${goarch}" go build -o "${out_dir}/bin/scmctld" ./cmd/scmctld
  GOOS="${goos}" GOARCH="${goarch}" go build -o "${out_dir}/bin/scmctld-agent" ./cmd/scmctld-agent

  cp "${ROOT_DIR}/configs/examples/scmctld.yaml" "${out_dir}/etc/scm/scmctld.yaml.example"
  cp "${ROOT_DIR}/configs/examples/scmctld-agent.yaml" "${out_dir}/etc/scm/scmctld-agent.yaml.example"
  cp "${ROOT_DIR}/packaging/systemd/scmctld.service" "${out_dir}/lib/systemd/system/scmctld.service"
  cp "${ROOT_DIR}/packaging/systemd/scmctld-agent.service" "${out_dir}/lib/systemd/system/scmctld-agent.service"
  cp "${ROOT_DIR}/examples/manifests/nginx.yaml" "${out_dir}/share/scm/examples/manifests/nginx.yaml"
  cp "${ROOT_DIR}/README.md" "${out_dir}/share/doc/scm/README.md"
  cp "${ROOT_DIR}/docs/install.md" "${out_dir}/share/doc/scm/install.md"
  cp "${ROOT_DIR}/docs/architecture.md" "${out_dir}/share/doc/scm/architecture.md"
  cp "${ROOT_DIR}/docs/dsl.md" "${out_dir}/share/doc/scm/dsl.md"
  cp "${ROOT_DIR}/packaging/install/install.sh" "${out_dir}/install.sh"

  chmod +x "${out_dir}/install.sh"
  tar -C "${STAGE_DIR}" -czf "${archive}" scm
}

build_arch linux amd64
build_arch linux arm64

(
  cd "${DIST_DIR}"
  shasum -a 256 scm_"${VERSION}"_linux_amd64.tar.gz scm_"${VERSION}"_linux_arm64.tar.gz > "scm_${VERSION}_checksums.txt"
)
