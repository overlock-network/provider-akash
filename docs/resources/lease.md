# Lease

**API:** `akash.overlock.network/v1alpha1`
**Kind:** `Lease`
**Scope:** Cluster

Represents an Akash lease — the agreement between a tenant and a provider to run a workload. The controller sends a `MsgCreateLease` transaction that accepts the chosen bid. Leases are typically created automatically by a `BidPolicy` with `autoAccept: true`, but can also be created manually.

## Spec fields (`forProvider`)

| Field | Required | Description |
|---|---|---|
| `deploymentRef.name` | yes | Name of the `Deployment` CR |
| `deploymentRef.namespace` | no | Namespace of the `Deployment` CR |
| `activeBidRef.name` | yes | Name of the `ActiveBid` CR to accept |
| `activeBidRef.namespace` | no | Namespace of the `ActiveBid` CR |
| `activeBidRef.bidId` | no | Bid ID (informational) |
| `activeBidRef.provider` | no | Provider address (informational) |
| `activeBidRef.price` | no | Bid price in uact (informational) |

## Status fields (`atProvider`)

| Field | Description |
|---|---|
| `leaseId` | Unique lease identifier on Akash (`owner/dseq/gseq/oseq/provider`) |
| `owner` | Deployment owner address |
| `dseq` | Deployment sequence number |
| `gseq` | Group sequence number |
| `oseq` | Order sequence number |
| `provider` | Provider address |
| `state` | Lease state: `active` or `closed` |
| `price` | Accepted bid price info |
| `createdAt` | Lease creation timestamp |
| `services` | Running services: name, availability, URIs, port mappings |

## Minimal example

```yaml
apiVersion: akash.overlock.network/v1alpha1
kind: Lease
metadata:
  name: my-app-lease
spec:
  providerConfigRef:
    name: default
  forProvider:
    deploymentRef:
      name: my-app
      namespace: default
    activeBidRef:
      name: my-app-activebid
      namespace: default
```

## Lifecycle notes

- **Create:** Sends `MsgCreateLease` on-chain, accepting the specified bid. The provider begins scheduling the workload.
- **Update:** Lease parameters cannot be changed after creation. The controller reconciles observed state only.
- **Delete:** Sends `MsgCloseLease` on-chain, stopping the workload on the provider. The escrow for that group is returned to the tenant.
