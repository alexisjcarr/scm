# DSL

## Manifest shape

```yaml
apiVersion: scm/v1
kind: Manifest
metadata:
  name: nginx-demo
  labels:
    team: platform
target:
  hosts:
    - demo-host-1
  selector:
    matchLabels:
      role: web
resources:
  - id: nginx_pkg
    type: package
    name: nginx
    state: installed
  - id: nginx_conf
    type: file
    path: /etc/nginx/nginx.conf
    content: "..."
    mode: "0644"
    owner: root
    group: root
    state: present
    requires: [nginx_pkg]
    notifies: [nginx_svc]
  - id: nginx_svc
    type: service
    name: nginx
    state: running
    enabled: true
    requires: [nginx_conf]
```

## Targeting

- `target.hosts`: explicit host IDs.
- `target.selector.matchLabels`: exact-match label selection against registered agent labels.
- If both are present, the final target set is the union of both, deduplicated by host ID.

## Resource types

### `package`

- `name`
- `state`: `installed` or `absent`

### `file`

- `path`
- `content`
- `mode`
- optional `owner`
- optional `group`
- `state`: `present` or `absent`

### `service`

- `name`
- `state`: `running` or `stopped`
- optional `enabled`

## Relationships

- `requires`: resource dependency edge used for ordering.
- `notifies`: change-triggered follow-up edge. In the MVP this is intended for services that should be revisited after an upstream change.

## Validation rules

- resource IDs must be unique
- all `requires` and `notifies` references must exist
- `notifies` may only point to service resources
- dependency cycles are rejected

## Ubuntu-focused examples

- `examples/manifests/nginx.yaml` is the smallest service demo.
- `examples/manifests/php-app-two-hosts.yaml` is the take-home example for two Ubuntu web hosts.
