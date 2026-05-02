# provider-akash

`provider-akash` is a [Crossplane](https://crossplane.io) provider that lets you manage [Akash Network](https://akash.network) resources declaratively from Kubernetes.

## What it does

The provider reconciles Kubernetes custom resources against the Akash Network chain and provider API:

- Submits and closes deployments on-chain
- Monitors bids and applies selection policies
- Creates leases by accepting chosen bids
- Sends manifests to Akash providers over mTLS
- Manages TLS certificates used for provider communication

## Resource overview

| Resource | API group | Scope | Purpose |
|---|---|---|---|
| `ProviderConfig` | `akash.overlock.network/v1alpha1` | Cluster | Akash credentials and node settings |
| `SDL` | `akash.overlock.network/v1alpha1` | Namespaced | Stack Definition Language workload spec |
| `Deployment` | `akash.overlock.network/v1alpha1` | Cluster | On-chain deployment (references SDL) |
| `BidPolicy` | `akash.overlock.network/v1alpha1` | Cluster | Bid selection rules; optionally auto-creates Leases |
| `Certificate` | `akash.overlock.network/v1alpha1` | Cluster | TLS certificate for provider mTLS |
| `Lease` | `akash.overlock.network/v1alpha1` | Cluster | Accepted lease for a Deployment/Bid pair |
| `Manifest` | `akash.overlock.network/v1alpha1` | Cluster | Sends the workload manifest to the provider |

## Typical resource flow

```
SDL --> Deployment --> (BidPolicy selects bid) --> Lease --> Manifest
                                                    ^
                                              Certificate
```

## Links

- [Getting started](./getting-started.md)
- [ProviderConfig](./resources/providerconfig.md)
- [SDL](./resources/sdl.md)
- [Deployment](./resources/deployment.md)
- [BidPolicy](./resources/bidpolicy.md)
- [Certificate](./resources/certificate.md)
- [Lease](./resources/lease.md)
- [Manifest](./resources/manifest.md)
- [Examples](../examples/)
- [Akash Network docs](https://akash.network/docs)
