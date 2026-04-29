/*
Copyright 2024 The Akash Provider Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sdl

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/overlock-network/provider-akash/apis/resource/v1alpha1"
	clienttypes "github.com/overlock-network/provider-akash/internal/client/types"
)

func TestGenerateSDLHash(t *testing.T) {
	testCases := []struct {
		name     string
		sdl1     v1alpha1.SDLParameters
		sdl2     v1alpha1.SDLParameters
		wantSame bool
	}{
		{
			name: "identical SDLs should produce same hash",
			sdl1: v1alpha1.SDLParameters{
				Version: "2.0",
				Services: map[string]v1alpha1.SDLService{
					"web": {Image: "nginx:1.21.6"},
				},
				Profiles: v1alpha1.SDLProfiles{
					Compute: map[string]v1alpha1.SDLComputeProfile{
						"web": {
							Resources: v1alpha1.SDLResourceUnits{
								CPU:    "0.5",
								Memory: "512Mi",
							},
						},
					},
					Placement: map[string]v1alpha1.SDLPlacementProfile{
						"westcoast": {
							Pricing: map[string]v1alpha1.SDLPricing{
								"web": {Amount: 100},
							},
						},
					},
				},
				Deployment: map[string]v1alpha1.SDLDeploymentGroup{
					"web": {Profile: "westcoast", Count: 1},
				},
			},
			sdl2: v1alpha1.SDLParameters{
				Version: "2.0",
				Services: map[string]v1alpha1.SDLService{
					"web": {Image: "nginx:1.21.6"},
				},
				Profiles: v1alpha1.SDLProfiles{
					Compute: map[string]v1alpha1.SDLComputeProfile{
						"web": {
							Resources: v1alpha1.SDLResourceUnits{
								CPU:    "0.5",
								Memory: "512Mi",
							},
						},
					},
					Placement: map[string]v1alpha1.SDLPlacementProfile{
						"westcoast": {
							Pricing: map[string]v1alpha1.SDLPricing{
								"web": {Amount: 100},
							},
						},
					},
				},
				Deployment: map[string]v1alpha1.SDLDeploymentGroup{
					"web": {Profile: "westcoast", Count: 1},
				},
			},
			wantSame: true,
		},
		{
			name: "different SDLs should produce different hashes",
			sdl1: v1alpha1.SDLParameters{
				Version: "2.0",
				Services: map[string]v1alpha1.SDLService{
					"web": {Image: "nginx:1.21.6"},
				},
				Profiles: v1alpha1.SDLProfiles{
					Compute: map[string]v1alpha1.SDLComputeProfile{
						"web": {
							Resources: v1alpha1.SDLResourceUnits{
								CPU:    "0.5",
								Memory: "512Mi",
							},
						},
					},
					Placement: map[string]v1alpha1.SDLPlacementProfile{
						"westcoast": {
							Pricing: map[string]v1alpha1.SDLPricing{
								"web": {Amount: 100},
							},
						},
					},
				},
				Deployment: map[string]v1alpha1.SDLDeploymentGroup{
					"web": {Profile: "westcoast", Count: 1},
				},
			},
			sdl2: v1alpha1.SDLParameters{
				Version: "2.0",
				Services: map[string]v1alpha1.SDLService{
					"web": {Image: "nginx:1.22.0"}, // Different image
				},
				Profiles: v1alpha1.SDLProfiles{
					Compute: map[string]v1alpha1.SDLComputeProfile{
						"web": {
							Resources: v1alpha1.SDLResourceUnits{
								CPU:    "0.5",
								Memory: "512Mi",
							},
						},
					},
					Placement: map[string]v1alpha1.SDLPlacementProfile{
						"westcoast": {
							Pricing: map[string]v1alpha1.SDLPricing{
								"web": {Amount: 100},
							},
						},
					},
				},
				Deployment: map[string]v1alpha1.SDLDeploymentGroup{
					"web": {Profile: "westcoast", Count: 1},
				},
			},
			wantSame: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hash1, err := generateSDLHash(tc.sdl1)
			if err != nil {
				t.Fatalf("generateSDLHash() error = %v", err)
			}

			hash2, err := generateSDLHash(tc.sdl2)
			if err != nil {
				t.Fatalf("generateSDLHash() error = %v", err)
			}

			same := hash1 == hash2
			if same != tc.wantSame {
				t.Errorf("generateSDLHash() same = %v, wantSame = %v", same, tc.wantSame)
				t.Errorf("hash1 = %s", hash1)
				t.Errorf("hash2 = %s", hash2)
			}
		})
	}
}

func TestConvertToInternalSDL(t *testing.T) {
	testCases := []struct {
		name   string
		input  v1alpha1.SDLParameters
		expect *clienttypes.SDL
	}{
		{
			name: "basic conversion",
			input: v1alpha1.SDLParameters{
				Version: "2.0",
				Services: map[string]v1alpha1.SDLService{
					"web": {
						Image: "nginx:1.21.6",
						Expose: []v1alpha1.SDLServiceExpose{
							{
								Port: 80,
								To: []v1alpha1.SDLServiceExposeTo{
									{Global: true},
								},
							},
						},
					},
				},
				Profiles: v1alpha1.SDLProfiles{
					Compute: map[string]v1alpha1.SDLComputeProfile{
						"web": {
							Resources: v1alpha1.SDLResourceUnits{
								CPU:    "0.5",
								Memory: "512Mi",
							},
						},
					},
					Placement: map[string]v1alpha1.SDLPlacementProfile{
						"westcoast": {
							Pricing: map[string]v1alpha1.SDLPricing{
								"web": {Amount: 100},
							},
						},
					},
				},
				Deployment: map[string]v1alpha1.SDLDeploymentGroup{
					"web": {Profile: "westcoast", Count: 1},
				},
			},
			expect: &clienttypes.SDL{
				Version: "2.0",
				Services: map[string]clienttypes.SDLService{
					"web": {
						Image: "nginx:1.21.6",
						Expose: []clienttypes.SDLExposeSpec{
							{
								Port: 80,
								To: []clienttypes.SDLExposeTo{
									{Global: true},
								},
							},
						},
					},
				},
				Profiles: clienttypes.SDLProfiles{
					Compute: map[string]clienttypes.SDLComputeProfile{
						"web": {
							Resources: clienttypes.SDLResources{
								CPU:     clienttypes.SDLResourceCPU{Units: "0.5"},
								Memory:  clienttypes.SDLResourceMemory{Size: "512Mi"},
								Storage: clienttypes.SDLResourceStorage{Size: "1Gi"}, // Default
							},
						},
					},
					Placement: map[string]clienttypes.SDLPlacementProfile{
						"westcoast": {
							Pricing: map[string]clienttypes.SDLPricing{
								"web": {Amount: 100},
							},
						},
					},
				},
				Deployment: map[string]clienttypes.SDLDeploymentGroup{
					"web": {Profile: "westcoast", Count: 1},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := convertToInternalSDL(tc.input)

			if diff := cmp.Diff(tc.expect, result); diff != "" {
				t.Errorf("convertToInternalSDL() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetStorageSize(t *testing.T) {
	testCases := []struct {
		name     string
		storage  []v1alpha1.SDLStorage
		expected string
	}{
		{
			name:     "empty storage returns default",
			storage:  []v1alpha1.SDLStorage{},
			expected: "1Gi",
		},
		{
			name: "first storage size is returned",
			storage: []v1alpha1.SDLStorage{
				{Size: "5Gi"},
				{Size: "10Gi"},
			},
			expected: "5Gi",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := getStorageSize(tc.storage)
			if result != tc.expected {
				t.Errorf("getStorageSize() = %v, want %v", result, tc.expected)
			}
		})
	}
}