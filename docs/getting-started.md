# Getting started

## Prerequisites

- Kubernetes cluster with [Crossplane](https://crossplane.io) installed
- An Akash wallet funded with [**minted ACT**](https://akash.network/docs/developers/deployment/cli/act-mint-burn/) (deposits and lease pricing) and **AKT** (transaction gas)
- Access to an Akash RPC node (defaults to `https://rpc.akashnet.io:443`)

## Install the provider

Using the Crossplane CLI:

```sh
crossplane xpkg install provider xpkg.upbound.io/overlock-network/provider-akash:v0.0.9
```

Or via a `Provider` manifest:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-akash
spec:
  package: xpkg.upbound.io/overlock-network/provider-akash:v0.0.9
```

Wait for the provider to become healthy:

```sh
kubectl get providers
```

## Configure credentials

Export your Akash wallet to the ASCII-armored format the cosmos-sdk keyring expects:

```sh
akash keys export <key-name> --keyring-backend <os|test>
# prompts for an encryption passphrase — remember it
```

Create a Secret with the armored block under `privateKey` and the passphrase under `passphrase`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: akash-credentials
  namespace: crossplane-system
type: Opaque
stringData:
  privateKey: |
    -----BEGIN TENDERMINT PRIVATE KEY-----
    salt: <...>
    type: secp256k1
    kdf: bcrypt

    <armored-body>
    -----END TENDERMINT PRIVATE KEY-----
  passphrase: "<passphrase-used-during-export>"
```

Create a `ProviderConfig` that references both:

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
  configuration:
    keyName: default
    keyringBackend: test
    net: mainnet
    chainId: akashnet-2
    node: "https://rpc.akashnet.io:443"
```

Apply and verify:

```sh
kubectl apply -f providerconfig.yaml
kubectl get providerconfig default
```

## Deploy a workload

A minimal deployment follows this sequence:

### 1. Define the workload (SDL)

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
              amount: 100        # uact per block (denom is hard-coded to uact under v2 BME)
    deployment:
      web:
        profile: westcoast
        count: 1
```

### 2. Create the on-chain deployment

```yaml
apiVersion: akash.overlock.network/v1alpha1
kind: Deployment
metadata:
  name: my-app
spec:
  providerConfigRef:
    name: default
  forProvider:
    sdlRef:
      name: my-app
      namespace: default
    deposit: 5000000           # uact (5 ACT). Minimum 500000, denom hard-coded to uact.
```

> Gas for the underlying `MsgCreateDeployment` is paid separately in `uakt` from the wallet referenced by the ProviderConfig.

Watch the deployment reach `Bidding` phase:

```sh
kubectl get deployment my-app
# PHASE column should show: Bidding
```

### 3. Select a bid (BidPolicy)

```yaml
apiVersion: akash.overlock.network/v1alpha1
kind: BidPolicy
metadata:
  name: my-app
spec:
  providerConfigRef:
    name: default
  forProvider:
    deploymentRef:
      name: my-app
    selectionStrategy: lowest-price
    autoAccept: true
```

With `autoAccept: true` the controller creates a `Lease` automatically once a qualifying bid is found.

### 4. Lease, Certificate, and Manifest are auto-created

Once the `Lease` is `Ready`, the lease controller creates a `Certificate` (per ProviderConfig) and a `Manifest` (per Lease) for you, and the manifest controller delivers the SDL to the provider over mTLS.

Watch progress:

```sh
kubectl get lease,certificate,manifest
```

When the `Manifest` reports `Ready`, the workload is running. Check `status.atProvider.services` on the Manifest for service URIs.

## Next steps

- See each [resource reference](./README.md#links) for full field documentation.
- Explore the [`examples/`](../examples/) directory for ready-to-apply manifests.
