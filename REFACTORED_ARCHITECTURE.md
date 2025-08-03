# Refactored Architecture: AkashProviderConfiguration Encapsulation and Constants

## Overview
Successfully refactored the codebase to eliminate AkashProviderConfiguration usage outside of AkashClient and implemented constants for repeated values, following clean architecture principles.

## Key Architectural Improvements

### 1. **Constants-Based Configuration** (`internal/controller/client/constants.go`)

All repeated configuration values are now defined as constants:

```go
const (
    // Default key and keyring settings
    DefaultKeyName        = "default"
    DefaultKeyringBackend = "test"
    
    // Default network settings
    DefaultNet     = "mainnet"
    DefaultChainId = "akashnet-2"
    DefaultNode    = "https://rpc.akashnet.io:443"
    
    // Default version and paths
    DefaultVersion     = "0.18.0"
    DefaultHome        = "/tmp/.akash"
    DefaultPath        = "/usr/local/bin/akash"
    DefaultProvidersApi = "https://akash-api.polkachu.com"
    
    // Validation constants
    KeyringBackendOS     = "os"
    KeyringBackendFile   = "file"
    KeyringBackendTest   = "test"
    KeyringBackendMemory = "memory"
    
    NetworkMainnet = "mainnet"
    NetworkTestnet = "testnet"
    NetworkSandbox = "sandbox"
)
```

**Benefits:**
- Single source of truth for all default values
- Easy maintenance and updates
- Type safety and consistency
- Prevents duplication across codebase

### 2. **Encapsulated AkashProviderConfiguration**

**Before:** AkashProviderConfiguration was exposed and used throughout controllers
**After:** AkashProviderConfiguration is only used internally within AkashClient

#### Clean Interface Boundaries
```go
// OUTSIDE AkashClient - Only ProviderConfig types used
type ProviderConfigInfo struct {
    Source            xpv1.CredentialsSource
    CredentialSelectors xpv1.CommonCredentialSelectors
    Configuration     *apisv1alpha1.AkashConfiguration  // ProviderConfig type
}

// INSIDE AkashClient - AkashProviderConfiguration used internally
type AkashClient struct {
    Config AkashProviderConfiguration  // Internal only
    // ... other fields
}
```

### 3. **Direct ProviderConfig Type Usage**

Controllers now work directly with ProviderConfig types:

```go
// In deployment controller - no more hardcoded AkashProviderConfiguration
pcInfo := deployment.ProviderConfigInfo{
    Source:              cd.Source,
    CredentialSelectors: cd.CommonCredentialSelectors,
    Configuration:       pc.Spec.Configuration, // Direct ProviderConfig type usage
}
```

### 4. **Conversion Layer with Constants**

Internal conversion uses constants for defaults:

```go
func buildAkashProviderConfiguration(config *apisv1alpha1.AkashConfiguration) AkashProviderConfiguration {
    if config == nil {
        return AkashProviderConfiguration{
            KeyName:        DefaultKeyName,        // Constant
            KeyringBackend: DefaultKeyringBackend, // Constant
            Net:            DefaultNet,            // Constant
            // ... all constants
        }
    }
    
    return AkashProviderConfiguration{
        KeyName:        getStringValue(config.KeyName, DefaultKeyName),        // Constant
        KeyringBackend: getStringValue(config.KeyringBackend, DefaultKeyringBackend), // Constant
        // ... all with constants
    }
}
```

## Architecture Layers

### Layer 1: ProviderConfig API Types (`apis/v1alpha1/`)
- `AkashConfiguration`: Kubernetes CRD type
- Kubebuilder validation and defaults
- User-facing configuration interface

### Layer 2: Controller Layer (`internal/controller/deployment/`)
- Works exclusively with ProviderConfig types
- No AkashProviderConfiguration exposure
- Uses constants for fallback scenarios
- Clean separation of concerns

### Layer 3: Client Interface (`internal/controller/client/`)
- `ProviderConfigInfo`: Bridge type using ProviderConfig types
- Constants for all default values
- Internal conversion to AkashProviderConfiguration

### Layer 4: AkashClient Implementation
- AkashProviderConfiguration used internally only
- Automatic conversion from ProviderConfig types
- Encapsulated configuration management

## Data Flow Architecture

```
ProviderConfig (K8s CRD)
         ↓
ProviderConfigInfo (Bridge Type)
         ↓
buildAkashProviderConfiguration() + Constants
         ↓
AkashProviderConfiguration (Internal Only)
         ↓
AkashClient (Encapsulated)
```

## Benefits Achieved

### 1. **Clean Separation of Concerns**
- ProviderConfig types: External API and validation
- AkashProviderConfiguration: Internal client implementation
- Constants: Centralized default management

### 2. **Type Safety and Consistency**
- Constants prevent typos and inconsistencies
- Compile-time validation of default values
- Single source of truth for configuration values

### 3. **Maintainability**
- Configuration changes require updates in one place only
- Clear interface boundaries
- Reduced coupling between layers

### 4. **Testability**
- Constants can be imported and tested
- Clear interfaces enable easy mocking
- Isolated concerns for unit testing

### 5. **Backward Compatibility**
- Legacy constructors maintained for existing users
- Gradual migration path available
- No breaking changes to existing deployments

## Usage Patterns

### ✅ **Recommended Pattern** (Controllers)
```go
// Controllers use ProviderConfig types directly
pcInfo := ProviderConfigInfo{
    Source:              providerConfig.Spec.Credentials.Source,
    CredentialSelectors: providerConfig.Spec.Credentials.CommonCredentialSelectors,
    Configuration:       providerConfig.Spec.Configuration, // ProviderConfig type
}

client, err := NewFromManagedResource(ctx, kubeClient, usage, mg, pcInfo)
```

### ✅ **Acceptable Pattern** (Constants)
```go
// When fallback defaults needed, use constants
config := AkashProviderConfiguration{
    KeyName:        DefaultKeyName,
    KeyringBackend: DefaultKeyringBackend,
    Net:            DefaultNet,
    // ... all constants
}
```

### ❌ **Discouraged Pattern** (Hardcoded Values)
```go
// Don't hardcode values anymore
config := AkashProviderConfiguration{
    KeyName:        "default",     // AVOID
    KeyringBackend: "test",        // AVOID
    Net:            "mainnet",     // AVOID
    // ... hardcoded values
}
```

## Migration Summary

### What Changed
1. **Added constants** for all repeated configuration values
2. **Eliminated AkashProviderConfiguration** from controller interfaces
3. **Direct ProviderConfig type usage** in controllers
4. **Encapsulated conversion logic** within client package
5. **Maintained backward compatibility** for existing code

### What Stayed the Same
- AkashClient internal structure (maintains compatibility)
- ProviderConfig API schema (no breaking changes)
- Legacy constructor availability (for migration)
- Core functionality and behavior

## Result

The architecture now properly encapsulates AkashProviderConfiguration within AkashClient, uses constants for all repeated values, and maintains clean separation between external API types (ProviderConfig) and internal implementation details. This provides better maintainability, type safety, and follows clean architecture principles while maintaining full backward compatibility.