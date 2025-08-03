package client

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	
	apisv1alpha1 "github.com/overlock-network/provider-akash/apis/v1alpha1"
)

type AkashClient struct {
	ctx             context.Context
	Config          AkashProviderConfiguration
	transactionNote string
	
	// Kubernetes-based credential loading
	kubeClient       client.Client
	credentialCache  *credentialCache
	secretRef        *SecretReference
	managedResource  resource.Managed  // Managed resource with ProviderConfigReference
	usage            resource.Tracker  // For tracking ProviderConfig usage
}

type SecretReference struct {
	Name      string
	Namespace string
	Key       string
}

type credentialCache struct {
	mu          sync.RWMutex
	credentials []byte
	lastUpdated time.Time
	ttl         time.Duration
}

type AkashProviderConfiguration struct {
	Creds          []byte
	KeyName        string
	KeyringBackend string
	AccountAddress string
	Net            string
	Version        string
	ChainId        string
	Node           string
	Home           string
	Path           string
	ProvidersApi   string
}

func (ak *AkashClient) GetContext() context.Context {
	return ak.ctx
}

func (ak *AkashClient) GetPath() string {
	return ak.Config.Path
}

func (ak *AkashClient) SetGlobalTransactionNote(note string) {
	ak.transactionNote = note
}

// New creates a new AkashClient with direct credential configuration (legacy)
func New(ctx context.Context, configuration AkashProviderConfiguration) *AkashClient {
	return &AkashClient{ctx: ctx, Config: configuration}
}

// NewWithSecretRef creates a new AkashClient that loads credentials from a Kubernetes secret
func NewWithSecretRef(ctx context.Context, kubeClient client.Client, secretRef SecretReference, config AkashProviderConfiguration) *AkashClient {
	return &AkashClient{
		ctx:        ctx,
		Config:     config,
		kubeClient: kubeClient,
		secretRef:  &secretRef,
		credentialCache: &credentialCache{
			ttl: 5 * time.Minute, // Default TTL for credential cache
		},
	}
}

// ProviderConfigInfo contains the credentials and configuration information from a ProviderConfig
type ProviderConfigInfo struct {
	Source            xpv1.CredentialsSource
	CredentialSelectors xpv1.CommonCredentialSelectors
	Configuration     *apisv1alpha1.AkashConfiguration
}

// Helper function to get string value with default fallback
func getStringValue(ptr *string, defaultValue string) string {
	if ptr != nil {
		return *ptr
	}
	return defaultValue
}

// buildAkashProviderConfiguration converts AkashConfiguration to AkashProviderConfiguration with constants for defaults
func buildAkashProviderConfiguration(config *apisv1alpha1.AkashConfiguration) AkashProviderConfiguration {
	// Set defaults if config is nil
	if config == nil {
		return AkashProviderConfiguration{
			KeyName:        DefaultKeyName,
			KeyringBackend: DefaultKeyringBackend,
			Net:            DefaultNet,
			Version:        DefaultVersion,
			ChainId:        DefaultChainId,
			Node:           DefaultNode,
			Home:           DefaultHome,
			Path:           DefaultPath,
			ProvidersApi:   DefaultProvidersApi,
		}
	}
	
	// Build configuration with values from ProviderConfig, using constants for defaults
	return AkashProviderConfiguration{
		KeyName:        getStringValue(config.KeyName, DefaultKeyName),
		KeyringBackend: getStringValue(config.KeyringBackend, DefaultKeyringBackend),
		AccountAddress: getStringValue(config.AccountAddress, ""),
		Net:            getStringValue(config.Net, DefaultNet),
		Version:        getStringValue(config.Version, DefaultVersion),
		ChainId:        getStringValue(config.ChainId, DefaultChainId),
		Node:           getStringValue(config.Node, DefaultNode),
		Home:           getStringValue(config.Home, DefaultHome),
		Path:           getStringValue(config.Path, DefaultPath),
		ProvidersApi:   getStringValue(config.ProvidersApi, DefaultProvidersApi),
		// Creds will be set later when loaded
	}
}

// NewFromManagedResource creates a new AkashClient that automatically loads credentials 
// and configuration from the ProviderConfig referenced by the managed resource
func NewFromManagedResource(ctx context.Context, kubeClient client.Client, usage resource.Tracker, mg resource.Managed, pcInfo ProviderConfigInfo) (*AkashClient, error) {
	// Build AkashProviderConfiguration from ProviderConfigInfo
	config := buildAkashProviderConfiguration(pcInfo.Configuration)
	
	client := &AkashClient{
		ctx:             ctx,
		Config:          config,
		kubeClient:      kubeClient,
		managedResource: mg,
		usage:           usage,
		credentialCache: &credentialCache{
			ttl: 5 * time.Minute, // Default TTL for credential cache
		},
	}
	
	// Set up secret reference if using secrets
	if pcInfo.Source == xpv1.CredentialsSourceSecret && pcInfo.CredentialSelectors.SecretRef != nil {
		client.secretRef = &SecretReference{
			Name:      pcInfo.CredentialSelectors.SecretRef.Name,
			Namespace: pcInfo.CredentialSelectors.SecretRef.Namespace,
			Key:       pcInfo.CredentialSelectors.SecretRef.Key,
		}
	}
	
	// Load credentials immediately using the provided ProviderConfig info
	creds, err := resource.CommonCredentialExtractor(ctx, pcInfo.Source, kubeClient, pcInfo.CredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load credentials from ProviderConfig")
	}
	
	// Track ProviderConfig usage
	if usage != nil {
		if err := usage.Track(ctx, mg); err != nil {
			return nil, errors.Wrap(err, "cannot track ProviderConfig usage")
		}
	}
	
	// Set the credentials in config and cache
	client.Config.Creds = creds
	if client.credentialCache != nil {
		client.credentialCache.mu.Lock()
		client.credentialCache.credentials = creds
		client.credentialCache.lastUpdated = time.Now()
		client.credentialCache.mu.Unlock()
	}
	
	return client, nil
}

// NewFromProviderConfig creates a new AkashClient from ProviderConfig credentials (legacy)
func NewFromProviderConfig(ctx context.Context, kubeClient client.Client, credSource xpv1.CredentialsSource, credSelectors xpv1.CommonCredentialSelectors, config AkashProviderConfiguration) (*AkashClient, error) {
	// Extract credentials using Crossplane's standard method
	creds, err := resource.CommonCredentialExtractor(ctx, credSource, kubeClient, credSelectors)
	if err != nil {
		return nil, err
	}
	
	// If using Secret source, set up secret reference for future credential loading
	if credSource == xpv1.CredentialsSourceSecret && credSelectors.SecretRef != nil {
		secretRef := SecretReference{
			Name:      credSelectors.SecretRef.Name,
			Namespace: credSelectors.SecretRef.Namespace,
			Key:       credSelectors.SecretRef.Key,
		}
		
		client := NewWithSecretRef(ctx, kubeClient, secretRef, config)
		client.Config.Creds = creds
		return client, nil
	}
	
	// For non-secret sources, use direct credentials
	config.Creds = creds
	return New(ctx, config), nil
}

// GetCredentials returns the current credentials, loading from secret if needed
func (ak *AkashClient) GetCredentials() ([]byte, error) {
	// If no secret reference, return direct credentials
	if ak.secretRef == nil {
		return ak.Config.Creds, nil
	}
	
	// Check cache first
	if creds := ak.getCachedCredentials(); creds != nil {
		return creds, nil
	}
	
	// Load from secret and cache
	return ak.loadAndCacheCredentials()
}

// getCachedCredentials returns cached credentials if valid, nil otherwise
func (ak *AkashClient) getCachedCredentials() []byte {
	if ak.credentialCache == nil {
		return nil
	}
	
	ak.credentialCache.mu.RLock()
	defer ak.credentialCache.mu.RUnlock()
	
	if time.Since(ak.credentialCache.lastUpdated) > ak.credentialCache.ttl {
		return nil
	}
	
	return ak.credentialCache.credentials
}

// loadAndCacheCredentials loads credentials from the Kubernetes secret and caches them
func (ak *AkashClient) loadAndCacheCredentials() ([]byte, error) {
	if ak.kubeClient == nil || ak.secretRef == nil {
		return ak.Config.Creds, nil
	}
	
	// Create credential selectors from secret reference
	credSelectors := xpv1.CommonCredentialSelectors{
		SecretRef: &xpv1.SecretKeySelector{
			SecretReference: xpv1.SecretReference{
				Name:      ak.secretRef.Name,
				Namespace: ak.secretRef.Namespace,
			},
			Key: ak.secretRef.Key,
		},
	}
	
	// Load credentials from secret
	creds, err := resource.CommonCredentialExtractor(ak.ctx, xpv1.CredentialsSourceSecret, ak.kubeClient, credSelectors)
	if err != nil {
		return nil, err
	}
	
	// Cache the credentials
	if ak.credentialCache != nil {
		ak.credentialCache.mu.Lock()
		ak.credentialCache.credentials = creds
		ak.credentialCache.lastUpdated = time.Now()
		ak.credentialCache.mu.Unlock()
	}
	
	// Update config for immediate use
	ak.Config.Creds = creds
	
	return creds, nil
}

// RefreshCredentials forces a refresh of cached credentials from the secret
func (ak *AkashClient) RefreshCredentials() error {
	if ak.secretRef == nil {
		return nil // Nothing to refresh for direct credentials
	}
	
	_, err := ak.loadAndCacheCredentials()
	return err
}

// SetCredentialCacheTTL sets the time-to-live for credential caching
func (ak *AkashClient) SetCredentialCacheTTL(ttl time.Duration) {
	if ak.credentialCache != nil {
		ak.credentialCache.mu.Lock()
		ak.credentialCache.ttl = ttl
		ak.credentialCache.mu.Unlock()
	}
}
