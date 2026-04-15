#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if ! command -v protoc >/dev/null 2>&1; then
  echo "protoc is required to regenerate pkg/api; see scripts/bootstrap.sh" >&2
  exit 1
fi

if ! command -v protoc-gen-go >/dev/null 2>&1 || ! command -v protoc-gen-go-grpc >/dev/null 2>&1; then
  echo "protoc-gen-go and protoc-gen-go-grpc are required" >&2
  exit 1
fi

rm -rf "${ROOT_DIR}/pkg/api/scm/v1"/*.pb.go

protoc \
  --proto_path="${ROOT_DIR}/proto" \
  --go_out="${ROOT_DIR}/pkg/api" \
  --go_opt=paths=source_relative \
  --go-grpc_out="${ROOT_DIR}/pkg/api" \
  --go-grpc_opt=paths=source_relative \
  "${ROOT_DIR}/proto/scm/v1/scm.proto"
