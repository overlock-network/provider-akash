package client

import (
	"testing"

	"github.com/overlock-network/provider-akash/internal/client/types"
)

func TestSDLValidation(t *testing.T) {
	client := &AkashClient{}

	testCases := []struct {
		name        string
		sdl         *types.SDL
		wantErrors  int
		expectError string
	}{
		{
			name: "valid SDL",
			sdl: &types.SDL{
				Version: "2.0",
				Services: map[string]types.SDLService{
					"web": {
						Image: "nginx:1.21.6",
						Expose: []types.SDLExposeSpec{
							{Port: 80, Proto: "tcp"},
						},
					},
				},
				Profiles: types.SDLProfiles{
					Compute: map[string]types.SDLComputeProfile{
						"web": {
							Resources: types.SDLResources{
								CPU:     types.SDLResourceCPU{Units: "0.5"},
								Memory:  types.SDLResourceMemory{Size: "512Mi"},
								Storage: types.SDLResourceStorage{Size: "1Gi"},
							},
						},
					},
					Placement: map[string]types.SDLPlacementProfile{
						"westcoast": {
							Pricing: map[string]types.SDLPricing{
								"web": {Denom: "uakt", Amount: 100},
							},
						},
					},
				},
				Deployment: map[string]types.SDLDeploymentGroup{
					"web": {Profile: "westcoast", Count: 1},
				},
			},
			wantErrors: 0,
		},
		{
			name: "invalid version",
			sdl: &types.SDL{
				Version: "1.0",
				Services: map[string]types.SDLService{
					"web": {Image: "nginx:1.21.6"},
				},
				Profiles: types.SDLProfiles{
					Compute: map[string]types.SDLComputeProfile{
						"web": {
							Resources: types.SDLResources{
								CPU:     types.SDLResourceCPU{Units: "0.5"},
								Memory:  types.SDLResourceMemory{Size: "512Mi"},
								Storage: types.SDLResourceStorage{Size: "1Gi"},
							},
						},
					},
					Placement: map[string]types.SDLPlacementProfile{
						"westcoast": {
							Pricing: map[string]types.SDLPricing{
								"web": {Denom: "uakt", Amount: 100},
							},
						},
					},
				},
				Deployment: map[string]types.SDLDeploymentGroup{
					"web": {Profile: "westcoast", Count: 1},
				},
			},
			wantErrors:  1,
			expectError: "version must be '2.0'",
		},
		{
			name: "empty services",
			sdl: &types.SDL{
				Version:  "2.0",
				Services: map[string]types.SDLService{},
				Profiles: types.SDLProfiles{
					Compute:   map[string]types.SDLComputeProfile{},
					Placement: map[string]types.SDLPlacementProfile{},
				},
				Deployment: map[string]types.SDLDeploymentGroup{},
			},
			wantErrors: 4, // empty services, empty compute, empty placement, empty deployment
		},
		{
			name: "invalid service name",
			sdl: &types.SDL{
				Version: "2.0",
				Services: map[string]types.SDLService{
					"Web-Service": {Image: "nginx:1.21.6"}, // Invalid name with capital and dash
				},
				Profiles: types.SDLProfiles{
					Compute: map[string]types.SDLComputeProfile{
						"web": {
							Resources: types.SDLResources{
								CPU:     types.SDLResourceCPU{Units: "0.5"},
								Memory:  types.SDLResourceMemory{Size: "512Mi"},
								Storage: types.SDLResourceStorage{Size: "1Gi"},
							},
						},
					},
					Placement: map[string]types.SDLPlacementProfile{
						"westcoast": {
							Pricing: map[string]types.SDLPricing{
								"web": {Denom: "uakt", Amount: 100},
							},
						},
					},
				},
				Deployment: map[string]types.SDLDeploymentGroup{
					"Web-Service": {Profile: "westcoast", Count: 1},
				},
			},
			wantErrors:  1,
			expectError: "service name 'Web-Service' is invalid",
		},
		{
			name: "invalid port range",
			sdl: &types.SDL{
				Version: "2.0",
				Services: map[string]types.SDLService{
					"web": {
						Image: "nginx:1.21.6",
						Expose: []types.SDLExposeSpec{
							{Port: 70000}, // Invalid port > 65535
						},
					},
				},
				Profiles: types.SDLProfiles{
					Compute: map[string]types.SDLComputeProfile{
						"web": {
							Resources: types.SDLResources{
								CPU:     types.SDLResourceCPU{Units: "0.5"},
								Memory:  types.SDLResourceMemory{Size: "512Mi"},
								Storage: types.SDLResourceStorage{Size: "1Gi"},
							},
						},
					},
					Placement: map[string]types.SDLPlacementProfile{
						"westcoast": {
							Pricing: map[string]types.SDLPricing{
								"web": {Denom: "uakt", Amount: 100},
							},
						},
					},
				},
				Deployment: map[string]types.SDLDeploymentGroup{
					"web": {Profile: "westcoast", Count: 1},
				},
			},
			wantErrors:  1,
			expectError: "port 70000 is out of valid range",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			errors := client.ValidateSDL(tc.sdl)

			if len(errors) != tc.wantErrors {
				t.Errorf("ValidateSDL() returned %d errors, want %d", len(errors), tc.wantErrors)
				for i, err := range errors {
					t.Errorf("Error %d: %s", i, err)
				}
			}

			if tc.expectError != "" {
				found := false
				for _, err := range errors {
					if containsString(err, tc.expectError) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error containing '%s', but got errors: %v", tc.expectError, errors)
				}
			}
		})
	}
}

func TestValidationHelpers(t *testing.T) {
	testCases := []struct {
		name     string
		function func(string) bool
		input    string
		expected bool
	}{
		{"valid service name", isValidServiceName, "web", true},
		{"valid service name with dash", isValidServiceName, "web-app", true},
		{"invalid service name with uppercase", isValidServiceName, "Web", false},
		{"invalid service name starting with dash", isValidServiceName, "-web", false},
		{"invalid service name ending with dash", isValidServiceName, "web-", false},
		
		{"valid CPU units integer", isValidCPUUnits, "1", true},
		{"valid CPU units fractional", isValidCPUUnits, "0.5", true},
		{"valid CPU units with milli", isValidCPUUnits, "100m", true},
		{"invalid CPU units", isValidCPUUnits, "invalid", false},
		
		{"valid memory size", isValidMemorySize, "512Mi", true},
		{"valid memory size with Gi", isValidMemorySize, "2Gi", true},
		{"invalid memory size", isValidMemorySize, "512", false},
		
		{"valid storage size", isValidStorageSize, "1Gi", true},
		{"valid storage size with Ti", isValidStorageSize, "1Ti", true},
		{"invalid storage size", isValidStorageSize, "1GB", false},
		
		{"valid denom uakt", isValidDenom, "uakt", true},
		{"valid denom akt", isValidDenom, "akt", true},
		{"valid denom IBC", isValidDenom, "ibc/170C677610AC31DF0904FFE09CD3B5C657492170E7E52372E48756B71E56F2F1", true},
		{"invalid denom", isValidDenom, "invalid", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.function(tc.input)
			if result != tc.expected {
				t.Errorf("%s(%s) = %v, want %v", tc.name, tc.input, result, tc.expected)
			}
		})
	}
}

func TestGenerateSDLHash(t *testing.T) {
	client := &AkashClient{}
	
	sdl := &types.SDL{
		Version: "2.0",
		Services: map[string]types.SDLService{
			"web": {Image: "nginx:1.21.6"},
		},
	}

	hash1, err := client.GenerateSDLHash(sdl)
	if err != nil {
		t.Fatalf("GenerateSDLHash() error = %v", err)
	}

	hash2, err := client.GenerateSDLHash(sdl)
	if err != nil {
		t.Fatalf("GenerateSDLHash() error = %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("GenerateSDLHash() should return consistent hash, got %s and %s", hash1, hash2)
	}

	if len(hash1) != 64 { // SHA256 hex string length
		t.Errorf("GenerateSDLHash() should return 64-character hex string, got %d characters", len(hash1))
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && s[0:len(substr)] == substr) ||
		(len(s) > len(substr) && s[len(s)-len(substr):] == substr) ||
		containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}