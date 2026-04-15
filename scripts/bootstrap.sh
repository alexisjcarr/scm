#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required tool: $1" >&2
    return 1
  fi
}

echo "checking local Go toolchain"
need go

if ! command -v protoc >/dev/null 2>&1; then
  echo "protoc is not installed."
  echo "on Ubuntu:"
  echo "  sudo apt-get update && sudo apt-get install -y protobuf-compiler"
fi

if ! command -v protoc-gen-go >/dev/null 2>&1; then
  echo "installing protoc-gen-go from local module cache when available"
  GOWORK=off go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.5 || true
fi

if ! command -v protoc-gen-go-grpc >/dev/null 2>&1; then
  echo "installing protoc-gen-go-grpc from local module cache when available"
  GOWORK=off go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1 || true
fi

echo "bootstrap finished"
echo "run 'make test' to verify the workspace"
echo "run 'make generate' after protoc is installed"
echo "workspace: $ROOT_DIR"
