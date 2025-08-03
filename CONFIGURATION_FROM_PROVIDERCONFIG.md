# AkashProviderConfiguration from ProviderConfig Implementation

## Overview
Successfully implemented automatic loading of AkashProviderConfiguration data from ProviderConfig, eliminating hardcoded configuration in controllers. All Akash-specific configuration properties are now provided through ProviderConfig properties with sensible defaults.

## Key Changes Made

### 1. Extended ProviderConfig Types (`apis/v1alpha1/providerconfig_types.go`)

#### New AkashConfiguration Struct
Added comprehensive Akash-specific configuration with defaults and validation:

```go
type AkashConfiguration struct {
    KeyName        *string `json:"keyName,omitempty"`        // default: "default"
    KeyringBackend *string `json:"keyringBackend,omitempty"` // default: "test", enum: os|file|test|memory
    AccountAddress *string `json:"accountAddress,omitempty"` // optional
    Net            *string `json:"net,omitempty"`            // default: "mainnet", enum: mainnet|testnet|sandbox
    Version        *string `json:"version,omitempty"`        // default: "0.18.0"
    ChainId        *string `json:"chainId,omitempty"`        // default: "akashnet-2"
    Node           *string `json:"node,omitempty"`           // default: "https://rpc.akashnet.io:443"
    Home           *string `json:"home,omitempty"`           // default: "/tmp/.akash"
    Path           *string `json:"path,omitempty"`           // default: "/usr/local/bin/akash"
    ProvidersApi   *string `json:"providersApi,omitempty"`   // default: "https://akash-api.polkachu.com"
}
```

#### Extended ProviderConfigSpec
```go
type ProviderConfigSpec struct {
    Credentials   ProviderCredentials  `json:"credentials"`
    Configuration *AkashConfiguration  `json:"configuration,omitempty"` // NEW
}
```

### 2. Enhanced AkashClient (`internal/controller/client/client.go`)

#### New Configuration Handling Types
- `AkashConfigurationSpec`: Mirror of ProviderConfig.AkashConfiguration for client use
- Updated `ProviderConfigInfo` to include configuration data
- `buildAkashProviderConfiguration()`: Converts spec to config with defaults
- `getStringValue()`: Helper for safe pointer dereferencing with defaults

#### Updated Constructor
```go
// NEW: No longer takes hardcoded config parameter
func NewFromManagedResource(ctx context.Context, kubeClient client.Client, usage resource.Tracker, mg resource.Managed, pcInfo ProviderConfigInfo) (*AkashClient, error)
```

**Key Features:**
- Automatically builds AkashProviderConfiguration from ProviderConfigInfo
- Uses sensible defaults for missing configuration fields
- Maintains credential loading and caching functionality
- Completely eliminates hardcoded configuration

### 3. Updated Deployment Controller (`internal/controller/deployment/deployment.go`)

#### Removed Hardcoded Configuration
**Before:**
```go
config := deployment.AkashProviderConfiguration{
    KeyName:        "default",           // HARDCODED
    KeyringBackend: "test",              // HARDCODED
    Net:            "mainnet",           // HARDCODED
    // ... more hardcoded values
}
```

**After:**
```go
// Configuration loaded from ProviderConfig.spec.configuration
var configSpec *deployment.AkashConfigurationSpec
if pc.Spec.Configuration != nil {
    configSpec = &deployment.AkashConfigurationSpec{
        KeyName:        pc.Spec.Configuration.KeyName,
        KeyringBackend: pc.Spec.Configuration.KeyringBackend,
        // ... all fields from ProviderConfig
    }
}
```

#### Updated Service Constructor
- Removed config parameter from `newDeploymentServiceFromManagedResource`
- Configuration now loaded from ProviderConfig automatically
- Controllers no longer need to provide any configuration

### 4. Generated CRDs (`package/crds/`)

#### Complete CRD Generation
- All new configuration fields properly generated with defaults
- Validation rules applied (enums, default values)
- Kubernetes-native configuration management
- OpenAPI schema with full documentation

### 5. Enhanced Examples

#### Comprehensive ProviderConfig Examples
- **Full Configuration** (`examples/provider/config.yaml`): Shows all available options
- **Minimal Configuration** (`examples/provider/config-minimal.yaml`): Uses defaults
- **Testnet Configuration** (`examples/provider/config-testnet.yaml`): Testnet-specific settings

#### Updated Usage Examples
- Demonstrates configuration loading from ProviderConfig
- Shows how defaults work when fields are omitted
- Explains the new declarative approach

## Configuration Options Available

### Network Settings
- `net`: "mainnet" | "testnet" | "sandbox" (default: "mainnet")
- `chainId`: Chain identifier (default: "akashnet-2")
- `node`: RPC endpoint (default: "https://rpc.akashnet.io:443")

### Key Management
- `keyName`: Key identifier (default: "default")
- `keyringBackend`: "os" | "file" | "test" | "memory" (default: "test")
- `accountAddress`: Specific account to use (optional)

### Binary and API Settings
- `version`: Akash version (default: "0.18.0")
- `path`: Binary path (default: "/usr/local/bin/akash")
- `home`: Config directory (default: "/tmp/.akash")
- `providersApi`: Providers API URL (default: "https://akash-api.polkachu.com")

## Usage Examples

### Minimal ProviderConfig (uses defaults)
```yaml
apiVersion: akash.web7.md/v1alpha1
kind: ProviderConfig
metadata:
  name: minimal
spec:
  credentials:
    source: Secret
    secretRef:
      name: akash-secret
      key: private-key
  # configuration omitted - defaults used
```

### Custom Configuration
```yaml
apiVersion: akash.web7.md/v1alpha1
kind: ProviderConfig
metadata:
  name: custom
spec:
  credentials:
    source: Secret
    secretRef:
      name: akash-secret
      key: private-key
  configuration:
    keyName: "production-key"
    keyringBackend: "os"
    net: "mainnet"
    chainId: "akashnet-2"
    node: "https://my-rpc.example.com:443"
```

### Testnet Configuration
```yaml
apiVersion: akash.web7.md/v1alpha1
kind: ProviderConfig
metadata:
  name: testnet
spec:
  credentials:
    source: Secret
    secretRef:
      name: testnet-secret
      key: private-key
  configuration:
    net: "testnet"
    chainId: "testnet-1"
    node: "https://rpc.testnet.akash.network:443"
```

## Benefits Achieved

### 1. **Declarative Configuration**
- Configuration managed through Kubernetes resources
- Version controlled with infrastructure as code
- Environment-specific configurations easily managed

### 2. **No Hardcoded Values**
- Controllers are completely configuration-agnostic
- All settings come from ProviderConfig
- Easy to update without code changes

### 3. **Sensible Defaults**
- Works out-of-the-box with minimal configuration
- Progressive disclosure - add config only when needed
- Mainnet defaults for production readiness

### 4. **Validation and Documentation**
- Kubernetes schema validation
- Enum validation for critical fields
- Self-documenting through CRD schema

### 5. **Environment Flexibility**
- Easy mainnet/testnet switching
- Custom RPC endpoints for private networks
- Different key management strategies per environment

## Migration Path

### For Existing Users
1. **No Breaking Changes**: Existing deployments continue to work
2. **Gradual Migration**: Add configuration to ProviderConfig over time
3. **Default Fallback**: Missing configuration uses sensible defaults

### For New Deployments
1. Create ProviderConfig with desired configuration
2. Reference from managed resources
3. Controller automatically uses configuration

## Testing

### Comprehensive Test Coverage
- Unit tests for configuration building logic
- Default value handling verification
- Pointer safety and nil handling
- Integration with existing deployment tests

### Validation
- All existing tests pass
- New configuration features tested
- CRD generation verified
- Example configurations validated

## Summary

The implementation successfully eliminates all hardcoded AkashProviderConfiguration data from controllers, making it completely declarative through ProviderConfig properties. Users can now configure all Akash-specific settings through Kubernetes resources with sensible defaults, providing a much more flexible and maintainable solution.

**Key Achievement**: Controllers now receive ready-to-use AkashClients with both credentials AND configuration automatically loaded from ProviderConfig, with zero hardcoded values.