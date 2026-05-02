# Manifest

**API:** `akash.overlock.network/v1alpha1`
**Kind:** `Manifest`
**Scope:** Cluster

Sends the workload manifest to the Akash provider over mTLS. This is the final step before a workload becomes running. The controller resolves the lease coordinates from the `Lease` CR, reads the mTLS keypair from the certificate Secret, and pushes the SDL-derived manifest to the provider's HTTP API.

## Spec fields (`forProvider`)

| Field | Required | Description |
|---|---|---|
| `leaseRef.name` | yes | Name of the `Lease` CR |
| `leaseRef.namespace` | no | Namespace of the `Lease` CR |
| `certificateSecretRef.name` | yes | Name of the Secret containing `tls.crt` / `tls.key` |
| `certificateSecretRef.namespace` | no | Namespace of the certificate Secret |

The certificate Secret is typically the one published by a `Certificate` CR via `writeConnectionSecretToRef`.

## Status fields (`atProvider`)

| Field | Description |
|---|---|
| `state` | Manifest state: `pending`, `deployed`, or `failed` |
| `owner` | Deployment owner (from Lease) |
| `dseq` / `gseq` / `oseq` | Lease sequence numbers (from Lease) |
| `provider` | Provider address (from Lease) |
| `sdlContent` | Rendered SDL sent to provider |
| `manifestVersion` | Content hash of the deployed manifest |
| `deployedAt` | Timestamp when manifest was sent |
| `services` | List of service definitions from the manifest |
| `validationErrors` | Validation errors reported by the provider |
| `providerResponse` | Raw response from provider |

## Minimal example

```yaml
apiVersion: akash.overlock.network/v1alpha1
kind: Manifest
metadata:
  name: my-app
spec:
  providerConfigRef:
    name: default
  forProvider:
    leaseRef:
      name: my-app-lease
      namespace: default
    certificateSecretRef:
      name: my-app-cert-tls
      namespace: default
```

## Lifecycle notes

- **Create:** Resolves the lease coordinates, renders the SDL into a manifest, and sends it to the provider via mTLS. State moves to `deployed` on success.
- **Update:** If the underlying SDL changes (detected via `manifestVersion` hash), the controller re-sends the updated manifest to the provider.
- **Delete:** No on-chain transaction. The provider will continue running the workload until the `Lease` is closed.
