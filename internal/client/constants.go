package client

// Default configuration constants for Akash provider
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