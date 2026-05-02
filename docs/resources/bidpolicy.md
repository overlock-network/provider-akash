# BidPolicy

**API:** `akash.overlock.network/v1alpha1`
**Kind:** `BidPolicy`
**Scope:** Cluster

Defines bid selection rules for one or more `Deployment` CRs. The controller watches incoming bids on-chain, filters them against the policy criteria, selects the best bid, and — when `autoAccept: true` — creates a `Lease` CR automatically.

## Spec fields (`forProvider`)

| Field | Required | Default | Description |
|---|---|---|---|
| `deploymentRef.name` | one of these | — | Target a single `Deployment` CR by name |
| `selector` | one of these | — | Label selector matching multiple `Deployment` CRs |
| `autoAccept` | no | `false` | Automatically create a `Lease` for the selected bid |
| `maxPrice` | no | — | Maximum acceptable bid price per block in `uact` (denom is `uact` under v2 BME) |
| `minProviderScore` | no | — | Minimum provider reputation score (0–100) |
| `requiredAttributes` | no | — | List of `{key, value}` provider attributes that must match |
| `excludedProviders` | no | — | Provider addresses to ignore |
| `preferredProviders` | no | — | Provider addresses given priority during selection |
| `selectionStrategy` | no | `lowest-price` | How to rank qualifying bids: `lowest-price`, `best-score`, `preferred-first` |
| `maxBids` | no | `10` | Stop collecting bids after this many are received |

## Status fields (`atProvider`)

| Field | Description |
|---|---|
| `state` | Policy state: `active`, `paused`, or `failed` |
| `matchedDeployments` | Deployment CRs matched by selector |
| `totalBidsReceived` | Total bids received across matched deployments |
| `eligibleBids` | Bids that passed all filter criteria |
| `selectedBids` | Map of deployment name to selected `ActiveBid` reference |
| `createdLeases` | Map of deployment name to created `Lease` reference |
| `rejectedBids` | List of rejected bids with reasons |
| `lastEvaluated` | Timestamp of last evaluation |

## Minimal example

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
    maxPrice: 500
```

## Example with label selector

```yaml
apiVersion: akash.overlock.network/v1alpha1
kind: BidPolicy
metadata:
  name: frontend-policy
spec:
  providerConfigRef:
    name: default
  forProvider:
    selector:
      matchLabels:
        tier: frontend
    selectionStrategy: best-score
    minProviderScore: 70
    requiredAttributes:
      - key: region
        value: us-west
    autoAccept: true
```

## Lifecycle notes

- **Create:** The controller begins watching bids for matched deployments. If `autoAccept: true`, it creates a `Lease` CR once a bid is selected.
- **Update:** Criteria changes take effect on the next evaluation cycle. Already-created Leases are not revoked.
- **Delete:** Stops bid monitoring. Leases previously created by this policy are not deleted.
