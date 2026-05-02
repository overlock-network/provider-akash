# ProviderConfig

**API:** `akash.overlock.network/v1alpha1`
**Kind:** `ProviderConfig`
**Scope:** Cluster

Holds the Akash wallet credentials and node connection settings used by all other resources. Every managed resource has a `providerConfigRef` that points to one of these.

## Spec fields

### `credentials` (required)

Reference to the wallet key in ASCII-armored format produced by `akash keys export <key-name> --keyring-backend <os|test>` (the block starting with `-----BEGIN TENDERMINT PRIVATE KEY-----`).

| Field | Description |
|---|---|
| `source` | Where to read the credential. One of `None`, `Secret`, `InjectedIdentity`, `Environment`, `Filesystem` |
| `secretRef.namespace` | Namespace of the Secret |
| `secretRef.name` | Name of the Secret |
| `secretRef.key` | Key inside the Secret holding the armored private key |

### `passphrase` (required when the key is encrypted)

Same shape as `credentials`. Holds the passphrase entered during `akash keys export`. The cosmos-sdk keyring uses this to decrypt the armored block — without it the controller cannot import the key.

### `configuration` (optional)

| Field | Default | Description |
|---|---|---|
| `keyName` | `default` | Name of the key in the keyring |
| `keyringBackend` | `test` | Keyring backend (`os`, `file`, `test`, `memory`) |
| `accountAddress` | — | Akash address override (auto-derived if omitted) |
| `net` | `mainnet` | Network (`mainnet`, `testnet`, `sandbox`) |
| `chainId` | `akashnet-2` | Chain ID |
| `node` | `https://rpc.akashnet.io:443` | RPC endpoint |
| `home` | `/tmp/.akash` | Akash home directory |
| `path` | `/usr/local/bin/akash` | Path to the `akash` binary |
| `providersApi` | `https://akash-api.polkachu.com` | Providers REST API base URL |
| `skipTLSVerification` | `false` | Skip TLS verification (dev/test only) |

## Minimal example

```yaml
apiVersion: akash.overlock.network/v1alpha1
kind: ProviderConfig
metadata:
  name: default
spec:
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: akash-credentials
      key: privateKey
  passphrase:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: akash-credentials
      key: passphrase
```

## Lifecycle notes

- `ProviderConfig` is not itself an on-chain resource; it is never created or deleted on Akash.
- Changing `configuration` fields takes effect on the next reconciliation of any resource that references this config.
- Deleting a `ProviderConfig` while resources still reference it will block those resources from reconciling.
