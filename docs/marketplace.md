# Crossplane Akash Network Provider

Manage [Akash Network](https://akash.network) resources declaratively from Kubernetes via [Crossplane](https://crossplane.io).

## Features

- **Akash Deployments** — create, update, and delete Akash deployments from Kubernetes manifests.
- **Full lifecycle management** — `ProviderConfig`, SDL, `Deployment`, `BidPolicy`, `Lease`, `Certificate`, `Manifest` are all reconciled by the controller.
- **On-chain transactions** — broadcasts to the Akash chain and delivers manifests to providers over mTLS.
- **v2 BME economy** — deposits and lease pricing in ACT (uact); gas in AKT (uakt).
- **Crossplane composition** — compose Akash resources together with any other Crossplane provider.

## Install

```bash
crossplane xpkg install provider xpkg.upbound.io/overlock-network/provider-akash:v0.0.10
```

Or as a manifest:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-akash
spec:
  package: xpkg.upbound.io/overlock-network/provider-akash:v0.0.10
```

## Prerequisites

- A Kubernetes cluster with Crossplane installed.
- An Akash wallet funded with [minted ACT](https://akash.network/docs/developers/deployment/cli/act-mint-burn/) and AKT for gas.

## Configure

Create a `ProviderConfig` referencing a `Secret` that holds the wallet credentials. See examples under [`examples/provider`](https://github.com/overlock-network/provider-akash/tree/main/examples/provider).

## Usage

Once installed and configured, the Crossplane Akash provider will reconcile Akash network resources based on your Kubernetes manifests.

### Wallet Secret

Holds the base64-encoded Akash wallet export and its passphrase. Referenced by `ProviderConfig` to sign on-chain transactions.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: akash-wallet-secret
type: Opaque
data:
  credentials: "<BASE64_ENCODED_TENDERMINT_PRIVATE_KEY>"
  passphrase: "<BASE64_ENCODED_PASSPHRASE>"
```

### ProviderConfig

Connects the provider to a specific Akash network (mainnet or sandbox) and points at the wallet secret used to authenticate transactions.

```yaml
apiVersion: akash.overlock.network/v1alpha1
kind: ProviderConfig
metadata:
  name: akash-provider
spec:
  credentials:
    source: Secret
    secretRef:
      namespace: default
      name: akash-wallet-secret
      key: credentials
  passphrase:
    source: Secret
    secretRef:
      namespace: default
      name: akash-wallet-secret
      key: passphrase
  configuration:
    keyName: "default"
    keyringBackend: "test"
    net: "sandbox"
    version: "v2.0.0"
    chainId: "sandbox-2"
    accountAddress: "akash1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
    node: "https://rpc.example.akash.network:443"
    home: "/tmp/.akash"
    path: "/usr/local/bin/akash"
    providersApi: "https://akash-api.example.com"
    skipTLSVerification: true
```

### SDL

Declares the workload — container images, exposed ports, compute resources, placement attributes, and pricing — using Akash's Stack Definition Language.

```yaml
apiVersion: akash.overlock.network/v1alpha1
kind: SDL
metadata:
  name: example-sdl
spec:
  providerConfigRef:
    name: akash-provider
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
          attributes:
            region: us-west
          pricing:
            web:
              amount: 100
    deployment:
      web:
        profile: westcoast
        count: 1
```

### Deployment

Creates the on-chain deployment from the referenced SDL, locks the deposit, and writes connection details to the named secret once a lease is active.

```yaml
apiVersion: akash.overlock.network/v1alpha1
kind: Deployment
metadata:
  name: example-deployment
  labels:
    app: web-server
    tier: frontend
spec:
  providerConfigRef:
    name: akash-provider
  forProvider:
    sdlRef:
      name: example-sdl
    deposit: 4500000
  writeConnectionSecretToRef:
    name: example-deployment-connection
    namespace: default
```

### BidPolicy

Decides how incoming provider bids are evaluated — filters by attributes, caps the price, and (when `autoAccept` is true) opens a Lease automatically against the matching deployment.

```yaml
apiVersion: akash.overlock.network/v1alpha1
kind: BidPolicy
metadata:
  name: example-bidpolicy
spec:
  providerConfigRef:
    name: akash-provider
  forProvider:
    selector:
      matchLabels:
        app: web-server
        tier: frontend
    maxPrice: 500
    excludedProviders:
      - "akash1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
    requiredAttributes:
      - key: "region"
        value: "us-west"
    selectionStrategy: "lowest-price"
    autoAccept: true
    maxBids: 15
```

> Certificate and Manifest CRs are created automatically by the Lease controller once a lease becomes Active — you don't need to author them yourself.

## Examples

See the [`examples/`](https://github.com/overlock-network/provider-akash/tree/main/examples) directory for sample configurations across deployments, bid policies, SDLs, and provider configs.

## Source

- Source: [github.com/overlock-network/provider-akash](https://github.com/overlock-network/provider-akash)
- Issues: [github.com/overlock-network/provider-akash/issues](https://github.com/overlock-network/provider-akash/issues)
- License: [Apache-2.0](https://github.com/overlock-network/provider-akash/blob/main/LICENSE)
