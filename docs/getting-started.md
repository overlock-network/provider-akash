# Getting started

## Prerequisites

- Kubernetes cluster with [Crossplane](https://crossplane.io) installed
- An Akash wallet with sufficient AKT balance (minimum 0.5 AKT per deployment deposit)
- Access to an Akash RPC node (defaults to `https://rpc.akashnet.io:443`)

## Install the provider

Using the Crossplane CLI:

```sh
crossplane xpkg install provider xpkg.upbound.io/overlock-network/provider-akash:v0.1.0
```

Or via a `Provider` manifest:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-akash
spec:
  package: xpkg.upbound.io/overlock-network/provider-akash:v0.1.0
```

Wait for the provider to become healthy:

```sh
kubectl get providers
```

## Configure credentials

Create a Secret containing your Akash wallet key (base64-encoded mnemonic or key export):

```sh
kubectl create secret generic akash-credentials \
  --from-literal=credentials="$(cat ~/.akash/key-export)" \
  --namespace crossplane-system
```

Create a `ProviderConfig` referencing the Secret:

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
      key: credentials
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
              amount: 100
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
    deposit: 5000000
```

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

### 4. Create a certificate (for mTLS)

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

### 5. Send the manifest

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
      name: my-app-<generated-by-bidpolicy>
    certificateSecretRef:
      name: my-app-cert-tls
      namespace: default
```

Once the `Manifest` is `Ready`, your workload is running on Akash. Check `status.atProvider.services` for URIs.

## Next steps

- See each [resource reference](./README.md#links) for full field documentation.
- Explore the [`examples/`](../examples/) directory for ready-to-apply manifests.
