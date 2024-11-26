# Crossplane Akash Provider

This Crossplane provider enables you to manage and reconcile Akash Network resources, such as deployments, directly from your Kubernetes cluster using Crossplane.

## Features

- **Manage Akash Deployments**: Automate the creation, update, and deletion of Akash deployments.
- **Network Resource Reconciliation**: Seamlessly integrate Akash network resources into your Kubernetes environment.
- **Crossplane Integration**: Leverage Crossplane’s powerful composition and reconciliation features to manage your Akash resources declaratively.

## Getting Started

### Prerequisites

- [Crossplane](https://crossplane.io) installed in your Kubernetes cluster.
- Akash CLI configured and accessible from the Kubernetes nodes.


## Install

To install the Akash provider without modifications, use the Crossplane CLI in a Kubernetes cluster where Crossplane is installed:

```console
crossplane xpkg install provider xpkg.upbound.io/web7/provider-akash:v0.1.0
```

You can also manually install the Akash provider by creating a Provider directly:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-akash
spec:
  package: xpkg.upbound.io/web7/provider-akash:v0.1.0
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
- **Akash CLI**: Verify the state of your deployments using the Akash CLI.


## License

This project is licensed under the [Apache 2.0 License](LICENSE).