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
crossplane xpkg install provider xpkg.upbound.io/overlock-network/provider-akash:v0.0.9
```

Or as a manifest:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-akash
spec:
  package: xpkg.upbound.io/overlock-network/provider-akash:v0.0.9
```

## Prerequisites

- A Kubernetes cluster with Crossplane installed.
- An Akash wallet funded with [minted ACT](https://akash.network/docs/developers/deployment/cli/act-mint-burn/) and AKT for gas.

## Configure

Create a `ProviderConfig` referencing a `Secret` that holds the wallet credentials. See examples under [`examples/provider`](https://github.com/overlock-network/provider-akash/tree/main/examples/provider).

## Usage

Once configured, the provider reconciles Akash resources based on your Kubernetes manifests:

- **Create** — submit a `Deployment` CR; the provider broadcasts the on-chain create-deployment transaction, places bids, accepts a lease, and delivers the manifest.
- **Update** — edits to the SDL or deployment spec propagate to the Akash provider.
- **Delete** — removing the Kubernetes resource closes the deployment on-chain.

Status is mirrored back into `status.atProvider` on every CR.

## Examples

See the [`examples/`](https://github.com/overlock-network/provider-akash/tree/main/examples) directory for sample configurations across deployments, bid policies, SDLs, and provider configs.

## Source

- Source: [github.com/overlock-network/provider-akash](https://github.com/overlock-network/provider-akash)
- Issues: [github.com/overlock-network/provider-akash/issues](https://github.com/overlock-network/provider-akash/issues)
- License: [Apache-2.0](https://github.com/overlock-network/provider-akash/blob/main/LICENSE)
