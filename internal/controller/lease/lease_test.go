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

package lease

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	akashv1alpha1 "github.com/overlock-network/provider-akash/apis/akash/v1alpha1"
	resourcev1alpha1 "github.com/overlock-network/provider-akash/apis/resource/v1alpha1"
	apisv1alpha1 "github.com/overlock-network/provider-akash/apis/v1alpha1"
	akashclient "github.com/overlock-network/provider-akash/internal/client"
)

func TestParseLeaseId(t *testing.T) {
	testCases := []struct {
		name     string
		leaseId  string
		wantOwner string
		wantDseq  string
		wantGseq  string
		wantOseq  string
		wantProvider string
		wantErr  bool
	}{
		{
			name:     "valid lease ID",
			leaseId:  "akash1owner-12345-1-1-akash1provider",
			wantOwner: "akash1owner",
			wantDseq:  "12345",
			wantGseq:  "1",
			wantOseq:  "1",
			wantProvider: "akash1provider",
			wantErr:  false,
		},
		{
			name:     "invalid lease ID - too few parts",
			leaseId:  "akash1owner-12345-1",
			wantErr:  true,
		},
		{
			name:     "invalid lease ID - too many parts",
			leaseId:  "akash1owner-12345-1-1-akash1provider-extra",
			wantErr:  true,
		},
		{
			name:     "empty lease ID",
			leaseId:  "",
			wantErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			owner, dseq, gseq, oseq, provider, err := parseLeaseId(tc.leaseId)
			
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseLeaseId() expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("parseLeaseId() unexpected error: %v", err)
				return
			}
			
			if owner != tc.wantOwner {
				t.Errorf("parseLeaseId() owner = %v, want %v", owner, tc.wantOwner)
			}
			if dseq != tc.wantDseq {
				t.Errorf("parseLeaseId() dseq = %v, want %v", dseq, tc.wantDseq)
			}
			if gseq != tc.wantGseq {
				t.Errorf("parseLeaseId() gseq = %v, want %v", gseq, tc.wantGseq)
			}
			if oseq != tc.wantOseq {
				t.Errorf("parseLeaseId() oseq = %v, want %v", oseq, tc.wantOseq)
			}
			if provider != tc.wantProvider {
				t.Errorf("parseLeaseId() provider = %v, want %v", provider, tc.wantProvider)
			}
		})
	}
}

func TestLeaseServiceGetLease(t *testing.T) {
	testCases := []struct {
		name     string
		leaseId  string
		want     *akashv1alpha1.LeaseObservation
		wantErr  bool
	}{
		{
			name:    "valid lease ID",
			leaseId: "akash1owner-12345-1-1-akash1provider",
			want: &akashv1alpha1.LeaseObservation{
				LeaseId:  "akash1owner-12345-1-1-akash1provider",
				Owner:    "akash1owner",
				Dseq:     "12345",
				Gseq:     "1",
				Oseq:     "1",
				Provider: "akash1provider",
				State:    stateActive,
			},
			wantErr: false,
		},
		{
			name:    "invalid lease ID",
			leaseId: "invalid-lease-id",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service := &LeaseService{
				client: nil, // Nil client to test fallback behavior
			}
			
			result, err := service.GetLease(context.Background(), tc.leaseId)
			
			if tc.wantErr {
				if err == nil {
					t.Errorf("GetLease() expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("GetLease() unexpected error: %v", err)
				return
			}
			
			if diff := cmp.Diff(tc.want, result); diff != "" {
				t.Errorf("GetLease() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConnectorConnect(t *testing.T) {
	// Test that connector properly validates Lease resource type
	connector := &connector{}
	
	// Test with invalid resource type
	_, err := connector.Connect(context.Background(), &akashv1alpha1.ActiveBid{})
	if err == nil {
		t.Error("Connect() should fail with non-Lease resource")
	}
	if !errors.Is(err, errors.New(errNotLease)) && err.Error() != errNotLease {
		t.Errorf("Connect() should return errNotLease, got: %v", err)
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

	mockCreateServiceFn := func(ctx context.Context, kubeClient client.Client, usage resource.Tracker, mg resource.Managed, pcInfo akashclient.ProviderConfigInfo) (*LeaseService, error) {
		return &LeaseService{
			client:     nil, // Would be real client in production
			kubeClient: kubeClient,
		}, nil
	}

	lease := &akashv1alpha1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-lease",
		},
		Spec: akashv1alpha1.LeaseSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: "test-config",
				},
			},
		},
	}

	connector := &connector{
		kubeClient:            kubeClient,
		usage:                 resource.NewProviderConfigUsageTracker(kubeClient, &apisv1alpha1.ProviderConfigUsage{}),
		createLeaseServiceFn: mockCreateServiceFn,
	}

	external, err := connector.Connect(context.Background(), lease)
	if err != nil {
		t.Errorf("Connect() unexpected error: %v", err)
	}
	if external == nil {
		t.Error("Connect() should return external client")
	}
}

func TestExternalValidation(t *testing.T) {
	// Test that external methods properly validate resource type
	external := &external{}
	
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
				t.Errorf("%s() should fail with non-Lease resource", tt.name)
			}
			if !errors.Is(err, errors.New(errNotLease)) && err.Error() != errNotLease {
				t.Errorf("%s() should return errNotLease, got: %v", tt.name, err)
			}
		})
	}
}

func TestResolveReferences(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = akashv1alpha1.SchemeBuilder.AddToScheme(scheme)
	_ = resourcev1alpha1.SchemeBuilder.AddToScheme(scheme)

	deployment := &resourcev1alpha1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
		},
		Status: resourcev1alpha1.DeploymentStatus{
			AtProvider: resourcev1alpha1.DeploymentObservation{
				Owner:        "akash1owner",
				DeploymentId: "12345",
			},
		},
	}

	activeBid := &akashv1alpha1.ActiveBid{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-activebid",
			Namespace: "default",
		},
		Status: akashv1alpha1.ActiveBidStatus{
			AtProvider: akashv1alpha1.ActiveBidObservation{
				Gseq:     "1",
				Oseq:     "1",
				Provider: "akash1provider",
				Price: &akashv1alpha1.ActiveBidPriceStatus{
					Amount: "100",
					Denom:  "uakt",
				},
			},
		},
	}

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deployment, activeBid).
		Build()

	service := &LeaseService{
		client:     nil,
		kubeClient: kubeClient,
	}

	external := &external{service: service}

	lease := &akashv1alpha1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-lease",
			Namespace: "default",
		},
		Spec: akashv1alpha1.LeaseSpec{
			ForProvider: akashv1alpha1.LeaseParameters{
				DeploymentRef: akashv1alpha1.DeploymentReference{
					Name: "test-deployment",
				},
				ActiveBidRef: akashv1alpha1.ActiveBidReference{
					Name: "test-activebid",
				},
			},
		},
	}

	err := external.resolveReferences(context.Background(), lease)
	if err != nil {
		t.Errorf("resolveReferences() unexpected error: %v", err)
	}

	// Verify lease ID was generated
	expectedLeaseId := "akash1owner-12345-1-1-akash1provider"
	if lease.Status.AtProvider.LeaseId != expectedLeaseId {
		t.Errorf("resolveReferences() LeaseId = %v, want %v", lease.Status.AtProvider.LeaseId, expectedLeaseId)
	}

	// Verify other fields were populated
	if lease.Status.AtProvider.Owner != "akash1owner" {
		t.Errorf("resolveReferences() Owner = %v, want %v", lease.Status.AtProvider.Owner, "akash1owner")
	}
	if lease.Status.AtProvider.Dseq != "12345" {
		t.Errorf("resolveReferences() Dseq = %v, want %v", lease.Status.AtProvider.Dseq, "12345")
	}
	if lease.Status.AtProvider.Provider != "akash1provider" {
		t.Errorf("resolveReferences() Provider = %v, want %v", lease.Status.AtProvider.Provider, "akash1provider")
	}
}

func TestLeaseServiceMethods(t *testing.T) {
	// Test that LeaseService methods exist and have correct signatures
	// We test the methods that don't require client access
	service := &LeaseService{}
	
	// Test GetLease method signature with invalid lease ID  
	_, err := service.GetLease(context.Background(), "invalid-lease-id")
	if err == nil {
		t.Error("GetLease should fail with invalid lease ID")
	}
	
	// Test CloseLease method signature with valid lease ID
	err = service.CloseLease(context.Background(), "akash1owner-12345-1-1-akash1provider")
	// This should not fail as it just prints for now
	if err != nil {
		t.Errorf("CloseLease should not fail: %v", err)
	}
	
	// Test GetLeaseServices method signature with valid lease ID
	services, err := service.GetLeaseServices(context.Background(), "akash1owner-12345-1-1-akash1provider")
	if err != nil {
		t.Errorf("GetLeaseServices should not fail: %v", err)
	}
	if services == nil {
		t.Error("GetLeaseServices should return empty slice, not nil")
	}
}

func TestParseLeaseServicesFromJSON(t *testing.T) {
	testCases := []struct {
		name     string
		jsonData string
		want     []akashv1alpha1.LeaseService
		wantErr  bool
	}{
		{
			name:     "empty JSON",
			jsonData: "",
			want:     []akashv1alpha1.LeaseService{},
			wantErr:  false,
		},
		{
			name:     "empty object",
			jsonData: "{}",
			want:     []akashv1alpha1.LeaseService{},
			wantErr:  false,
		},
		{
			name: "single service",
			jsonData: `{
				"web": {
					"name": "web",
					"available": true,
					"uris": ["http://example.com:8080"]
				}
			}`,
			want: []akashv1alpha1.LeaseService{
				{
					Name:      "web",
					Available: true,
					URIs:      []string{"http://example.com:8080"},
					Ports: []akashv1alpha1.ServicePort{
						{
							Port:         8080,
							ExternalPort: 8080,
							Protocol:     "TCP",
							Host:         "example.com",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple services",
			jsonData: `{
				"web": {
					"name": "web",
					"available": true,
					"uris": ["http://web.example.com:80"]
				},
				"api": {
					"name": "api",
					"available": false,
					"uris": ["https://api.example.com:443"]
				}
			}`,
			want: []akashv1alpha1.LeaseService{
				{
					Name:      "web",
					Available: true,
					URIs:      []string{"http://web.example.com:80"},
					Ports: []akashv1alpha1.ServicePort{
						{
							Port:         80,
							ExternalPort: 80,
							Protocol:     "TCP",
							Host:         "web.example.com",
						},
					},
				},
				{
					Name:      "api",
					Available: false,
					URIs:      []string{"https://api.example.com:443"},
					Ports: []akashv1alpha1.ServicePort{
						{
							Port:         443,
							ExternalPort: 443,
							Protocol:     "TCP",
							Host:         "api.example.com",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "nested services response",
			jsonData: `{
				"services": {
					"web": {
						"name": "web",
						"available": true,
						"uris": ["http://example.com:3000"]
					}
				}
			}`,
			want: []akashv1alpha1.LeaseService{
				{
					Name:      "web",
					Available: true,
					URIs:      []string{"http://example.com:3000"},
					Ports: []akashv1alpha1.ServicePort{
						{
							Port:         3000,
							ExternalPort: 3000,
							Protocol:     "TCP",
							Host:         "example.com",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:     "invalid JSON",
			jsonData: "invalid json",
			want:     nil,
			wantErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseLeaseServicesFromJSON(tc.jsonData)

			if tc.wantErr {
				if err == nil {
					t.Errorf("parseLeaseServicesFromJSON() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("parseLeaseServicesFromJSON() unexpected error: %v", err)
				return
			}

			// Compare results (order-independent for multiple services)
			if len(result) != len(tc.want) {
				t.Errorf("parseLeaseServicesFromJSON() length = %v, want %v", len(result), len(tc.want))
				return
			}

			// For simplicity, just check that we got the expected services by name
			for _, wantService := range tc.want {
				found := false
				for _, gotService := range result {
					if gotService.Name == wantService.Name {
						found = true
						if gotService.Available != wantService.Available {
							t.Errorf("Service %s availability = %v, want %v", wantService.Name, gotService.Available, wantService.Available)
						}
						if len(gotService.URIs) != len(wantService.URIs) {
							t.Errorf("Service %s URIs length = %v, want %v", wantService.Name, len(gotService.URIs), len(wantService.URIs))
						}
						break
					}
				}
				if !found {
					t.Errorf("Expected service %s not found in result", wantService.Name)
				}
			}
		})
	}
}

func TestExtractPortsFromURIs(t *testing.T) {
	testCases := []struct {
		name string
		uris []string
		want []akashv1alpha1.ServicePort
	}{
		{
			name: "empty URIs",
			uris: []string{},
			want: []akashv1alpha1.ServicePort{},
		},
		{
			name: "HTTP URI",
			uris: []string{"http://example.com:8080"},
			want: []akashv1alpha1.ServicePort{
				{
					Port:         8080,
					ExternalPort: 8080,
					Protocol:     "TCP",
					Host:         "example.com",
				},
			},
		},
		{
			name: "HTTPS URI",
			uris: []string{"https://api.example.com:443/path"},
			want: []akashv1alpha1.ServicePort{
				{
					Port:         443,
					ExternalPort: 443,
					Protocol:     "TCP",
					Host:         "api.example.com",
				},
			},
		},
		{
			name: "multiple URIs",
			uris: []string{
				"http://web.example.com:80",
				"https://api.example.com:443",
			},
			want: []akashv1alpha1.ServicePort{
				{
					Port:         80,
					ExternalPort: 80,
					Protocol:     "TCP",
					Host:         "web.example.com",
				},
				{
					Port:         443,
					ExternalPort: 443,
					Protocol:     "TCP",
					Host:         "api.example.com",
				},
			},
		},
		{
			name: "URI without port",
			uris: []string{"http://example.com"},
			want: []akashv1alpha1.ServicePort{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractPortsFromURIs(tc.uris)

			if len(result) != len(tc.want) {
				t.Errorf("extractPortsFromURIs() length = %v, want %v", len(result), len(tc.want))
				return
			}

			for i, want := range tc.want {
				if i >= len(result) {
					break
				}
				got := result[i]
				if got.Port != want.Port {
					t.Errorf("extractPortsFromURIs()[%d].Port = %v, want %v", i, got.Port, want.Port)
				}
				if got.Host != want.Host {
					t.Errorf("extractPortsFromURIs()[%d].Host = %v, want %v", i, got.Host, want.Host)
				}
			}
		})
	}
}

func TestExtractHostFromURI(t *testing.T) {
	testCases := []struct {
		name string
		uri  string
		want string
	}{
		{
			name: "HTTP URI",
			uri:  "http://example.com:8080",
			want: "example.com",
		},
		{
			name: "HTTPS URI with path",
			uri:  "https://api.example.com:443/v1/health",
			want: "api.example.com",
		},
		{
			name: "URI without protocol",
			uri:  "example.com:8080",
			want: "example.com",
		},
		{
			name: "URI without port",
			uri:  "http://example.com/path",
			want: "example.com",
		},
		{
			name: "plain hostname",
			uri:  "example.com",
			want: "example.com",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractHostFromURI(tc.uri)
			if result != tc.want {
				t.Errorf("extractHostFromURI() = %v, want %v", result, tc.want)
			}
		})
	}
}

func TestConstants(t *testing.T) {
	// Test that important constants are defined
	constants := map[string]string{
		"errNotLease":     errNotLease,
		"errTrackPCUsage": errTrackPCUsage,
		"errGetPC":        errGetPC,
		"errGetCreds":     errGetCreds,
		"errNewClient":    errNewClient,
		"stateActive":     stateActive,
		"statePaused":     statePaused,
		"stateClosed":     stateClosed,
	}
	
	for name, value := range constants {
		if value == "" {
			t.Errorf("Constant %s should not be empty", name)
		}
	}
}