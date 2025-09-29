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
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	"github.com/overlock-network/provider-akash/apis/resource/v1alpha1"
	apisv1alpha1 "github.com/overlock-network/provider-akash/apis/v1alpha1"
	client "github.com/overlock-network/provider-akash/internal/client"
	clienttypes "github.com/overlock-network/provider-akash/internal/client/types"
	"github.com/overlock-network/provider-akash/internal/features"
)

const (
	errNotSDL       = "managed resource is not an SDL custom resource"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"

	errNewClient = "cannot create new Service"
)

// Setup adds a controller that reconciles SDL managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.SDLGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.SDLGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.SDL{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube  kubeclient.Client
	usage resource.Tracker
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.SDL)
	if !ok {
		return nil, errors.New(errNotSDL)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	pc := &apisv1alpha1.ProviderConfig{}
	if err := c.kube.Get(ctx, types.NamespacedName{Name: cr.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	// Create AkashClient using NewFromManagedResource
	pcInfo := client.ProviderConfigInfo{
		Source:              pc.Spec.Credentials.Source,
		CredentialSelectors: pc.Spec.Credentials.CommonCredentialSelectors,
		PassphraseSource:    nil, // SDL validation doesn't need passphrase
		PassphraseSelectors: nil,
		Configuration:       pc.Spec.Configuration,
	}

	akashClient, err := client.NewFromManagedResource(ctx, c.kube, c.usage, mg, pcInfo)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{service: akashClient, kube: c.kube}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	service *client.AkashClient
	kube    kubeclient.Client
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.SDL)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotSDL)
	}

	// Generate hash of current SDL spec
	currentHash, err := generateSDLHash(cr.Spec.ForProvider)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot generate SDL hash")
	}

	// Check if SDL has changed by comparing hashes
	if cr.Status.AtProvider.Hash != currentHash {
		// SDL has changed, needs validation
		return managed.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: false,
		}, nil
	}

	// SDL hasn't changed and is already validated
	if cr.Status.AtProvider.Validated && len(cr.Status.AtProvider.ValidationErrors) == 0 {
		cr.SetConditions(xpv1.Available())
		return managed.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: true,
		}, nil
	}

	// SDL needs validation
	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: false,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.SDL)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotSDL)
	}

	return c.validateSDL(ctx, cr)
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.SDL)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotSDL)
	}

	_, err := c.validateSDL(ctx, cr)
	return managed.ExternalUpdate{}, err
}

// Disconnect is called when the ExternalClient is no longer needed
func (c *external) Disconnect(ctx context.Context) error {
	// No cleanup needed for now
	return nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	// SDL is a configuration resource, no external cleanup needed
	return managed.ExternalDelete{}, nil
}

func (c *external) validateSDL(ctx context.Context, cr *v1alpha1.SDL) (managed.ExternalCreation, error) {
	// Validate SDL using the client (convert CRD SDL to internal format)
	internalSDL := convertToInternalSDL(cr.Spec.ForProvider)
	validationErrors := c.service.ValidateSDL(internalSDL)

	// Generate hash for the current SDL
	hash, err := generateSDLHash(cr.Spec.ForProvider)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot generate SDL hash")
	}

	// Update status
	now := metav1.NewTime(time.Now())
	cr.Status.AtProvider.Hash = hash
	cr.Status.AtProvider.LastValidated = &now
	cr.Status.AtProvider.Validated = len(validationErrors) == 0
	cr.Status.AtProvider.ValidationErrors = validationErrors

	if len(validationErrors) == 0 {
		cr.SetConditions(xpv1.Available())
	} else {
		cr.SetConditions(xpv1.Unavailable().WithMessage(fmt.Sprintf("SDL validation failed: %v", validationErrors)))
	}

	return managed.ExternalCreation{}, nil
}

// generateSDLHash creates a deterministic hash of the SDL specification
func generateSDLHash(sdl v1alpha1.SDLParameters) (string, error) {
	data, err := json.Marshal(sdl)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash), nil
}

// convertToInternalSDL converts the CRD SDL to the internal SDL type
func convertToInternalSDL(crdSDL v1alpha1.SDLParameters) *clienttypes.SDL {
	internal := &clienttypes.SDL{
		Version:    crdSDL.Version,
		Services:   make(map[string]clienttypes.SDLService),
		Profiles:   convertProfiles(crdSDL.Profiles),
		Deployment: make(map[string]clienttypes.SDLDeploymentGroup),
	}

	// Convert services
	for name, service := range crdSDL.Services {
		internal.Services[name] = convertService(service)
	}

	// Convert deployment groups
	for name, group := range crdSDL.Deployment {
		internal.Deployment[name] = clienttypes.SDLDeploymentGroup{
			Profile: group.Profile,
			Count:   int(group.Count),
		}
	}

	return internal
}

func convertService(crdService v1alpha1.SDLService) clienttypes.SDLService {
	service := clienttypes.SDLService{
		Image:   crdService.Image,
		Command: crdService.Command,
		Args:    crdService.Args,
		Env:     crdService.Env,
	}

	// Convert expose specs
	for _, expose := range crdService.Expose {
		exposeSpec := clienttypes.SDLExposeSpec{
			Port:  int(expose.Port),
			Proto: expose.Proto,
		}

		if expose.As != nil {
			exposeSpec.As = int(*expose.As)
		}

		// Convert To specs
		for _, to := range expose.To {
			exposeSpec.To = append(exposeSpec.To, clienttypes.SDLExposeTo{
				Global: to.Global,
			})
		}

		// Convert Accept specs
		if expose.Accept != nil {
			exposeSpec.Accept = expose.Accept.Items
		}

		service.Expose = append(service.Expose, exposeSpec)
	}

	return service
}

func convertProfiles(crdProfiles v1alpha1.SDLProfiles) clienttypes.SDLProfiles {
	profiles := clienttypes.SDLProfiles{
		Compute:   make(map[string]clienttypes.SDLComputeProfile),
		Placement: make(map[string]clienttypes.SDLPlacementProfile),
	}

	// Convert compute profiles
	for name, compute := range crdProfiles.Compute {
		// Convert storage array to internal format
		var storage clienttypes.SDLResourceStorage
		if len(compute.Resources.Storage) > 0 {
			storage = clienttypes.SDLResourceStorage{
				Size:  compute.Resources.Storage[0].Size,
				Class: compute.Resources.Storage[0].Class,
			}
		} else {
			// Default storage if not specified
			storage = clienttypes.SDLResourceStorage{
				Size: "1Gi",
			}
		}
		
		profiles.Compute[name] = clienttypes.SDLComputeProfile{
			Resources: clienttypes.SDLResources{
				CPU:     clienttypes.SDLResourceCPU{Units: compute.Resources.CPU},
				Memory:  clienttypes.SDLResourceMemory{Size: compute.Resources.Memory},
				Storage: storage,
			},
		}
	}

	// Convert placement profiles
	for name, placement := range crdProfiles.Placement {
		placementProfile := clienttypes.SDLPlacementProfile{
			Pricing: make(map[string]clienttypes.SDLPricing),
		}

		// Convert attributes
		if placement.Attributes != nil {
			placementProfile.Attributes = make(map[string]interface{})
			for k, v := range placement.Attributes {
				placementProfile.Attributes[k] = v
			}
		}

		// Convert signedBy
		if placement.SignedBy != nil {
			placementProfile.SignedBy = clienttypes.SDLSignedBy{
				AnyOf: placement.SignedBy.AnyOf,
				AllOf: placement.SignedBy.AllOf,
			}
		}

		// Convert pricing
		for serviceName, pricing := range placement.Pricing {
			placementProfile.Pricing[serviceName] = clienttypes.SDLPricing{
				Denom:  pricing.Denom,
				Amount: pricing.Amount,
			}
		}

		profiles.Placement[name] = placementProfile
	}

	return profiles
}

// getStorageSize is no longer needed - keeping for backward compatibility
func getStorageSize(storage []v1alpha1.SDLStorage) string {
	if len(storage) == 0 {
		return "1Gi" // Default storage size
	}
	return storage[0].Size
}