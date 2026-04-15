# Setup

## Local development

1. Ensure Go 1.22+ is installed.
2. Run `./scripts/bootstrap.sh`.
3. Run `make test`.

## Local demo

1. Start the control plane:

```bash
go run ./cmd/scmctld -config ./configs/examples/scmctld.yaml
```

2. In another terminal, start the agent:

```bash
go run ./cmd/scmctld-agent -config ./configs/examples/scmctld-agent.yaml
```

3. Submit an apply:

```bash
go run ./cmd/scmctl apply -f ./examples/manifests/nginx.yaml --server 127.0.0.1:8443 --watch
```

4. Open `http://127.0.0.1:8080` to inspect the UI.
