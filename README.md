# Crossplane Akash Network Provider
<p align="center"><img src="assets/akash-crossplane.png" alt="Akash Crossplane Logo" width="50%"></p>

This Crossplane provider enables you to manage and reconcile Akash Network resources, such as deployments, directly from your Kubernetes cluster using Crossplane.

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
crossplane xpkg install provider xpkg.upbound.io/overlock-network/provider-akash:v0.0.9
```

You can also manually install the Akash provider by creating a Provider directly:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-akash
spec:
  package: xpkg.upbound.io/overlock-network/provider-akash:v0.0.9
```

## Usage

Once installed and configured, the Crossplane Akash provider will reconcile Akash network resources based on your Kubernetes manifests.

- **Create**: New resources will be created on the Akash network.
- **Update**: Any changes in the manifest will be reflected on the Akash deployment.
- **Delete**: Deleting the Kubernetes resource will clean up the corresponding Akash resource.

## Examples

Check out the `examples/` directory for more sample configurations and usage scenarios.

## Troubleshooting

- **Logs**: Check the Crossplane provider logs for any errors during reconciliation.
- **Status**: Verify the state of your resources via `kubectl get/describe` — each CR's `status.atProvider` reflects the live on-chain state.


## License

This project is licensed under the [Apache 2.0 License](LICENSE).