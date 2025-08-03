package client

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	
	apisv1alpha1 "github.com/overlock-network/provider-akash/apis/v1alpha1"
)

func TestBuildAkashProviderConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		config   *apisv1alpha1.AkashConfiguration
		expected AkashProviderConfiguration
	}{
		{
			name:   "nil config uses constants for defaults",
			config: nil,
			expected: AkashProviderConfiguration{
				KeyName:        DefaultKeyName,
				KeyringBackend: DefaultKeyringBackend,
				Net:            DefaultNet,
				Version:        DefaultVersion,
				ChainId:        DefaultChainId,
				Node:           DefaultNode,
				Home:           DefaultHome,
				Path:           DefaultPath,
				ProvidersApi:   DefaultProvidersApi,
			},
		},
		{
			name: "partial config with custom values",
			config: &apisv1alpha1.AkashConfiguration{
				KeyName: stringPtr("custom-key"),
				Net:     stringPtr("testnet"),
				ChainId: stringPtr("testnet-1"),
				// Other fields nil - should use constants for defaults
			},
			expected: AkashProviderConfiguration{
				KeyName:        "custom-key",
				KeyringBackend: DefaultKeyringBackend,
				Net:            "testnet",
				Version:        DefaultVersion,
				ChainId:        "testnet-1",
				Node:           DefaultNode,
				Home:           DefaultHome,
				Path:           DefaultPath,
				ProvidersApi:   DefaultProvidersApi,
			},
		},
		{
			name: "all custom values",
			config: &apisv1alpha1.AkashConfiguration{
				KeyName:        stringPtr("my-key"),
				KeyringBackend: stringPtr("os"),
				AccountAddress: stringPtr("akash1234567890"),
				Net:            stringPtr("testnet"),
				Version:        stringPtr("0.20.0"),
				ChainId:        stringPtr("testnet-2"),
				Node:           stringPtr("https://custom-rpc.example.com:443"),
				Home:           stringPtr("/custom/.akash"),
				Path:           stringPtr("/custom/bin/akash"),
				ProvidersApi:   stringPtr("https://custom-api.example.com"),
			},
			expected: AkashProviderConfiguration{
				KeyName:        "my-key",
				KeyringBackend: "os",
				AccountAddress: "akash1234567890",
				Net:            "testnet",
				Version:        "0.20.0",
				ChainId:        "testnet-2",
				Node:           "https://custom-rpc.example.com:443",
				Home:           "/custom/.akash",
				Path:           "/custom/bin/akash",
				ProvidersApi:   "https://custom-api.example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildAkashProviderConfiguration(tt.config)
			if diff := cmp.Diff(tt.expected, result); diff != "" {
				t.Errorf("buildAkashProviderConfiguration() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetStringValue(t *testing.T) {
	tests := []struct {
		name         string
		ptr          *string
		defaultValue string
		expected     string
	}{
		{
			name:         "nil pointer returns default",
			ptr:          nil,
			defaultValue: "default-value",
			expected:     "default-value",
		},
		{
			name:         "non-nil pointer returns value",
			ptr:          stringPtr("custom-value"),
			defaultValue: "default-value",
			expected:     "custom-value",
		},
		{
			name:         "empty string pointer returns empty string",
			ptr:          stringPtr(""),
			defaultValue: "default-value",
			expected:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getStringValue(tt.ptr, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("getStringValue() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}