#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COVER_DIR="${ROOT_DIR}/dist/coverage"
PROFILE="${COVER_DIR}/cover.out"
SUMMARY="${COVER_DIR}/summary.txt"
HTML="${COVER_DIR}/cover.html"

mkdir -p "${COVER_DIR}"

cd "${ROOT_DIR}"
go test ./... -coverprofile="${PROFILE}"
go tool cover -func="${PROFILE}" | tee "${SUMMARY}"
go tool cover -html="${PROFILE}" -o "${HTML}"

echo "coverage profile: ${PROFILE}"
echo "coverage summary: ${SUMMARY}"
echo "coverage html: ${HTML}"
