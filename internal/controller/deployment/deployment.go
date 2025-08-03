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

package deployment

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	kubeclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/overlock-network/provider-akash/apis/resource/v1alpha1"
	apisv1alpha1 "github.com/overlock-network/provider-akash/apis/v1alpha1"
	client "github.com/overlock-network/provider-akash/internal/client"
	"github.com/overlock-network/provider-akash/internal/features"
)

const (
	errNotDeployment = "managed resource is not a Deployment custom resource"
	errTrackPCUsage  = "cannot track ProviderConfig usage"
	errGetPC         = "cannot get ProviderConfig"
	errGetCreds      = "cannot get credentials"

	errNewClient = "cannot create new Service"
)

type DeploymentService struct {
	client *client.AkashClient
}

// newDeploymentService creates DeploymentService with AkashClient created from managed resource  
var newDeploymentService = func(ctx context.Context, kubeClient kubeclient.Client, usage resource.Tracker, mg resource.Managed, pcInfo client.ProviderConfigInfo) (*DeploymentService, error) {
	c, err := client.NewFromManagedResource(ctx, kubeClient, usage, mg, pcInfo)
	if err != nil {
		return nil, err
	}
	return &DeploymentService{client: c}, nil
}

// Setup adds a controller that reconciles Deployment managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.DeploymentGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.DeploymentGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kubeClient:                mgr.GetClient(),
			usage:                     resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			createDeploymentServiceFn: newDeploymentService}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.Deployment{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kubeClient                kubeclient.Client
	usage                     resource.Tracker
	createDeploymentServiceFn func(ctx context.Context, kubeClient kubeclient.Client, usage resource.Tracker, mg resource.Managed, pcInfo client.ProviderConfigInfo) (*DeploymentService, error)
}

// Connect produces an ExternalClient with ready-to-use AkashClient
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.Deployment)
	if !ok {
		return nil, errors.New(errNotDeployment)
	}

	// Get the ProviderConfig referenced by the managed resource
	pc := &apisv1alpha1.ProviderConfig{}
	if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: cr.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	// Create ProviderConfig info struct directly using ProviderConfig types
	pcInfo := client.ProviderConfigInfo{
		Source:              pc.Spec.Credentials.Source,
		CredentialSelectors: pc.Spec.Credentials.CommonCredentialSelectors,
		Configuration:       pc.Spec.Configuration,
	}

	// Create service with AkashClient - this handles everything internally
	svc, err := c.createDeploymentServiceFn(ctx, c.kubeClient, c.usage, mg, pcInfo)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{service: svc}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	// A 'client' used to connect to the external resource API. In practice this
	// would be something like an AWS SDK client.
	service *DeploymentService
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Deployment)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotDeployment)
	}
	// These fmt statements should be removed in the real implementation.
	fmt.Printf("Observing: %+v", cr)
	deployment, err := c.service.client.GetDeployment("test", "test")
	fmt.Println(deployment)
	if err != nil {
		fmt.Println(err)
	}
	return managed.ExternalObservation{
		// Return false when the external resource does not exist. This lets
		// the managed resource reconciler know that it needs to call Create to
		// (re)create the resource, or that it has successfully been deleted.
		ResourceExists: false,

		// Return false when the external resource exists, but it not up to date
		// with the desired managed resource state. This lets the managed
		// resource reconciler know that it needs to call Update.
		ResourceUpToDate: false,

		// Return any details that may be required to connect to the external
		// resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Deployment)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotDeployment)
	}

	fmt.Printf("Creating: %+v", cr)
	_, err := c.service.client.CreateDeployment("test")
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	return managed.ExternalCreation{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Deployment)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotDeployment)
	}

	fmt.Printf("Updating: %+v", cr)

	return managed.ExternalUpdate{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Deployment)
	if !ok {
		return errors.New(errNotDeployment)
	}

	fmt.Printf("Deleting: %+v", cr)

	return nil
}
