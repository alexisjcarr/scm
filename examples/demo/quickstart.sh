#!/usr/bin/env bash
set -euo pipefail

go run ./cmd/scmctld -config ./configs/examples/scmctld.yaml &
CTLD_PID=$!
go run ./cmd/scmctld-agent -config ./configs/examples/scmctld-agent.yaml &
AGENT_PID=$!

cleanup() {
  kill "${AGENT_PID}" "${CTLD_PID}" 2>/dev/null || true
}
trap cleanup EXIT

sleep 2
go run ./cmd/scmctl apply -f ./examples/manifests/nginx.yaml --server 127.0.0.1:8443 --watch
