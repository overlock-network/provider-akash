# Deployment

**API:** `akash.overlock.network/v1alpha1`
**Kind:** `Deployment`
**Scope:** Cluster

Represents an on-chain Akash deployment. The controller submits a `MsgCreateDeployment` transaction when the resource is created and a `MsgCloseDeployment` when it is deleted.

## Spec fields (`forProvider`)

| Field | Required | Default | Description |
|---|---|---|---|
| `sdlRef.name` | yes | — | Name of the `SDL` CR |
| `sdlRef.namespace` | no | same namespace | Namespace of the `SDL` CR |
| `deposit` | no | `5000000` | Escrow deposit amount in `uact` (min `500000`). Denom is hard-coded to `uact` under v2 BME — only the integer amount is configurable. Gas for the broadcast is paid separately in `uakt`. |

## Status fields (`atProvider`)

| Field | Description |
|---|---|
| `deploymentId` | On-chain deployment sequence number (dseq) |
| `owner` | Wallet address that owns the deployment |
| `state` | Raw chain state: `active` or `closed` |
| `phase` | Derived lifecycle phase (see below) |
| `ordersOpen` | Number of open orders on-chain |
| `bidsOpen` | Number of bids currently in `Open` state |
| `bids` | Total bids ever recorded (stays >0 after bid window) |
| `leasesActive` | Number of active leases |
| `createdHeight` | Block height at creation |
| `escrowBalance` | Current escrow `{denom: uact, amount}` (denom is always `uact` under v2 BME) |

### Phase values

| Phase | Meaning |
|---|---|
| `Pending` | Not yet broadcast to chain |
| `Bidding` | Order open; providers are bidding |
| `Leased` | At least one active lease |
| `Expired` | No open orders and no active leases (bid window passed) |
| `Closed` | Chain closed the deployment (escrow drained or owner closed) |

## Minimal example

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
  writeConnectionSecretToRef:
    name: my-app-connection
    namespace: default
```

## Lifecycle notes

- **Create:** Broadcasts `MsgCreateDeployment` on-chain and transitions to `Bidding` once the order appears.
- **Update:** If the referenced SDL changes (hash differs), the controller broadcasts `MsgUpdateDeployment`.
- **Delete:** Broadcasts `MsgCloseDeployment`, which also closes any open orders; active leases are unaffected until their escrow runs out.
