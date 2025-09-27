package client

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"crypto/tls"
	"net/http"

	deploymentcli "github.com/akash-network/akash-api/go/node/deployment/v1beta3"
	sdkclient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/std"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	clienttx "github.com/cosmos/cosmos-sdk/client/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/bank"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/pkg/errors"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"

	// Note: Using a simple client wrapper instead of akash-api client due to version incompatibilities
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	apisv1alpha1 "github.com/overlock-network/provider-akash/apis/v1alpha1"
)

const (
	// Akash network constants
	Bech32PrefixAccAddr = "akash"
)

// EncodingConfig specifies the concrete encoding types to use for a given app.
type EncodingConfig struct {
	InterfaceRegistry types.InterfaceRegistry
	Codec             codec.Codec
	TxConfig          sdkclient.TxConfig
	Amino             *codec.LegacyAmino
}

// makeEncodingConfig creates an EncodingConfig for the cosmos SDK
func makeEncodingConfig(moduleBasics ...interface{}) EncodingConfig {
	amino := codec.NewLegacyAmino()
	interfaceRegistry := types.NewInterfaceRegistry()
	codec := codec.NewProtoCodec(interfaceRegistry)
	txCfg := tx.NewTxConfig(codec, tx.DefaultSignModes)

	return EncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Codec:             codec,
		TxConfig:          txCfg,
		Amino:             amino,
	}
}

type AkashClient struct {
	ctx             context.Context
	Config          AkashProviderConfiguration
	transactionNote string

	// Kubernetes-based credential loading
	kubeClient      client.Client
	credentialCache *credentialCache
	secretRef       *SecretReference
	managedResource resource.Managed // Managed resource with ProviderConfigReference
	usage           resource.Tracker // For tracking ProviderConfig usage

	// Development tracking for sent deployment IDs to prevent false updates
	sentDeploymentIDs map[string]bool
	mu               sync.RWMutex
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
	Creds               []byte
	Passphrase          []byte
	KeyName             string
	KeyringBackend      string
	AccountAddress      string
	Net                 string
	Version             string
	ChainId             string
	Node                string
	Home                string
	Path                string
	ProvidersApi        string
	SkipTLSVerification bool
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
	return &AkashClient{
		ctx:               ctx, 
		Config:            configuration,
		sentDeploymentIDs: make(map[string]bool),
	}
}

// NewWithSecretRef creates a new AkashClient that loads credentials from a Kubernetes secret
func NewWithSecretRef(ctx context.Context, kubeClient client.Client, secretRef SecretReference, config AkashProviderConfiguration) *AkashClient {
	return &AkashClient{
		ctx:               ctx,
		Config:            config,
		kubeClient:        kubeClient,
		secretRef:         &secretRef,
		sentDeploymentIDs: make(map[string]bool),
		credentialCache: &credentialCache{
			ttl: 5 * time.Minute, // Default TTL for credential cache
		},
	}
}

// ProviderConfigInfo contains the credentials and configuration information from a ProviderConfig
type ProviderConfigInfo struct {
	Source              xpv1.CredentialsSource
	CredentialSelectors xpv1.CommonCredentialSelectors
	PassphraseSource    *xpv1.CredentialsSource
	PassphraseSelectors *xpv1.CommonCredentialSelectors
	Configuration       *apisv1alpha1.AkashConfiguration
}

// Helper function to get string value with default fallback
func getStringValue(ptr *string, defaultValue string) string {
	if ptr != nil {
		return *ptr
	}
	return defaultValue
}

// Helper function to get bool value with default fallback
func getBoolValue(ptr *bool, defaultValue bool) bool {
	if ptr != nil {
		return *ptr
	}
	return defaultValue
}

// QueryClient provides query functionality
type QueryClient struct {
	clientCtx sdkclient.Context
}

// Deployment returns the deployment query client
func (q *QueryClient) Deployment() deploymentcli.QueryClient {
	return deploymentcli.NewQueryClient(q.clientCtx)
}

// Auth returns the auth query client
func (q *QueryClient) Auth() authtypes.QueryClient {
	return authtypes.NewQueryClient(q.clientCtx)
}

// Bank returns the bank query client
func (q *QueryClient) Bank() banktypes.QueryClient {
	return banktypes.NewQueryClient(q.clientCtx)
}

// Staking returns the staking query client
func (q *QueryClient) Staking() stakingtypes.QueryClient {
	return stakingtypes.NewQueryClient(q.clientCtx)
}

// TxClient provides transaction functionality
type TxClient struct {
	clientCtx sdkclient.Context
}

// BroadcastMsgs broadcasts messages using the client context
func (t *TxClient) BroadcastMsgs(ctx context.Context, msgs ...sdktypes.Msg) (*sdktypes.TxResponse, error) {
	// Build transaction with proper gas and fees
	gasLimit := uint64(200000)
	gasPrice := sdktypes.NewDecWithPrec(25, 4) // 0.0025 AKT per gas unit
	feeAmount := gasPrice.MulInt64(int64(gasLimit)).TruncateInt()

	// Ensure minimum fee of 500uakt
	minFee := sdktypes.NewInt(500)
	if feeAmount.LT(minFee) {
		feeAmount = minFee
	}

	fees := sdktypes.NewCoins(sdktypes.NewCoin("uakt", feeAmount))

	// Check if we have proper client context setup
	if t.clientCtx.GetFromAddress().Empty() {
		return nil, fmt.Errorf("no from address configured in client context")
	}

	if t.clientCtx.Keyring == nil {
		return nil, fmt.Errorf("no keyring configured in client context")
	}

	// Log transaction details for debugging
	fmt.Printf("🚀 Preparing transaction - Chain ID: %s, From: %s\n", 
		t.clientCtx.ChainID, t.clientCtx.GetFromAddress().String())

	// Create transaction factory for signing and broadcasting
	txf := clienttx.Factory{}.
		WithTxConfig(t.clientCtx.TxConfig).
		WithAccountRetriever(t.clientCtx.AccountRetriever).
		WithChainID(t.clientCtx.ChainID).
		WithKeybase(t.clientCtx.Keyring).
		WithGas(gasLimit).
		WithFees(fees.String()).
		WithSignMode(t.clientCtx.TxConfig.SignModeHandler().DefaultMode())

	// Build, sign and broadcast the transaction
	return t.broadcastTxWithFactory(txf, msgs...)
}

// broadcastTxWithFactory handles the actual transaction building, signing and broadcasting
func (t *TxClient) broadcastTxWithFactory(txf clienttx.Factory, msgs ...sdktypes.Msg) (*sdktypes.TxResponse, error) {
	// Prepare the factory by fetching account info from the network
	txf, err := t.prepareFactory(txf)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare factory: %w", err)
	}

	// Build the unsigned transaction
	txBuilder, err := txf.BuildUnsignedTx(msgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to build unsigned transaction: %w", err)
	}

	// Sign the transaction
	err = clienttx.Sign(txf, t.clientCtx.GetFromName(), txBuilder, true)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Encode the transaction
	txBytes, err := t.clientCtx.TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return nil, fmt.Errorf("failed to encode transaction: %w", err)
	}

	// Broadcast the transaction
	res, err := t.clientCtx.BroadcastTx(txBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to broadcast transaction: %w", err)
	}

	return res, nil
}

// prepareFactory ensures the account exists and fetches account number and sequence
func (t *TxClient) prepareFactory(txf clienttx.Factory) (clienttx.Factory, error) {
	from := t.clientCtx.GetFromAddress()

	// Ensure the account exists
	if err := txf.AccountRetriever().EnsureExists(t.clientCtx, from); err != nil {
		return txf, fmt.Errorf("account does not exist: %w", err)
	}

	// Get account number and sequence from the network
	num, seq, err := txf.AccountRetriever().GetAccountNumberSequence(t.clientCtx, from)
	if err != nil {
		return txf, fmt.Errorf("failed to get account number and sequence: %w", err)
	}

	// Log the account information for debugging
	fmt.Printf("🔐 Account info - Address: %s, Number: %d, Sequence: %d, Chain-ID: %s\n", 
		from.String(), num, seq, t.clientCtx.ChainID)

	// Update factory with the fetched account information
	txf = txf.WithAccountNumber(num).WithSequence(seq)

	return txf, nil
}

// NodeClient provides Akash node client functionality
type NodeClient struct {
	ctx       context.Context
	clientCtx sdkclient.Context
	query     *QueryClient
	tx        *TxClient
}

// NewNodeClient creates a new NodeClient
func NewNodeClient(ctx context.Context, clientCtx sdkclient.Context) *NodeClient {
	return &NodeClient{
		ctx:       ctx,
		clientCtx: clientCtx,
		query:     &QueryClient{clientCtx: clientCtx},
		tx:        &TxClient{clientCtx: clientCtx},
	}
}

// Context returns the client context
func (c *NodeClient) Context() sdkclient.Context {
	return c.clientCtx
}

// GetContext returns the Go context
func (c *NodeClient) GetContext() context.Context {
	return c.ctx
}

// Query returns the query client
func (c *NodeClient) Query() *QueryClient {
	return c.query
}

// Tx returns the transaction client
func (c *NodeClient) Tx() *TxClient {
	return c.tx
}

// buildAkashProviderConfiguration converts AkashConfiguration to AkashProviderConfiguration with constants for defaults
func buildAkashProviderConfiguration(config *apisv1alpha1.AkashConfiguration) AkashProviderConfiguration {
	// Set defaults if config is nil
	if config == nil {
		return AkashProviderConfiguration{
			KeyName:             DefaultKeyName,
			KeyringBackend:      DefaultKeyringBackend,
			Net:                 DefaultNet,
			Version:             DefaultVersion,
			ChainId:             DefaultChainId,
			Node:                DefaultNode,
			Home:                DefaultHome,
			Path:                DefaultPath,
			ProvidersApi:        DefaultProvidersApi,
			SkipTLSVerification: false,
		}
	}

	// Build configuration with values from ProviderConfig, using constants for defaults
	return AkashProviderConfiguration{
		KeyName:             getStringValue(config.KeyName, DefaultKeyName),
		KeyringBackend:      getStringValue(config.KeyringBackend, DefaultKeyringBackend),
		AccountAddress:      getStringValue(config.AccountAddress, ""),
		Net:                 getStringValue(config.Net, DefaultNet),
		Version:             getStringValue(config.Version, DefaultVersion),
		ChainId:             getStringValue(config.ChainId, DefaultChainId),
		Node:                getStringValue(config.Node, DefaultNode),
		Home:                getStringValue(config.Home, DefaultHome),
		Path:                getStringValue(config.Path, DefaultPath),
		ProvidersApi:        getStringValue(config.ProvidersApi, DefaultProvidersApi),
		SkipTLSVerification: getBoolValue(config.SkipTLSVerification, false),
		// Creds will be set later when loaded
	}
}

// NewFromManagedResource creates a new AkashClient that automatically loads credentials
// and configuration from the ProviderConfig referenced by the managed resource
func NewFromManagedResource(ctx context.Context, kubeClient client.Client, usage resource.Tracker, mg resource.Managed, pcInfo ProviderConfigInfo) (*AkashClient, error) {
	// Build AkashProviderConfiguration from ProviderConfigInfo
	config := buildAkashProviderConfiguration(pcInfo.Configuration)

	client := &AkashClient{
		ctx:               ctx,
		Config:            config,
		kubeClient:        kubeClient,
		managedResource:   mg,
		usage:             usage,
		sentDeploymentIDs: make(map[string]bool),
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

	// Load passphrase if configured
	var passphrase []byte
	if pcInfo.PassphraseSource != nil && pcInfo.PassphraseSelectors != nil {
		passphrase, err = resource.CommonCredentialExtractor(ctx, *pcInfo.PassphraseSource, kubeClient, *pcInfo.PassphraseSelectors)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load passphrase from ProviderConfig")
		}
	}

	// Track ProviderConfig usage
	if usage != nil {
		if err := usage.Track(ctx, mg); err != nil {
			return nil, errors.Wrap(err, "cannot track ProviderConfig usage")
		}
	}

	// Set the credentials and passphrase in config and cache
	client.Config.Creds = creds
	client.Config.Passphrase = passphrase
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

// getNodeClient creates and returns an Akash node client using the stored credentials
func (ak *AkashClient) getNodeClient() (*NodeClient, error) {
	creds, err := ak.GetCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get credentials: %w", err)
	}

	if len(creds) == 0 {
		return nil, fmt.Errorf("no credentials available")
	}

	// Create encoding configuration using cosmos-sdk directly
	// since akash-api types may have version incompatibility
	encodingConfig := makeEncodingConfig(
		auth.AppModuleBasic{},
		bank.AppModuleBasic{},
	)
	// Register cosmos-sdk interfaces
	std.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	
	// Register auth module types specifically for account handling
	authtypes.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	
	// Register bank module types  
	banktypes.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	
	// Register staking module types (may be needed for account queries)
	stakingtypes.RegisterInterfaces(encodingConfig.InterfaceRegistry)

	// Use the properly configured codec from Akash
	cdc := encodingConfig.Codec
	txConfig := encodingConfig.TxConfig
	interfaceRegistry := encodingConfig.InterfaceRegistry

	// Create account retriever for querying account info
	accountRetriever := authtypes.AccountRetriever{}

	// Create RPC client for node communication with configurable TLS verification
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: ak.Config.SkipTLSVerification,
			},
		},
	}
	rpcClient, err := rpchttp.NewWithClient(ak.Config.Node, "/websocket", httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create RPC client: %w", err)
	}

	kr := keyring.NewInMemory()

	passphrase := ""
	if len(ak.Config.Passphrase) > 0 {
		passphrase = string(ak.Config.Passphrase)
	}

	err = kr.ImportPrivKey(ak.Config.KeyName, string(creds), passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to import private key: %w", err)
	}

	clientCtx := sdkclient.Context{}.
		WithKeyring(kr).
		WithChainID(ak.Config.ChainId).
		WithNodeURI(ak.Config.Node).
		WithClient(rpcClient).
		WithCodec(cdc).
		WithInterfaceRegistry(interfaceRegistry).
		WithBroadcastMode(flags.BroadcastSync).
		WithFromName(ak.Config.KeyName).
		WithFromAddress(nil).
		WithSkipConfirmation(true).
		WithTxConfig(txConfig).
		WithSignModeStr("direct").
		WithAccountRetriever(accountRetriever).
		WithInput(nil).
		WithOutput(nil).
		WithViper("")

	if ak.Config.AccountAddress != "" {
		// Validate that the address uses the Akash prefix
		if !strings.HasPrefix(ak.Config.AccountAddress, Bech32PrefixAccAddr) {
			return nil, fmt.Errorf("account address must use Akash prefix '%s', got: %s", Bech32PrefixAccAddr, ak.Config.AccountAddress)
		}

		// Parse the account address directly using SDK types
		accAddr, err := sdktypes.AccAddressFromBech32(ak.Config.AccountAddress)
		if err != nil {
			return nil, fmt.Errorf("invalid account address %s: %w", ak.Config.AccountAddress, err)
		}
		clientCtx = clientCtx.WithFromAddress(accAddr)
	}

	// Create node client wrapper around the clientCtx
	client := NewNodeClient(ak.ctx, clientCtx)

	return client, nil
}
