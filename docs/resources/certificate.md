# Certificate

**API:** `akash.overlock.network/v1alpha1`
**Kind:** `Certificate`
**Scope:** Cluster

Manages a TLS certificate used for mTLS communication with Akash providers. The certificate is registered on-chain and the keypair is published to a Kubernetes Secret via `writeConnectionSecretToRef`. The `Manifest` CR references this Secret to authenticate against the provider.

## Spec fields (`forProvider`)

| Field | Required | Default | Description |
|---|---|---|---|
| `domains` | yes | — | List of domain names for the certificate |
| `deploymentRef.name` | no | — | Optional reference to an associated `Deployment` CR |
| `deploymentRef.namespace` | no | — | Namespace of the referenced Deployment |
| `autoRenew` | no | `true` | Automatically renew before expiry |
| `validityDays` | no | `365` | Certificate validity period in days (1–3650) |

## Status fields (`atProvider`)

| Field | Description |
|---|---|
| `serial` | Certificate serial number |
| `owner` | Owner address from ProviderConfig |
| `state` | Certificate state: `valid`, `expired`, or `revoked` |
| `fingerprint` | Certificate fingerprint |
| `pem` | Certificate PEM content |
| `notBefore` / `notAfter` | Validity window (Unix timestamps) |
| `createdAt` / `expiresAt` / `lastRenewed` | Lifecycle timestamps |

## Minimal example

```yaml
apiVersion: akash.overlock.network/v1alpha1
kind: Certificate
metadata:
  name: my-app-cert
spec:
  providerConfigRef:
    name: default
  writeConnectionSecretToRef:
    name: my-app-cert-tls
    namespace: default
  forProvider:
    domains:
      - my-app.example.com
    autoRenew: true
    validityDays: 365
```

The Secret named `my-app-cert-tls` will contain `tls.crt` and `tls.key` keys used by the `Manifest` CR.

## Lifecycle notes

- **Create:** Generates a keypair and registers the certificate on the Akash chain. Publishes `tls.crt` / `tls.key` to the connection Secret.
- **Update:** Changing `domains` or `validityDays` triggers certificate re-issuance on-chain.
- **Delete:** Revokes the certificate on the Akash chain. The connection Secret is deleted by Crossplane.
