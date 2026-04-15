# Development

## Repository rules

- domain packages do not depend on gRPC, SQLite, or HTTP
- transport and persistence live in `infra`
- shared helpers live in `internal/platform`
- generated or generated-equivalent API transport code stays in `pkg/api`

## Common commands

- `make build`
- `make test`
- `make generate`
- `./scripts/release.sh dev`

## Notes

- The protobuf definitions under `proto/scm/v1` are the canonical contract.
- The repository is buildable without forcing every evaluator to install proto tooling first.
- The Linux resource backend targets Ubuntu-style hosts using `apt`, `dpkg`, and `systemctl`.
