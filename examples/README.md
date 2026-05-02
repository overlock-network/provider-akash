# Examples

This directory contains example manifests for resources you create directly.
Resources created automatically by the controllers (ActiveBid, Lease, Certificate, Manifest) are not included here.

## Apply order

Resources must be applied in this order because each depends on the previous:

### 1. ProviderConfig

Configure your Akash wallet credentials. Choose mainnet or sandbox:

```bash
# Mainnet
kubectl apply -f examples/provider/config.yaml

# Sandbox (for testing)
kubectl apply -f examples/provider/config-sandbox.yaml
```

Edit the Secret's `credentials` field with your wallet mnemonic before applying.

### 2. SDL

Define the workload (services, compute resources, placement, pricing):

```bash
# Simple nginx web server
kubectl apply -f examples/sdl/sdl-simple.yaml

# Multi-service (frontend + backend)
kubectl apply -f examples/sdl/sdl-multi-service.yaml

# GPU workload (Jupyter with NVIDIA GPU)
kubectl apply -f examples/sdl/sdl-gpu.yaml

# Persistent storage (PostgreSQL)
kubectl apply -f examples/sdl/sdl-persistent-storage.yaml
```

### 3. Deployment

Create the on-chain deployment, referencing the SDL above:

```bash
kubectl apply -f examples/deployment/deployment.yaml
```

### 4. BidPolicy

Define how to select a provider bid. The controller evaluates incoming bids
against the policy and, if `autoAccept` is enabled, creates a Lease automatically:

```bash
# Lowest-price selection across all matching deployments
kubectl apply -f examples/bidpolicy/bidpolicy.yaml

# Auto-accept the lowest bid for a specific deployment
kubectl apply -f examples/bidpolicy/bidpolicy-auto-accept.yaml

# Filter by provider attributes and reputation score
kubectl apply -f examples/bidpolicy/bidpolicy-attribute-filter.yaml

# Reject bids above a price cap
kubectl apply -f examples/bidpolicy/bidpolicy-price-cap.yaml
```

## File layout

```
examples/
├── provider/
│   ├── config.yaml               # ProviderConfig — mainnet
│   └── config-sandbox.yaml       # ProviderConfig — sandbox
├── sdl/
│   ├── sdl-simple.yaml           # Simple nginx web server
│   ├── sdl-multi-service.yaml    # Frontend + backend services
│   ├── sdl-gpu.yaml              # GPU workload (Jupyter)
│   └── sdl-persistent-storage.yaml # Persistent storage (PostgreSQL)
├── deployment/
│   └── deployment.yaml           # Deployment referencing an SDL
└── bidpolicy/
    ├── bidpolicy.yaml                  # Lowest-price selection
    ├── bidpolicy-auto-accept.yaml      # Auto-accept lowest bid
    ├── bidpolicy-attribute-filter.yaml # Attribute + score filter
    └── bidpolicy-price-cap.yaml        # Maximum price cap
```
