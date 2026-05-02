# SDL

**API:** `akash.overlock.network/v1alpha1`
**Kind:** `SDL`
**Scope:** Namespaced

An SDL (Stack Definition Language) resource describes the workload to be deployed on Akash: container images, resource requirements, placement constraints, and pricing. It is referenced by a `Deployment` CR and is not itself submitted on-chain directly.

## Spec fields (`forProvider`)

| Field | Required | Description |
|---|---|---|
| `version` | yes | SDL version — must be `"2.0"` |
| `services` | yes | Map of service name to service definition |
| `profiles` | yes | Compute and placement profiles |
| `deployment` | yes | Binds services to placement profiles with instance counts |

### `services.<name>`

| Field | Description |
|---|---|
| `image` | Docker image (required) |
| `command` | Override container entrypoint |
| `args` | Arguments for the command |
| `env` | List of `KEY=value` environment strings |
| `expose` | List of port exposure rules (see below) |
| `params.storage` | Map of named storage mounts (`mount`, `readOnly`) |

### `expose` entry

| Field | Description |
|---|---|
| `port` | Container port |
| `as` | External port (defaults to `port`) |
| `proto` | `tcp` (default) or `udp` |
| `to` | List of `{global: true}` or `{service: <name>}` |
| `accept.items` | Accepted hostnames for HTTP ingress |

### `profiles.compute.<name>.resources`

| Field | Example | Description |
|---|---|---|
| `cpu` | `"0.5"` | CPU units (fractional or millicores) |
| `memory` | `"512Mi"` | Memory (Ki, Mi, Gi, Ti) |
| `storage` | `[{size: "1Gi"}]` | Ephemeral storage list |
| `gpu.units` | `1` | Number of GPU units |

### `profiles.placement.<name>`

| Field | Description |
|---|---|
| `attributes` | Map of provider attributes to require |
| `signedBy.anyOf` | Require signature from any listed address |
| `pricing.<service>.amount` | Max price per block in uact |

### `deployment.<service>`

| Field | Description |
|---|---|
| `profile` | Name of a placement profile |
| `count` | Number of instances (1–100) |

## Minimal example

```yaml
apiVersion: akash.overlock.network/v1alpha1
kind: SDL
metadata:
  name: my-app
  namespace: default
spec:
  providerConfigRef:
    name: default
  forProvider:
    version: "2.0"
    services:
      web:
        image: nginx:1.21.6
        expose:
          - port: 80
            as: 80
            to:
              - global: true
    profiles:
      compute:
        web:
          resources:
            cpu: "0.5"
            memory: "512Mi"
            storage:
              - size: "1Gi"
      placement:
        westcoast:
          pricing:
            web:
              amount: 100
    deployment:
      web:
        profile: westcoast
        count: 1
```

## Lifecycle notes

- **Create:** The SDL is validated and a content hash is stored in `status.atProvider.hash`. No on-chain transaction is triggered.
- **Update:** Any change to `forProvider` recomputes the hash. Dependent `Deployment` CRs detect the new hash and re-broadcast an updated deployment on-chain.
- **Delete:** Safe to delete once no `Deployment` references this SDL.
