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

package certificate

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
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	akashv1alpha1 "github.com/overlock-network/provider-akash/apis/akash/v1alpha1"
	apisv1alpha1 "github.com/overlock-network/provider-akash/apis/v1alpha1"
	clientpkg "github.com/overlock-network/provider-akash/internal/client"
)

// Unlike many Kubernetes projects Crossplane does not use third party testing
// libraries, per the common Go test review comments. Crossplane encourages the
// use of table driven unit tests. The tests of the crossplane-runtime project
// are representative of the testing style Crossplane encourages.
//
// https://github.com/golang/go/wiki/TestComments
// https://github.com/crossplane/crossplane/blob/master/CONTRIBUTING.md#contributing-code

func TestObserve(t *testing.T) {
	type fields struct {
		service *CertificateService
	}

	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	type want struct {
		o   managed.ExternalObservation
		err error
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"NotCertificate": {
			reason: "Should return error when managed resource is not a Certificate",
			fields: fields{
				service: &CertificateService{},
			},
			args: args{
				ctx: context.Background(),
				mg:  &akashv1alpha1.ActiveBid{},
			},
			want: want{
				err: errors.New(errNotCertificate),
			},
		},
		"CertificateDoesNotExist": {
			reason: "Should return ResourceExists=false when certificate has no serial",
			fields: fields{
				service: &CertificateService{},
			},
			args: args{
				ctx: context.Background(),
				mg: &akashv1alpha1.Certificate{
					Spec: akashv1alpha1.CertificateSpec{
						ForProvider: akashv1alpha1.CertificateParameters{
							Domains: []string{"example.com"},
						},
						ResourceSpec: xpv1.ResourceSpec{
							ProviderConfigReference: &xpv1.Reference{
								Name: "test-config",
							},
						},
					},
					Status: akashv1alpha1.CertificateStatus{
						AtProvider: akashv1alpha1.CertificateObservation{
							Serial: "", // No serial means certificate doesn't exist
						},
					},
				},
			},
			want: want{
				o: managed.ExternalObservation{
					ResourceExists:   false,
					ResourceUpToDate: false,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create a fake Kubernetes client
			scheme := runtime.NewScheme()
			_ = akashv1alpha1.SchemeBuilder.AddToScheme(scheme)
			_ = apisv1alpha1.SchemeBuilder.AddToScheme(scheme)

			// Add a mock ProviderConfig
			pc := &apisv1alpha1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-config",
				},
				Spec: apisv1alpha1.ProviderConfigSpec{
					Configuration: &apisv1alpha1.AkashConfiguration{
						AccountAddress: stringPtr("akash1234567890abcdef"),
					},
				},
			}

			kubeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(pc).
				Build()

			e := external{
				service:    tc.fields.service,
				kubeClient: kubeClient,
			}
			got, err := e.Observe(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Observe(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.o, got); diff != "" {
				t.Errorf("\n%s\ne.Observe(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	type fields struct {
		service *CertificateService
	}

	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	type want struct {
		o   managed.ExternalCreation
		err error
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"NotCertificate": {
			reason: "Should return error when managed resource is not a Certificate",
			fields: fields{
				service: &CertificateService{},
			},
			args: args{
				ctx: context.Background(),
				mg:  &akashv1alpha1.ActiveBid{},
			},
			want: want{
				err: errors.New(errNotCertificate),
			},
		},
		"NoDomains": {
			reason: "Should return error when no domains are specified",
			fields: fields{
				service: &CertificateService{},
			},
			args: args{
				ctx: context.Background(),
				mg: &akashv1alpha1.Certificate{
					Spec: akashv1alpha1.CertificateSpec{
						ForProvider: akashv1alpha1.CertificateParameters{
							Domains: []string{}, // No domains
						},
						ResourceSpec: xpv1.ResourceSpec{
							ProviderConfigReference: &xpv1.Reference{
								Name: "test-config",
							},
						},
					},
				},
			},
			want: want{
				err: errors.New(errInvalidDomains),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create a fake Kubernetes client
			scheme := runtime.NewScheme()
			_ = akashv1alpha1.SchemeBuilder.AddToScheme(scheme)
			_ = apisv1alpha1.SchemeBuilder.AddToScheme(scheme)

			// Add a mock ProviderConfig
			pc := &apisv1alpha1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-config",
				},
				Spec: apisv1alpha1.ProviderConfigSpec{
					Configuration: &apisv1alpha1.AkashConfiguration{
						AccountAddress: stringPtr("akash1234567890abcdef"),
					},
				},
			}

			kubeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(pc).
				Build()

			e := external{
				service:    tc.fields.service,
				kubeClient: kubeClient,
			}
			got, err := e.Create(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Create(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.o, got); diff != "" {
				t.Errorf("\n%s\ne.Create(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	type fields struct {
		service *CertificateService
	}

	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	type want struct {
		o   managed.ExternalDelete
		err error
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"NotCertificate": {
			reason: "Should return error when managed resource is not a Certificate",
			fields: fields{
				service: &CertificateService{},
			},
			args: args{
				ctx: context.Background(),
				mg:  &akashv1alpha1.ActiveBid{},
			},
			want: want{
				err: errors.New(errNotCertificate),
			},
		},
		"NoSerial": {
			reason: "Should return success when certificate has no serial (nothing to delete)",
			fields: fields{
				service: &CertificateService{},
			},
			args: args{
				ctx: context.Background(),
				mg: &akashv1alpha1.Certificate{
					Spec: akashv1alpha1.CertificateSpec{
						ResourceSpec: xpv1.ResourceSpec{
							ProviderConfigReference: &xpv1.Reference{
								Name: "test-config",
							},
						},
					},
					Status: akashv1alpha1.CertificateStatus{
						AtProvider: akashv1alpha1.CertificateObservation{
							Serial: "", // No serial means nothing to delete
						},
					},
				},
			},
			want: want{
				o: managed.ExternalDelete{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create a fake Kubernetes client
			scheme := runtime.NewScheme()
			_ = akashv1alpha1.SchemeBuilder.AddToScheme(scheme)
			_ = apisv1alpha1.SchemeBuilder.AddToScheme(scheme)

			// Add a mock ProviderConfig
			pc := &apisv1alpha1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-config",
				},
				Spec: apisv1alpha1.ProviderConfigSpec{
					Configuration: &apisv1alpha1.AkashConfiguration{
						AccountAddress: stringPtr("akash1234567890abcdef"),
					},
				},
			}

			kubeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(pc).
				Build()

			e := external{
				service:    tc.fields.service,
				kubeClient: kubeClient,
			}
			got, err := e.Delete(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Delete(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.o, got); diff != "" {
				t.Errorf("\n%s\ne.Delete(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestConnectorConnect(t *testing.T) {
	// Test that connector properly validates Certificate resource type
	connector := &connector{}

	// Test with invalid resource type
	_, err := connector.Connect(context.Background(), &akashv1alpha1.ActiveBid{})
	if err == nil {
		t.Error("Connect() should fail with non-Certificate resource")
	}
	if !errors.Is(err, errors.New(errNotCertificate)) && err.Error() != errNotCertificate {
		t.Errorf("Connect() should return errNotCertificate, got: %v", err)
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

	mockCreateServiceFn := func(ctx context.Context, kubeClient client.Client, usage resource.Tracker, mg resource.Managed, pcInfo clientpkg.ProviderConfigInfo) (*CertificateService, error) {
		return &CertificateService{
			client:     nil, // Would be real client in production
			kubeClient: kubeClient,
		}, nil
	}

	certificate := &akashv1alpha1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-certificate",
		},
		Spec: akashv1alpha1.CertificateSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: "test-config",
				},
			},
		},
	}

	connector := &connector{
		kubeClient:                 kubeClient,
		usage:                      resource.NewProviderConfigUsageTracker(kubeClient, &apisv1alpha1.ProviderConfigUsage{}),
		createCertificateServiceFn: mockCreateServiceFn,
	}

	externalClient, err := connector.Connect(context.Background(), certificate)
	if err != nil {
		t.Errorf("Connect() failed: %v", err)
	}
	if externalClient == nil {
		t.Error("Connect() returned nil external client")
	}
}

// Helper function for creating string pointers
func stringPtr(s string) *string {
	return &s
}
