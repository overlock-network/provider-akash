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

package manifest

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	akashv1alpha1 "github.com/overlock-network/provider-akash/apis/akash/v1alpha1"
	apisv1alpha1 "github.com/overlock-network/provider-akash/apis/v1alpha1"
	clientpkg "github.com/overlock-network/provider-akash/internal/client"
)

func TestManifestService_SendManifestToProvider(t *testing.T) {
	testCases := []struct {
		name      string
		leaseInfo clientpkg.LeaseInfo
		sdl       string
		wantError bool
	}{
		{
			name: "valid inputs",
			leaseInfo: clientpkg.LeaseInfo{
				Owner:    "akash1owner123",
				Dseq:     "12345",
				Gseq:     "1",
				Oseq:     "1",
				Provider: "akash1provider456",
			},
			sdl: `---
version: "2.0"
services:
  web:
    image: nginx:latest
    expose:
      - port: 80
        as: 80
        to:
          - global: true`,
			wantError: true, // Will fail at CLI call since no real client
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service := &ManifestService{
				client: &clientpkg.AkashClient{},
			}

			_, err := service.SendManifestToProvider(context.Background(), tc.leaseInfo, tc.sdl, nil, nil)
			if (err != nil) != tc.wantError {
				t.Errorf("SendManifestToProvider() error = %v, wantError %v", err, tc.wantError)
			}
		})
	}
}

func TestManifestService_SendManifestToProvider_InvalidInputs(t *testing.T) {
	tests := []struct {
		name      string
		leaseInfo clientpkg.LeaseInfo
		sdl       string
		wantError bool
	}{
		{
			name: "empty owner",
			leaseInfo: clientpkg.LeaseInfo{
				Owner:    "",
				Dseq:     "12345",
				Provider: "akash1provider456",
			},
			sdl:       "valid sdl",
			wantError: true,
		},
		{
			name: "empty SDL",
			leaseInfo: clientpkg.LeaseInfo{
				Owner:    "akash1owner123",
				Dseq:     "12345",
				Provider: "akash1provider456",
			},
			sdl:       "",
			wantError: true,
		},
		{
			name: "empty provider",
			leaseInfo: clientpkg.LeaseInfo{
				Owner:    "akash1owner123",
				Dseq:     "12345",
				Provider: "",
			},
			sdl:       "valid sdl",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &ManifestService{
				client: &clientpkg.AkashClient{},
			}
			_, err := service.SendManifestToProvider(context.Background(), tt.leaseInfo, tt.sdl, nil, nil)
			if (err != nil) != tt.wantError {
				t.Errorf("SendManifestToProvider() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestConnectorConnect(t *testing.T) {
	// Test that connector properly validates Manifest resource type
	connector := &connector{}

	// Test with invalid resource type
	_, err := connector.Connect(context.Background(), &akashv1alpha1.ActiveBid{})
	if err == nil {
		t.Error("Connect() should fail with non-Manifest resource")
	}
	if !errors.Is(err, errors.New(errNotManifest)) && err.Error() != errNotManifest {
		t.Errorf("Connect() should return errNotManifest, got: %v", err)
	}
}

func TestConnectorConnectSuccess(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = akashv1alpha1.SchemeBuilder.AddToScheme(scheme)
	_ = apisv1alpha1.SchemeBuilder.AddToScheme(scheme)

	pc := &apisv1alpha1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-config",
		},
		Spec: apisv1alpha1.ProviderConfigSpec{
			Credentials: apisv1alpha1.ProviderCredentials{
				Source: xpv1.CredentialsSourceSecret,
				CommonCredentialSelectors: xpv1.CommonCredentialSelectors{
					SecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "test-secret",
							Namespace: "test-namespace",
						},
						Key: "credentials",
					},
				},
			},
		},
	}

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(pc).
		Build()

	mockCreateServiceFn := func(ctx context.Context, kubeClient client.Client, usage resource.Tracker, mg resource.Managed, pcInfo clientpkg.ProviderConfigInfo) (*ManifestService, error) {
		return &ManifestService{
			client:     nil, // Would be real client in production
			kubeClient: kubeClient,
		}, nil
	}

	manifest := &akashv1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-manifest",
		},
		Spec: akashv1alpha1.ManifestSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: "test-config",
				},
			},
		},
	}

	connector := &connector{
		kubeClient:               kubeClient,
		usage:                    resource.NewProviderConfigUsageTracker(kubeClient, &apisv1alpha1.ProviderConfigUsage{}),
		createManifestServiceFn: mockCreateServiceFn,
	}

	external, err := connector.Connect(context.Background(), manifest)
	if err != nil {
		t.Errorf("Connect() unexpected error: %v", err)
	}
	if external == nil {
		t.Error("Connect() should return external client")
	}
}

func TestExternalValidation(t *testing.T) {
	// Test that external methods properly validate resource type
	scheme := runtime.NewScheme()
	_ = akashv1alpha1.SchemeBuilder.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	external := &external{
		service:    &ManifestService{},
		kubeClient: fakeClient,
	}

	tests := []struct {
		name string
		fn   func() error
	}{
		{
			name: "Observe",
			fn: func() error {
				_, err := external.Observe(context.Background(), &akashv1alpha1.ActiveBid{})
				return err
			},
		},
		{
			name: "Create",
			fn: func() error {
				_, err := external.Create(context.Background(), &akashv1alpha1.ActiveBid{})
				return err
			},
		},
		{
			name: "Update",
			fn: func() error {
				_, err := external.Update(context.Background(), &akashv1alpha1.ActiveBid{})
				return err
			},
		},
		{
			name: "Delete",
			fn: func() error {
				_, err := external.Delete(context.Background(), &akashv1alpha1.ActiveBid{})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if err == nil {
				t.Errorf("%s() should fail with non-Manifest resource", tt.name)
				return
			}
			if !errors.Is(err, errors.New(errNotManifest)) && err.Error() != errNotManifest {
				t.Errorf("%s() should return errNotManifest, got: %v", tt.name, err)
			}
		})
	}
}

func TestStates(t *testing.T) {
	// Test that our state constants are properly defined
	states := []string{statePending, stateDeployed, stateFailed, stateUpdating}

	expectedStates := []string{"pending", "deployed", "failed", "updating"}

	for i, state := range states {
		if state != expectedStates[i] {
			t.Errorf("Expected state %s, got %s", expectedStates[i], state)
		}
	}
}

func TestConvertToManifestServices(t *testing.T) {
	testCases := []struct {
		name     string
		input    []clientpkg.ManifestServiceInfo
		expected []akashv1alpha1.ManifestService
	}{
		{
			name: "multiple services",
			input: []clientpkg.ManifestServiceInfo{
				{
					Name:      "web",
					Image:     "nginx:latest",
					Available: true,
				},
				{
					Name:      "api",
					Image:     "myapp:v1.0",
					Available: false,
				},
			},
			expected: []akashv1alpha1.ManifestService{
				{
					Name:  "web",
					Image: "nginx:latest",
				},
				{
					Name:  "api",
					Image: "myapp:v1.0",
				},
			},
		},
		{
			name:     "empty services",
			input:    []clientpkg.ManifestServiceInfo{},
			expected: []akashv1alpha1.ManifestService{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := convertToManifestServices(tc.input)

			if len(result) != len(tc.expected) {
				t.Errorf("Expected %d services, got %d", len(tc.expected), len(result))
				return
			}

			for i, expected := range tc.expected {
				if result[i].Name != expected.Name {
					t.Errorf("Expected service[%d] name '%s', got '%s'", i, expected.Name, result[i].Name)
				}
				if result[i].Image != expected.Image {
					t.Errorf("Expected service[%d] image '%s', got '%s'", i, expected.Image, result[i].Image)
				}
			}
		})
	}
}

func TestConvertToManifestValidationErrors(t *testing.T) {
	testCases := []struct {
		name     string
		input    []clientpkg.ManifestError
		expected []akashv1alpha1.ManifestValidationError
	}{
		{
			name: "single error",
			input: []clientpkg.ManifestError{
				{
					Field:   "services.image",
					Message: "Invalid image format",
					Code:    "INVALID_IMAGE",
				},
			},
			expected: []akashv1alpha1.ManifestValidationError{
				{
					Field:   "services.image",
					Message: "Invalid image format",
					Code:    "INVALID_IMAGE",
				},
			},
		},
		{
			name:     "empty errors",
			input:    []clientpkg.ManifestError{},
			expected: []akashv1alpha1.ManifestValidationError{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := convertToManifestValidationErrors(tc.input)

			if len(result) != len(tc.expected) {
				t.Errorf("Expected %d errors, got %d", len(tc.expected), len(result))
				return
			}

			for i, expected := range tc.expected {
				if result[i].Field != expected.Field {
					t.Errorf("Expected error[%d] field '%s', got '%s'", i, expected.Field, result[i].Field)
				}
				if result[i].Message != expected.Message {
					t.Errorf("Expected error[%d] message '%s', got '%s'", i, expected.Message, result[i].Message)
				}
				if result[i].Code != expected.Code {
					t.Errorf("Expected error[%d] code '%s', got '%s'", i, expected.Code, result[i].Code)
				}
			}
		})
	}
}

func TestIsManifestNotFoundError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "generic error",
			err:      context.DeadlineExceeded,
			expected: false,
		},
		{
			name:     "context canceled",
			err:      context.Canceled,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isManifestNotFoundError(tc.err)
			if result != tc.expected {
				t.Errorf("isManifestNotFoundError() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

func TestConstants(t *testing.T) {
	// Test that important constants are defined
	constants := map[string]string{
		"errNotManifest":     errNotManifest,
		"errTrackPCUsage":    errTrackPCUsage,
		"errGetPC":           errGetPC,
		"errGetCreds":        errGetCreds,
		"errNewClient":       errNewClient,
		"statePending":       statePending,
		"stateDeployed":      stateDeployed,
		"stateFailed":        stateFailed,
		"stateUpdating":      stateUpdating,
	}

	for name, value := range constants {
		if value == "" {
			t.Errorf("Constant %s should not be empty", name)
		}
	}
}