# Crossplane Akash Network Provider
<p align="center"><img src="assets/akash-crossplane.png" alt="Akash Crossplane Logo" width="50%"></p>

This Crossplane provider enables you to manage and reconcile [Akash Network](https://akash.network) resources, such as deployments, directly from your Kubernetes cluster using Crossplane.

## Features

- **Manage Akash Deployments**: Automate the creation, update, and deletion of Akash deployments.
- **Network Resource Reconciliation**: Seamlessly integrate Akash network resources into your Kubernetes environment.
- **Crossplane Integration**: Leverage Crossplane’s powerful composition and reconciliation features to manage your Akash resources declaratively.

## Getting Started

### Prerequisites

- [Crossplane](https://crossplane.io) installed in your Kubernetes cluster.
- An Akash wallet funded with [minted ACT](https://akash.network/docs/developers/deployment/cli/act-mint-burn/) and AKT (gas).


## Install

To install the Akash provider without modifications, use the Crossplane CLI in a Kubernetes cluster where Crossplane is installed:

```console
crossplane xpkg install provider xpkg.upbound.io/overlock-network/provider-akash:v0.0.10
```

You can also manually install the Akash provider by creating a Provider directly:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-akash
spec:
  package: xpkg.upbound.io/overlock-network/provider-akash:v0.0.10
```

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

Check out the [examples](examples/) directory for more sample configurations and usage scenarios.

## Troubleshooting

- **Logs**: Check the Crossplane provider logs for any errors during reconciliation.
- **Status**: Verify the state of your resources via `kubectl get/describe` — each CR's `status.atProvider` reflects the live on-chain state.


## License

This project is licensed under the [Apache 2.0 License](LICENSE).