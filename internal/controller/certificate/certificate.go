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
	"fmt"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	kubeclient "sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	akashv1alpha1 "github.com/overlock-network/provider-akash/apis/akash/v1alpha1"
	apisv1alpha1 "github.com/overlock-network/provider-akash/apis/v1alpha1"
	client "github.com/overlock-network/provider-akash/internal/client"
	"github.com/overlock-network/provider-akash/internal/features"
)

const (
	errNotCertificate = "managed resource is not a Certificate custom resource"
	errTrackPCUsage   = "cannot track ProviderConfig usage"
	errGetPC          = "cannot get ProviderConfig"
	errGetCreds       = "cannot get credentials"
	errNewClient      = "cannot create new Service"

	// Certificate-specific errors
	errCreateCertificate = "failed to create certificate"
	errGetCertificate    = "failed to get certificate"
	errUpdateCertificate = "failed to update certificate"
	errRevokeCertificate = "failed to revoke certificate"
	errInvalidDomains    = "invalid domains specified"
	errCertificateExpiry = "certificate is expired or near expiry"

	// Requeue timing
	statusCheckInterval  = 60 * time.Second // Check certificate status every 60s
	renewalCheckInterval = 24 * time.Hour   // Check for renewal daily
)

// Certificate states
var (
	stateValid   = "valid"
	stateExpired = "expired"
	stateRevoked = "revoked"
	statePending = "pending"
)

type CertificateService struct {
	client     *client.AkashClient
	kubeClient kubeclient.Client
}

// CreateCertificate creates a new certificate on the Akash network
func (s *CertificateService) CreateCertificate(ctx context.Context, domains []string, owner string, validityDays int32) (*client.CertificateInfo, error) {
	if len(domains) == 0 {
		return nil, fmt.Errorf("at least one domain is required")
	}

	if owner == "" {
		return nil, fmt.Errorf("owner address is required")
	}

	// Create certificate using the client
	certInfo, err := s.client.CreateCertificate(ctx, domains, owner)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create certificate")
	}

	return certInfo, nil
}

// GetCertificate retrieves a certificate from the Akash network
func (s *CertificateService) GetCertificate(ctx context.Context, serial string, owner string) (*client.CertificateInfo, error) {
	certInfo, err := s.client.GetCertificate(ctx, serial, owner)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get certificate")
	}

	return certInfo, nil
}

// RevokeCertificate revokes a certificate on the Akash network
func (s *CertificateService) RevokeCertificate(ctx context.Context, serial string, owner string) error {
	err := s.client.RevokeCertificate(ctx, serial, owner)
	if err != nil {
		return errors.Wrap(err, "failed to revoke certificate")
	}

	return nil
}

// ValidateForRenewal checks if a certificate needs renewal
func (s *CertificateService) ValidateForRenewal(ctx context.Context, certInfo *client.CertificateInfo, autoRenew bool, validityDays int32) (bool, error) {
	needsRenewal, err := s.client.ValidateCertificate(certInfo, autoRenew, validityDays)
	if err != nil {
		return false, errors.Wrap(err, "failed to validate certificate")
	}

	return needsRenewal, nil
}

// newCertificateService creates CertificateService with AkashClient and Kubernetes client
var newCertificateService = func(ctx context.Context, kubeClient kubeclient.Client, usage resource.Tracker, mg resource.Managed, pcInfo client.ProviderConfigInfo) (*CertificateService, error) {
	c, err := client.NewFromManagedResource(ctx, kubeClient, usage, mg, pcInfo)
	if err != nil {
		return nil, err
	}
	return &CertificateService{client: c, kubeClient: kubeClient}, nil
}

// Setup adds a controller that reconciles Certificate managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(akashv1alpha1.CertificateGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(akashv1alpha1.CertificateGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kubeClient:                mgr.GetClient(),
			usage:                     resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			createCertificateServiceFn: newCertificateService}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&akashv1alpha1.Certificate{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method is called.
type connector struct {
	kubeClient                kubeclient.Client
	usage                     resource.Tracker
	createCertificateServiceFn func(ctx context.Context, kubeClient kubeclient.Client, usage resource.Tracker, mg resource.Managed, pcInfo client.ProviderConfigInfo) (*CertificateService, error)
}

// Connect produces an ExternalClient with ready-to-use AkashClient
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*akashv1alpha1.Certificate)
	if !ok {
		return nil, errors.New(errNotCertificate)
	}
	fmt.Printf("Connect Certificate: %s\n", cr.Name)

	pc := &apisv1alpha1.ProviderConfig{}
	if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: cr.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	pcInfo := client.ProviderConfigInfo{
		Source:              pc.Spec.Credentials.Source,
		CredentialSelectors: pc.Spec.Credentials.CommonCredentialSelectors,
		Configuration:       pc.Spec.Configuration,
	}

	if pc.Spec.Passphrase != nil {
		pcInfo.PassphraseSource = &pc.Spec.Passphrase.Source
		pcInfo.PassphraseSelectors = &pc.Spec.Passphrase.CommonCredentialSelectors
	}

	svc, err := c.createCertificateServiceFn(ctx, c.kubeClient, c.usage, mg, pcInfo)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{service: svc, kubeClient: c.kubeClient}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an external resource
type external struct {
	service    *CertificateService
	kubeClient kubeclient.Client
}

// Observe monitors the certificate status on the Akash network
func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*akashv1alpha1.Certificate)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotCertificate)
	}

	// The chain-side identity is the cert serial; we stash it in the
	// external-name annotation (same pattern Deployment uses for DSEQ)
	// because the managed reconciler persists annotations across reconciles
	// but not cr.Status.AtProvider mutations made inside Create.
	serial := cr.GetAnnotations()["crossplane.io/external-name"]
	if serial == cr.Name {
		serial = ""
	}
	fmt.Printf("Observing Certificate: %s (serial=%q)\n", cr.Name, serial)

	// Get owner address from ProviderConfig
	owner, err := c.resolveOwnerAddress(ctx, cr)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, "failed to resolve owner address")
	}

	// If no serial is available yet, the certificate doesn't exist on chain
	if serial == "" {
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	// Query certificate status from Akash network
	certInfo, err := c.service.GetCertificate(ctx, serial, owner)
	if err != nil {
		// If certificate not found, it doesn't exist yet
		if isCertificateNotFoundError(err) {
			return managed.ExternalObservation{
				ResourceExists: false,
			}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errGetCertificate)
	}

	// Update status with observed data
	cr.Status.AtProvider.Serial = certInfo.Serial
	cr.Status.AtProvider.Owner = certInfo.Owner
	cr.Status.AtProvider.Issuer = certInfo.Issuer
	cr.Status.AtProvider.Subject = certInfo.Subject
	cr.Status.AtProvider.NotBefore = certInfo.NotBefore
	cr.Status.AtProvider.NotAfter = certInfo.NotAfter
	cr.Status.AtProvider.State = certInfo.State
	cr.Status.AtProvider.Fingerprint = certInfo.Fingerprint
	cr.Status.AtProvider.PEM = certInfo.PEM
	cr.Status.AtProvider.CreatedAt = certInfo.CreatedAt
	cr.Status.AtProvider.ExpiresAt = certInfo.ExpiresAt

	// Check if certificate needs renewal
	autoRenew := getBoolValue(cr.Spec.ForProvider.AutoRenew, true)
	validityDays := getInt32Value(cr.Spec.ForProvider.ValidityDays, 365)

	needsRenewal, err := c.service.ValidateForRenewal(ctx, certInfo, autoRenew, validityDays)
	if err != nil {
		// Log but don't fail observation for renewal check errors
		cr.Status.AtProvider.LastRenewed = 0
	}

	// Set conditions based on state
	switch certInfo.State {
	case stateValid:
		if needsRenewal && autoRenew {
			cr.SetConditions(xpv1.Creating().WithMessage("Certificate renewal in progress"))
		} else {
			cr.SetConditions(xpv1.Available())
		}
	case stateExpired:
		cr.SetConditions(xpv1.Unavailable().WithMessage("Certificate is expired"))
	case stateRevoked:
		cr.SetConditions(xpv1.Unavailable().WithMessage("Certificate is revoked"))
	default:
		cr.SetConditions(xpv1.Creating())
	}

	// Check if resource needs update (renewal)
	resourceUpToDate := !needsRenewal || !autoRenew

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: resourceUpToDate,
	}, nil
}

// Create creates a new certificate on the Akash network
func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*akashv1alpha1.Certificate)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotCertificate)
	}

	fmt.Printf("Creating Certificate: %s\n", cr.Name)

	// Validate domains
	if len(cr.Spec.ForProvider.Domains) == 0 {
		return managed.ExternalCreation{}, errors.New(errInvalidDomains)
	}

	// Get owner address from ProviderConfig
	owner, err := c.resolveOwnerAddress(ctx, cr)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "failed to resolve owner address")
	}

	// Get validity days
	validityDays := getInt32Value(cr.Spec.ForProvider.ValidityDays, 365)

	// Create certificate
	certInfo, err := c.service.CreateCertificate(ctx, cr.Spec.ForProvider.Domains, owner, validityDays)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateCertificate)
	}
	fmt.Printf("Certificate broadcast OK: serial=%s state=%s\n", certInfo.Serial, certInfo.State)

	// Persist the chain-side identity (cert serial) on the CR via the
	// external-name annotation so subsequent Observe runs can query the
	// chain even after the in-memory status is dropped.
	annotations := cr.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations["crossplane.io/external-name"] = certInfo.Serial
	cr.SetAnnotations(annotations)

	// Update status with creation result
	cr.Status.AtProvider.Serial = certInfo.Serial
	cr.Status.AtProvider.Owner = certInfo.Owner
	cr.Status.AtProvider.Issuer = certInfo.Issuer
	cr.Status.AtProvider.Subject = certInfo.Subject
	cr.Status.AtProvider.NotBefore = certInfo.NotBefore
	cr.Status.AtProvider.NotAfter = certInfo.NotAfter
	cr.Status.AtProvider.State = certInfo.State
	cr.Status.AtProvider.Fingerprint = certInfo.Fingerprint
	cr.Status.AtProvider.PEM = certInfo.PEM
	cr.Status.AtProvider.CreatedAt = certInfo.CreatedAt
	cr.Status.AtProvider.ExpiresAt = certInfo.ExpiresAt

	// Set conditions
	if certInfo.State == stateValid {
		cr.SetConditions(xpv1.Available().WithMessage("Certificate created successfully"))
	} else {
		cr.SetConditions(xpv1.Creating().WithMessage("Certificate creation in progress"))
	}

	// Surface the cert + key PEMs as connection details so Crossplane's
	// NewAPISecretPublisher writes them into the Secret referenced by
	// spec.writeConnectionSecretToRef. Downstream resources (Manifest mTLS,
	// provider lease-status calls) consume tls.crt + tls.key from that
	// Secret. Standard kubernetes.io/tls keys are used so the Secret can be
	// fed straight into a TLS-enabled HTTP client.
	return managed.ExternalCreation{
		ConnectionDetails: managed.ConnectionDetails{
			"tls.crt": []byte(certInfo.PEM),
			"tls.key": []byte(certInfo.PrivateKeyPEM),
			"tls.pub": []byte(certInfo.PubkeyPEM),
			"serial":  []byte(certInfo.Serial),
			"owner":   []byte(certInfo.Owner),
		},
	}, nil
}

// Update handles certificate renewal
func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*akashv1alpha1.Certificate)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotCertificate)
	}

	// Check if auto-renew is enabled
	autoRenew := getBoolValue(cr.Spec.ForProvider.AutoRenew, true)
	if !autoRenew {
		return managed.ExternalUpdate{}, nil // No renewal needed
	}

	// Get owner address from ProviderConfig
	owner, err := c.resolveOwnerAddress(ctx, cr)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "failed to resolve owner address")
	}

	// Create new certificate (renewal)
	validityDays := getInt32Value(cr.Spec.ForProvider.ValidityDays, 365)
	certInfo, err := c.service.CreateCertificate(ctx, cr.Spec.ForProvider.Domains, owner, validityDays)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errCreateCertificate)
	}

	// Update status with new certificate
	cr.Status.AtProvider.Serial = certInfo.Serial
	cr.Status.AtProvider.Owner = certInfo.Owner
	cr.Status.AtProvider.Issuer = certInfo.Issuer
	cr.Status.AtProvider.Subject = certInfo.Subject
	cr.Status.AtProvider.NotBefore = certInfo.NotBefore
	cr.Status.AtProvider.NotAfter = certInfo.NotAfter
	cr.Status.AtProvider.State = certInfo.State
	cr.Status.AtProvider.Fingerprint = certInfo.Fingerprint
	cr.Status.AtProvider.PEM = certInfo.PEM
	cr.Status.AtProvider.LastRenewed = time.Now().Unix()
	cr.Status.AtProvider.ExpiresAt = certInfo.ExpiresAt

	// Renewal generated a new keypair — refresh the connection-details
	// Secret so consumers always pick up the latest tls.key.
	return managed.ExternalUpdate{
		ConnectionDetails: managed.ConnectionDetails{
			"tls.crt": []byte(certInfo.PEM),
			"tls.key": []byte(certInfo.PrivateKeyPEM),
			"tls.pub": []byte(certInfo.PubkeyPEM),
			"serial":  []byte(certInfo.Serial),
			"owner":   []byte(certInfo.Owner),
		},
	}, nil
}

// Delete revokes the certificate on the Akash network
func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*akashv1alpha1.Certificate)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotCertificate)
	}

	serial := cr.GetAnnotations()["crossplane.io/external-name"]
	if serial == "" || serial == cr.Name {
		return managed.ExternalDelete{}, nil
	}

	owner, err := c.resolveOwnerAddress(ctx, cr)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "failed to resolve owner address")
	}

	if err := c.service.RevokeCertificate(ctx, serial, owner); err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errRevokeCertificate)
	}

	return managed.ExternalDelete{}, nil
}

// Disconnect is called when the external client is no longer needed
func (c *external) Disconnect(ctx context.Context) error {
	// No cleanup needed for our client
	return nil
}

// Helper functions

// resolveOwnerAddress resolves the owner address from the ProviderConfig
func (c *external) resolveOwnerAddress(ctx context.Context, cr *akashv1alpha1.Certificate) (string, error) {
	// Get the ProviderConfig
	pc := &apisv1alpha1.ProviderConfig{}
	if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: cr.GetProviderConfigReference().Name}, pc); err != nil {
		return "", errors.Wrap(err, "failed to get ProviderConfig")
	}

	// Return the account address from the configuration
	if pc.Spec.Configuration != nil && pc.Spec.Configuration.AccountAddress != nil {
		return *pc.Spec.Configuration.AccountAddress, nil
	}

	return "", fmt.Errorf("no account address configured in ProviderConfig")
}

// resolveDeploymentReference resolves an optional deployment reference
func (c *external) resolveDeploymentReference(ctx context.Context, deploymentRef *akashv1alpha1.CertificateDeploymentReference) error {
	if deploymentRef == nil {
		return nil // No reference to resolve
	}

	// In a real implementation, you would validate that the deployment exists
	// For now, we'll just return nil
	return nil
}

// isCertificateNotFoundError checks if an error indicates certificate not found
func isCertificateNotFoundError(err error) bool {
	return err != nil && (
		// Add conditions for certificate not found errors
		false)
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

// Helper function to get int32 value with default fallback
func getInt32Value(ptr *int32, defaultValue int32) int32 {
	if ptr != nil {
		return *ptr
	}
	return defaultValue
}