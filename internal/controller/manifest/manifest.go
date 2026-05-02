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
	"fmt"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
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
	resourcev1alpha1 "github.com/overlock-network/provider-akash/apis/resource/v1alpha1"
	apisv1alpha1 "github.com/overlock-network/provider-akash/apis/v1alpha1"
	client "github.com/overlock-network/provider-akash/internal/client"
	"github.com/overlock-network/provider-akash/internal/features"
)

const (
	errNotManifest  = "managed resource is not a Manifest custom resource"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"
	errNewClient    = "cannot create new Service"

	// Manifest-specific errors
	errGetLease          = "failed to get referenced Lease"
	errGetDeployment     = "failed to get referenced Deployment"
	errSendManifest      = "failed to send manifest to provider"
	errQueryManifest     = "failed to query manifest status"
	errUpdateManifest    = "failed to update manifest"
	errInvalidLease      = "referenced Lease is not ready or does not exist"
	errLeaseNotActive    = "referenced Lease is not in active state"
	errManifestNotFound  = "manifest not found on provider"

	// Requeue timing
	statusCheckInterval = 30 * time.Second // Check manifest status every 30s
)

// Manifest states
var (
	statePending  = "pending"
	stateDeployed = "deployed"
	stateFailed   = "failed"
	stateUpdating = "updating"
)

type ManifestService struct {
	client     *client.AkashClient
	kubeClient kubeclient.Client
}

// SendManifestToProvider sends the manifest to the provider over mTLS.
func (s *ManifestService) SendManifestToProvider(ctx context.Context, leaseInfo client.LeaseInfo, sdlContent string, certPEM, keyPEM []byte) (*client.ManifestStatus, error) {
	if leaseInfo.Owner == "" || leaseInfo.Dseq == "" || leaseInfo.Provider == "" {
		return nil, fmt.Errorf("invalid lease information provided")
	}
	if sdlContent == "" {
		return nil, fmt.Errorf("SDL content cannot be empty")
	}
	if validationErrors := s.client.ValidateManifestSDL(sdlContent); len(validationErrors) > 0 {
		return &client.ManifestStatus{
			State:            stateFailed,
			ValidationErrors: validationErrors,
		}, fmt.Errorf("SDL validation failed: %d errors", len(validationErrors))
	}

	status, err := s.client.SendManifestToProvider(ctx, leaseInfo, sdlContent, certPEM, keyPEM)
	if err != nil {
		return status, fmt.Errorf("failed to send manifest to provider: %w", err)
	}
	return status, nil
}

// GetManifestStatus retrieves the current status of the manifest from the provider
func (s *ManifestService) GetManifestStatus(ctx context.Context, leaseInfo client.LeaseInfo, certPEM, keyPEM []byte) (*client.ManifestStatus, error) {
	status, err := s.client.GetManifestStatus(ctx, leaseInfo, certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest status: %w", err)
	}
	return status, nil
}

// UpdateManifestOnProvider updates the manifest on the provider
func (s *ManifestService) UpdateManifestOnProvider(ctx context.Context, leaseInfo client.LeaseInfo, sdlContent string, certPEM, keyPEM []byte) (*client.ManifestStatus, error) {
	if validationErrors := s.client.ValidateManifestSDL(sdlContent); len(validationErrors) > 0 {
		return &client.ManifestStatus{
			State:            stateFailed,
			ValidationErrors: validationErrors,
		}, fmt.Errorf("SDL validation failed: %d errors", len(validationErrors))
	}
	status, err := s.client.UpdateManifest(ctx, leaseInfo, sdlContent, certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to update manifest: %w", err)
	}
	return status, nil
}

// newManifestService creates ManifestService with AkashClient and Kubernetes client
var newManifestService = func(ctx context.Context, kubeClient kubeclient.Client, usage resource.Tracker, mg resource.Managed, pcInfo client.ProviderConfigInfo) (*ManifestService, error) {
	c, err := client.NewFromManagedResource(ctx, kubeClient, usage, mg, pcInfo)
	if err != nil {
		return nil, err
	}
	return &ManifestService{client: c, kubeClient: kubeClient}, nil
}

// Setup adds a controller that reconciles Manifest managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(akashv1alpha1.ManifestGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	wedgeHealer := managed.InitializerFn(func(ctx context.Context, mg resource.Managed) error {
		cr, ok := mg.(*akashv1alpha1.Manifest)
		if !ok {
			return nil
		}
		if cr.Status.AtProvider.DeployedAt != 0 {
			return nil
		}
		anns := cr.GetAnnotations()
		if anns == nil {
			return nil
		}
		if _, wedged := anns["crossplane.io/external-create-pending"]; !wedged {
			return nil
		}
		delete(anns, "crossplane.io/external-create-pending")
		cr.SetAnnotations(anns)
		return nil
	})

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(akashv1alpha1.ManifestGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kubeClient:              mgr.GetClient(),
			usage:                   resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			createManifestServiceFn: newManifestService}),
		managed.WithInitializers(wedgeHealer),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&akashv1alpha1.Manifest{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method is called.
type connector struct {
	kubeClient              kubeclient.Client
	usage                   resource.Tracker
	createManifestServiceFn func(ctx context.Context, kubeClient kubeclient.Client, usage resource.Tracker, mg resource.Managed, pcInfo client.ProviderConfigInfo) (*ManifestService, error)
}

// Connect produces an ExternalClient with ready-to-use AkashClient
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*akashv1alpha1.Manifest)
	if !ok {
		return nil, errors.New(errNotManifest)
	}

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

	svc, err := c.createManifestServiceFn(ctx, c.kubeClient, c.usage, mg, pcInfo)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{service: svc, kubeClient: c.kubeClient}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an external resource
type external struct {
	service    *ManifestService
	kubeClient kubeclient.Client
}

// Observe reports whether the manifest has been delivered to the provider.
func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*akashv1alpha1.Manifest)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotManifest)
	}
	fmt.Printf("Observing Manifest: %s\n", cr.Name)

	if cr.GetDeletionTimestamp() != nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	leaseInfo, err := c.resolveLeaseReference(ctx, cr.Spec.ForProvider.LeaseRef)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetLease)
	}

	cr.Status.AtProvider.Owner = leaseInfo.Owner
	cr.Status.AtProvider.Dseq = leaseInfo.Dseq
	cr.Status.AtProvider.Gseq = leaseInfo.Gseq
	cr.Status.AtProvider.Oseq = leaseInfo.Oseq
	cr.Status.AtProvider.Provider = leaseInfo.Provider

	certPEM, keyPEM, err := c.loadCertSecret(ctx, cr)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, "wait for cert secret")
	}

	delivered := cr.GetAnnotations()["crossplane.io/external-create-succeeded"] != ""
	if !delivered {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	if cr.Status.AtProvider.DeployedAt == 0 {
		cr.Status.AtProvider.DeployedAt = time.Now().Unix()
	}

	status, err := c.service.GetManifestStatus(ctx, *leaseInfo, certPEM, keyPEM)
	if err != nil {
		cr.SetConditions(xpv1.Available().WithMessage("manifest delivered; live status unavailable: " + err.Error()))
		return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
	}

	cr.Status.AtProvider.State = status.State
	if status.Version != "" {
		cr.Status.AtProvider.ManifestVersion = status.Version
	}
	cr.Status.AtProvider.ProviderResponse = status.ProviderResponse
	cr.Status.AtProvider.Services = convertToManifestServices(status.Services)
	cr.Status.AtProvider.ValidationErrors = convertToManifestValidationErrors(status.ValidationErrors)

	switch status.State {
	case stateDeployed:
		cr.SetConditions(xpv1.Available())
	case stateFailed:
		cr.SetConditions(xpv1.Unavailable().WithMessage("Manifest deployment failed"))
	default:
		cr.SetConditions(xpv1.Available().WithMessage("manifest delivered; provider state: " + status.State))
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil
}

// Create deploys the manifest to the provider
func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*akashv1alpha1.Manifest)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotManifest)
	}
	leaseInfo, err := c.resolveLeaseReference(ctx, cr.Spec.ForProvider.LeaseRef)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errGetLease)
	}

	sdlContent, err := c.getSDLContentFromLease(ctx, *leaseInfo)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errGetDeployment)
	}

	certPEM, keyPEM, err := c.loadCertSecret(ctx, cr)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "load mTLS secret")
	}

	status, err := c.service.SendManifestToProvider(ctx, *leaseInfo, sdlContent, certPEM, keyPEM)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errSendManifest)
	}

	_ = status
	_ = sdlContent
	return managed.ExternalCreation{}, nil
}

// Update updates the manifest on the provider
func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*akashv1alpha1.Manifest)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotManifest)
	}

	// Resolve lease reference to get lease information
	leaseInfo, err := c.resolveLeaseReference(ctx, cr.Spec.ForProvider.LeaseRef)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errGetLease)
	}

	// Get SDL content from the deployment
	sdlContent, err := c.getSDLContentFromLease(ctx, *leaseInfo)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errGetDeployment)
	}

	certPEM, keyPEM, err := c.loadCertSecret(ctx, cr)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "load mTLS secret")
	}

	// Update manifest on provider
	status, err := c.service.UpdateManifestOnProvider(ctx, *leaseInfo, sdlContent, certPEM, keyPEM)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateManifest)
	}

	// Update status
	cr.Status.AtProvider.SdlContent = sdlContent
	cr.Status.AtProvider.State = status.State
	cr.Status.AtProvider.ManifestVersion = status.Version

	return managed.ExternalUpdate{}, nil
}

// Delete removes the manifest from the provider (typically handled by lease closure)
func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	_, ok := mg.(*akashv1alpha1.Manifest)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotManifest)
	}

	// For Akash, manifests are automatically removed when leases are closed
	// So we don't need to do anything here explicitly
	return managed.ExternalDelete{}, nil
}

// Disconnect is called when the external client is no longer needed
func (c *external) Disconnect(ctx context.Context) error {
	// No cleanup needed for our client
	return nil
}

// Helper functions

// loadCertSecret reads tls.crt and tls.key from the referenced Secret.
func (c *external) loadCertSecret(ctx context.Context, cr *akashv1alpha1.Manifest) ([]byte, []byte, error) {
	ref := cr.Spec.ForProvider.CertificateSecretRef
	if ref.Name == "" {
		return nil, nil, fmt.Errorf("certificateSecretRef.name is required")
	}
	ns := ref.Namespace
	if ns == "" {
		ns = cr.GetNamespace()
	}
	sec := &corev1.Secret{}
	if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ns}, sec); err != nil {
		return nil, nil, fmt.Errorf("get Secret %s/%s: %w", ns, ref.Name, err)
	}
	cert, ok := sec.Data["tls.crt"]
	if !ok || len(cert) == 0 {
		return nil, nil, fmt.Errorf("Secret %s/%s missing tls.crt", ns, ref.Name)
	}
	key, ok := sec.Data["tls.key"]
	if !ok || len(key) == 0 {
		return nil, nil, fmt.Errorf("Secret %s/%s missing tls.key", ns, ref.Name)
	}
	return cert, key, nil
}

// resolveLeaseReference resolves a lease reference to get lease information
func (c *external) resolveLeaseReference(ctx context.Context, leaseRef akashv1alpha1.ManifestLeaseReference) (*client.LeaseInfo, error) {
	// Check if kubeClient is available
	if c.kubeClient == nil {
		return nil, fmt.Errorf("kubeClient is not available")
	}

	// Get the lease
	lease := &akashv1alpha1.Lease{}
	namespacedName := types.NamespacedName{
		Name:      leaseRef.Name,
		Namespace: leaseRef.Namespace,
	}

	if err := c.kubeClient.Get(ctx, namespacedName, lease); err != nil {
		return nil, err
	}

	if !resource.IsConditionTrue(lease.GetCondition(xpv1.TypeReady)) {
		return nil, fmt.Errorf("Lease %s not ready yet", leaseRef.Name)
	}
	if lease.Status.AtProvider.LeaseId == "" {
		return nil, fmt.Errorf("Lease %s has no leaseId yet", leaseRef.Name)
	}

	return &client.LeaseInfo{
		Owner:    lease.Status.AtProvider.Owner,
		Dseq:     lease.Status.AtProvider.Dseq,
		Gseq:     lease.Status.AtProvider.Gseq,
		Oseq:     lease.Status.AtProvider.Oseq,
		Provider: lease.Status.AtProvider.Provider,
	}, nil
}

// getSDLContentFromLease gets SDL content from the deployment referenced by the lease
func (c *external) getSDLContentFromLease(ctx context.Context, leaseInfo client.LeaseInfo) (string, error) {
	lease, err := c.getLeaseByInfo(ctx, leaseInfo)
	if err != nil {
		return "", fmt.Errorf("failed to get lease: %w", err)
	}

	deployment, err := c.getReferencedDeployment(ctx, lease)
	if err != nil {
		return "", fmt.Errorf("failed to get referenced deployment: %w", err)
	}

	sdlContent, err := c.getSDLContentFromDeployment(ctx, deployment)
	if err != nil {
		return "", fmt.Errorf("failed to get SDL content: %w", err)
	}

	return sdlContent, nil
}

// getLeaseByInfo retrieves a lease resource by matching lease info
func (c *external) getLeaseByInfo(ctx context.Context, leaseInfo client.LeaseInfo) (*akashv1alpha1.Lease, error) {
	// List all leases and find the one matching our lease info
	leaseList := &akashv1alpha1.LeaseList{}
	if err := c.kubeClient.List(ctx, leaseList); err != nil {
		return nil, fmt.Errorf("failed to list leases: %w", err)
	}

	for _, lease := range leaseList.Items {
		// Match by the lease identifier fields
		if lease.Status.AtProvider.Owner == leaseInfo.Owner &&
			lease.Status.AtProvider.Dseq == leaseInfo.Dseq &&
			lease.Status.AtProvider.Gseq == leaseInfo.Gseq &&
			lease.Status.AtProvider.Oseq == leaseInfo.Oseq &&
			lease.Status.AtProvider.Provider == leaseInfo.Provider {
			return &lease, nil
		}
	}

	return nil, fmt.Errorf("lease not found for Owner:%s Dseq:%s Gseq:%s Oseq:%s Provider:%s",
		leaseInfo.Owner, leaseInfo.Dseq, leaseInfo.Gseq, leaseInfo.Oseq, leaseInfo.Provider)
}

// getReferencedDeployment gets the Deployment referenced by the lease
func (c *external) getReferencedDeployment(ctx context.Context, lease *akashv1alpha1.Lease) (*resourcev1alpha1.Deployment, error) {
	deploymentRef := lease.Spec.ForProvider.DeploymentRef

	namespace := deploymentRef.Namespace
	if namespace == nil {
		ns := lease.Namespace
		namespace = &ns
	}

	deployment := &resourcev1alpha1.Deployment{}
	err := c.kubeClient.Get(ctx, types.NamespacedName{
		Name:      deploymentRef.Name,
		Namespace: *namespace,
	}, deployment)
	if err != nil {
		return nil, fmt.Errorf("failed to get Deployment %s/%s: %w", *namespace, deploymentRef.Name, err)
	}

	return deployment, nil
}

// getSDLContentFromDeployment gets the SDL content from a deployment
func (c *external) getSDLContentFromDeployment(ctx context.Context, deployment *resourcev1alpha1.Deployment) (string, error) {
	sdlRef := deployment.Spec.ForProvider.SDLRef

	namespace := sdlRef.Namespace
	if namespace == "" {
		namespace = deployment.Namespace
	}

	sdl := &resourcev1alpha1.SDL{}
	if err := c.kubeClient.Get(ctx, types.NamespacedName{
		Name:      sdlRef.Name,
		Namespace: namespace,
	}, sdl); err != nil {
		return "", fmt.Errorf("failed to get SDL %s/%s: %w", namespace, sdlRef.Name, err)
	}

	sdlYAML, err := c.convertSDLToYAML(sdl)
	if err != nil {
		return "", fmt.Errorf("failed to convert SDL to YAML: %w", err)
	}

	return sdlYAML, nil
}

// isManifestNotFoundError checks if an error indicates manifest not found
func isManifestNotFoundError(err error) bool {
	return err != nil && errors.Is(err, client.ErrManifestNotFound)
}

// convertToManifestServices converts client manifest services to CRD format
func convertToManifestServices(services []client.ManifestServiceInfo) []akashv1alpha1.ManifestService {
	result := make([]akashv1alpha1.ManifestService, len(services))
	for i, svc := range services {
		result[i] = akashv1alpha1.ManifestService{
			Name:  svc.Name,
			Image: svc.Image,
		}
	}
	return result
}

// convertToManifestValidationErrors converts client errors to CRD format
func convertToManifestValidationErrors(errors []client.ManifestError) []akashv1alpha1.ManifestValidationError {
	result := make([]akashv1alpha1.ManifestValidationError, len(errors))
	for i, err := range errors {
		result[i] = akashv1alpha1.ManifestValidationError{
			Field:   err.Field,
			Message: err.Message,
			Code:    err.Code,
		}
	}
	return result
}

// convertSDLToYAML renders the SDL CR into canonical YAML.
func (c *external) convertSDLToYAML(sdl *resourcev1alpha1.SDL) (string, error) {
	return client.RenderSDLToYAML(sdl)
}

// convertSDLServices converts SDL services to a format suitable for YAML
func convertSDLServices(services map[string]resourcev1alpha1.SDLService) map[string]interface{} {
	result := make(map[string]interface{})

	for name, service := range services {
		svcMap := map[string]interface{}{
			"image": service.Image,
		}

		if len(service.Command) > 0 {
			svcMap["command"] = service.Command
		}

		if len(service.Args) > 0 {
			svcMap["args"] = service.Args
		}

		if len(service.Env) > 0 {
			svcMap["env"] = service.Env
		}

		if len(service.Expose) > 0 {
			exposes := make([]map[string]interface{}, len(service.Expose))
			for i, expose := range service.Expose {
				exposeMap := map[string]interface{}{
					"port": expose.Port,
				}

				if expose.As != nil {
					exposeMap["as"] = *expose.As
				}

				if expose.Proto != "" {
					exposeMap["proto"] = expose.Proto
				}

				if len(expose.To) > 0 {
					toList := make([]map[string]interface{}, len(expose.To))
					for j, to := range expose.To {
						toMap := make(map[string]interface{})
						if to.Service != "" {
							toMap["service"] = to.Service
						}
						if to.Global {
							toMap["global"] = to.Global
						}
						toList[j] = toMap
					}
					exposeMap["to"] = toList
				}

				if expose.Accept != nil && len(expose.Accept.Items) > 0 {
					exposeMap["accept"] = expose.Accept.Items
				}

				exposes[i] = exposeMap
			}
			svcMap["expose"] = exposes
		}

		result[name] = svcMap
	}

	return result
}

// convertSDLProfiles converts SDL profiles to a format suitable for YAML
func convertSDLProfiles(profiles resourcev1alpha1.SDLProfiles) map[string]interface{} {
	compute := convertSDLComputeProfiles(profiles.Compute)
	placement := convertSDLPlacementProfiles(profiles.Placement)
	result := map[string]interface{}{
		"compute":   compute,
		"placement": placement,
	}

	return result
}

// convertSDLComputeProfiles converts compute profiles
func convertSDLComputeProfiles(compute map[string]resourcev1alpha1.SDLComputeProfile) map[string]interface{} {
	result := make(map[string]interface{})

	for name, profile := range compute {
		resources := map[string]interface{}{
			"cpu":    profile.Resources.CPU,
			"memory": profile.Resources.Memory,
		}

		if len(profile.Resources.Storage) > 0 {
			storage := make([]map[string]interface{}, len(profile.Resources.Storage))
			for i, s := range profile.Resources.Storage {
				storageMap := map[string]interface{}{
					"size": s.Size,
				}
				if s.Name != "" {
					storageMap["name"] = s.Name
				}
				if s.Class != "" {
					storageMap["class"] = s.Class
				}
				storage[i] = storageMap
			}
			resources["storage"] = storage
		}

		if profile.Resources.GPU != nil {
			gpu := map[string]interface{}{
				"units": profile.Resources.GPU.Units,
			}
			if len(profile.Resources.GPU.Attributes) > 0 {
				gpu["attributes"] = profile.Resources.GPU.Attributes
			}
			resources["gpu"] = gpu
		}

		result[name] = map[string]interface{}{
			"resources": resources,
		}
	}

	return result
}

// convertSDLPlacementProfiles converts placement profiles
func convertSDLPlacementProfiles(placement map[string]resourcev1alpha1.SDLPlacementProfile) map[string]interface{} {
	result := make(map[string]interface{})

	for name, profile := range placement {
		profileMap := make(map[string]interface{})

		if len(profile.Attributes) > 0 {
			profileMap["attributes"] = profile.Attributes
		}

		if profile.SignedBy != nil && (len(profile.SignedBy.AnyOf) > 0 || len(profile.SignedBy.AllOf) > 0) {
			signedBy := make(map[string]interface{})
			if len(profile.SignedBy.AnyOf) > 0 {
				signedBy["anyOf"] = profile.SignedBy.AnyOf
			}
			if len(profile.SignedBy.AllOf) > 0 {
				signedBy["allOf"] = profile.SignedBy.AllOf
			}
			profileMap["signedBy"] = signedBy
		}

		if len(profile.Pricing) > 0 {
			pricing := make(map[string]interface{})
			for svcName, price := range profile.Pricing {
				pricing[svcName] = map[string]interface{}{
					"denom":  resourcev1alpha1.DepositDenom,
					"amount": price.Amount,
				}
			}
			profileMap["pricing"] = pricing
		}

		result[name] = profileMap
	}

	return result
}