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

package bid

import (
	"context"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
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

	akashv1alpha1 "github.com/overlock-network/provider-akash/apis/akash/v1alpha1"
	"github.com/overlock-network/provider-akash/apis/resource/v1alpha1"
	apisv1alpha1 "github.com/overlock-network/provider-akash/apis/v1alpha1"
	client "github.com/overlock-network/provider-akash/internal/client"
	clienttypes "github.com/overlock-network/provider-akash/internal/client/types"
	"github.com/overlock-network/provider-akash/internal/features"
)

const (
	errNotBid       = "managed resource is not a Bid custom resource"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"
	errNewClient    = "cannot create new Service"

	// Bid-specific errors
	errGetDeployment     = "failed to get referenced deployment"
	errObserveBids       = "failed to observe bids"
	errNoBidsFound       = "no bids found for deployment"
	errAcceptBid         = "failed to accept bid"
	errCloseBid          = "failed to close bid"
	errInvalidDeployment = "referenced deployment is not ready or does not exist"
)

type BidService struct {
	client     *client.AkashClient
	kubeClient kubeclient.Client
}

// newBidService creates BidService with AkashClient and Kubernetes client
var newBidService = func(ctx context.Context, kubeClient kubeclient.Client, usage resource.Tracker, mg resource.Managed, pcInfo client.ProviderConfigInfo) (*BidService, error) {
	c, err := client.NewFromManagedResource(ctx, kubeClient, usage, mg, pcInfo)
	if err != nil {
		return nil, err
	}
	return &BidService{client: c, kubeClient: kubeClient}, nil
}

// Setup adds a controller that reconciles Bid managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(akashv1alpha1.BidGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(akashv1alpha1.BidGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kubeClient:         mgr.GetClient(),
			usage:              resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			createBidServiceFn: newBidService}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&akashv1alpha1.Bid{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method is called.
type connector struct {
	kubeClient         kubeclient.Client
	usage              resource.Tracker
	createBidServiceFn func(ctx context.Context, kubeClient kubeclient.Client, usage resource.Tracker, mg resource.Managed, pcInfo client.ProviderConfigInfo) (*BidService, error)
}

// Connect produces an ExternalClient with ready-to-use AkashClient
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*akashv1alpha1.Bid)
	if !ok {
		return nil, errors.New(errNotBid)
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

	// Add passphrase info if configured
	if pc.Spec.Passphrase != nil {
		pcInfo.PassphraseSource = &pc.Spec.Passphrase.Source
		pcInfo.PassphraseSelectors = &pc.Spec.Passphrase.CommonCredentialSelectors
	}

	// Create service with AkashClient - this handles everything internally
	svc, err := c.createBidServiceFn(ctx, c.kubeClient, c.usage, mg, pcInfo)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{service: svc}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an external resource
type external struct {
	service *BidService
}

// Observe queries the current state of bids for the referenced deployment
func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*akashv1alpha1.Bid)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotBid)
	}

	fmt.Printf("Observing bids for deployment reference: %s\n", cr.Spec.ForProvider.DeploymentRef.Name)

	// Resolve the deployment reference to get DSEQ and owner
	deployment, err := c.getReferencedDeployment(ctx, cr)
	if err != nil {
		setStatusCondition(cr, xpv1.TypeSynced, corev1.ConditionFalse, xpv1.ReasonReconcileError, fmt.Sprintf("Failed to get deployment: %v", err))
		return managed.ExternalObservation{}, errors.Wrap(err, errGetDeployment)
	}

	// Check if deployment has external-name (DSEQ)
	dseq := deployment.GetAnnotations()["crossplane.io/external-name"]
	if dseq == "" {
		setStatusCondition(cr, xpv1.TypeSynced, corev1.ConditionFalse, xpv1.ReasonReconcileError, "Referenced deployment has no DSEQ")
		return managed.ExternalObservation{
			ResourceExists:   false,
			ResourceUpToDate: false,
		}, nil
	}

	// Get owner from deployment status
	owner := deployment.Status.AtProvider.Owner
	if owner == "" {
		setStatusCondition(cr, xpv1.TypeSynced, corev1.ConditionFalse, xpv1.ReasonReconcileError, "Referenced deployment has no owner")
		return managed.ExternalObservation{
			ResourceExists:   false,
			ResourceUpToDate: false,
		}, nil
	}

	fmt.Printf("Querying bids for DSEQ: %s, Owner: %s\n", dseq, owner)

	// Query bids for the deployment
	bids, err := c.service.client.GetBids(ctx, dseq, owner)
	if err != nil {
		fmt.Printf("Error querying bids: %v\n", err)
		setStatusCondition(cr, xpv1.TypeSynced, corev1.ConditionFalse, xpv1.ReasonReconcileError, fmt.Sprintf("Failed to query bids: %v", err))
		return managed.ExternalObservation{
			ResourceExists:   false,
			ResourceUpToDate: false,
		}, nil
	}

	// Update status with deployment information
	cr.Status.AtProvider.Dseq = dseq
	cr.Status.AtProvider.Owner = owner

	if len(bids) == 0 {
		fmt.Printf("No bids found for deployment %s\n", dseq)
		setStatusCondition(cr, xpv1.TypeSynced, corev1.ConditionTrue, xpv1.ReasonReconcileSuccess, "No bids available yet")
		setStatusCondition(cr, xpv1.TypeReady, corev1.ConditionFalse, xpv1.ReasonUnavailable, "Waiting for bids")
		return managed.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: true,
		}, nil
	}

	// Filter bids by maxPrice constraint if specified
	filteredBids := bids
	if cr.Spec.ForProvider.MaxPrice != nil {
		// Convert maxPrice from int64 (uakt) to float32 (akt) for comparison
		maxPriceAkt := float32(*cr.Spec.ForProvider.MaxPrice) / 1000000.0
		filteredBids = bids.FilterByMaxPrice(maxPriceAkt, "akt")
	}

	// Find the best bid (lowest price)
	bestBid := filteredBids.GetLowestPriceBid()
	if bestBid == nil {
		fmt.Printf("No suitable bids found within constraints\n")
		setStatusCondition(cr, xpv1.TypeSynced, corev1.ConditionTrue, xpv1.ReasonReconcileSuccess, "No bids meet criteria")
		setStatusCondition(cr, xpv1.TypeReady, corev1.ConditionFalse, xpv1.ReasonUnavailable, "No acceptable bids")
		return managed.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: true,
		}, nil
	}

	// Update status with best bid information
	c.updateStatusWithBid(cr, bestBid, dseq)

	setStatusCondition(cr, xpv1.TypeSynced, corev1.ConditionTrue, xpv1.ReasonReconcileSuccess, "Successfully observed bids")
	setStatusCondition(cr, xpv1.TypeReady, corev1.ConditionTrue, xpv1.ReasonAvailable, "Bids available")

	// Check if auto-accept is enabled and we should accept this bid
	shouldAccept := cr.Spec.ForProvider.AutoAccept != nil && *cr.Spec.ForProvider.AutoAccept
	isUpToDate := !shouldAccept // If auto-accept is disabled, we're always up to date

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate,
		ConnectionDetails: managed.ConnectionDetails{
			"dseq":     []byte(dseq),
			"provider": []byte(bestBid.Id.Provider),
			"price":    []byte(fmt.Sprintf("%.2f", bestBid.Price.Amount)),
			"gseq":     []byte("1"), // Default group sequence
			"oseq":     []byte("1"), // Default order sequence
		},
	}, nil
}

// Create accepts the best bid if auto-accept is enabled
func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*akashv1alpha1.Bid)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotBid)
	}

	fmt.Printf("Creating bid acceptance for: %s\n", cr.Name)

	// Auto-accept logic would go here
	// For now, we just mark as created since bids are observed, not created
	setStatusCondition(cr, xpv1.TypeSynced, corev1.ConditionTrue, xpv1.ReasonReconcileSuccess, "Bid observation started")

	return managed.ExternalCreation{
		ConnectionDetails: managed.ConnectionDetails{
			"status": []byte("observing"),
		},
	}, nil
}

// Update handles bid acceptance if auto-accept is enabled
func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*akashv1alpha1.Bid)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotBid)
	}

	fmt.Printf("Updating bid for: %s\n", cr.Name)

	// If auto-accept is enabled, accept the current best bid
	if cr.Spec.ForProvider.AutoAccept != nil && *cr.Spec.ForProvider.AutoAccept {
		if cr.Status.AtProvider.Provider != "" && cr.Status.AtProvider.Dseq != "" {
			fmt.Printf("Auto-accepting bid for provider: %s\n", cr.Status.AtProvider.Provider)
			
			err := c.service.client.AcceptBid(ctx, 
				cr.Status.AtProvider.Dseq, 
				cr.Status.AtProvider.Gseq, 
				cr.Status.AtProvider.Oseq, 
				cr.Status.AtProvider.Provider)
			
			if err != nil {
				setStatusCondition(cr, xpv1.TypeSynced, corev1.ConditionFalse, xpv1.ReasonReconcileError, fmt.Sprintf("Failed to accept bid: %v", err))
				return managed.ExternalUpdate{}, errors.Wrap(err, errAcceptBid)
			}
			
			// Update state to indicate bid was accepted
			cr.Status.AtProvider.State = "matched"
			setStatusCondition(cr, xpv1.TypeReady, corev1.ConditionTrue, xpv1.ReasonAvailable, "Bid accepted, lease created")
		}
	}

	return managed.ExternalUpdate{
		ConnectionDetails: managed.ConnectionDetails{
			"lastUpdated": []byte(fmt.Sprintf("%d", metav1.Now().Unix())),
			"dseq":        []byte(cr.Status.AtProvider.Dseq),
			"provider":    []byte(cr.Status.AtProvider.Provider),
			"state":       []byte(cr.Status.AtProvider.State),
		},
	}, nil
}

// Delete is a no-op for bids since they are observations, not resources we create
func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*akashv1alpha1.Bid)
	if !ok {
		return errors.New(errNotBid)
	}

	fmt.Printf("Deleting bid observation: %s\n", cr.Name)
	// Nothing to delete - bids are observations of the Akash network state
	return nil
}

// getReferencedDeployment retrieves the Deployment CR referenced by the Bid
// Supports cross-namespace references and validates the deployment is ready
func (c *external) getReferencedDeployment(ctx context.Context, cr *akashv1alpha1.Bid) (*v1alpha1.Deployment, error) {
	deployment := &v1alpha1.Deployment{}
	
	// Determine the namespace for the deployment reference
	namespace := cr.Namespace
	if cr.Spec.ForProvider.DeploymentRef.Namespace != nil {
		namespace = *cr.Spec.ForProvider.DeploymentRef.Namespace
	}

	// Get the deployment
	err := c.service.kubeClient.Get(ctx, types.NamespacedName{
		Name:      cr.Spec.ForProvider.DeploymentRef.Name,
		Namespace: namespace,
	}, deployment)

	if err != nil {
		return nil, fmt.Errorf("failed to get deployment %s/%s: %w", namespace, cr.Spec.ForProvider.DeploymentRef.Name, err)
	}

	// Validate deployment has required fields
	if deployment.GetAnnotations() == nil {
		return nil, fmt.Errorf("deployment %s/%s has no annotations", namespace, cr.Spec.ForProvider.DeploymentRef.Name)
	}

	dseq := deployment.GetAnnotations()["crossplane.io/external-name"]
	if dseq == "" {
		return nil, fmt.Errorf("deployment %s/%s has no DSEQ (external-name annotation)", namespace, cr.Spec.ForProvider.DeploymentRef.Name)
	}

	if deployment.Status.AtProvider.Owner == "" {
		return nil, fmt.Errorf("deployment %s/%s has no owner information", namespace, cr.Spec.ForProvider.DeploymentRef.Name)
	}

	return deployment, nil
}


// updateStatusWithBid updates the CR status with bid information
func (c *external) updateStatusWithBid(cr *akashv1alpha1.Bid, bid *clienttypes.Bid, dseq string) {
	cr.Status.AtProvider.BidId = fmt.Sprintf("%s-%s-%s-%s-%s", bid.Id.Owner, bid.Id.Dseq, bid.Id.Gseq, bid.Id.Oseq, bid.Id.Provider)
	cr.Status.AtProvider.Provider = bid.Id.Provider
	cr.Status.AtProvider.Gseq = bid.Id.Gseq
	cr.Status.AtProvider.Oseq = bid.Id.Oseq
	cr.Status.AtProvider.State = bid.State
	cr.Status.AtProvider.CreatedAt = bid.CreatedAt
	
	if cr.Status.AtProvider.Price == nil {
		cr.Status.AtProvider.Price = &akashv1alpha1.BidPriceStatus{}
	}
	cr.Status.AtProvider.Price.Amount = strconv.FormatFloat(float64(bid.Price.Amount), 'f', 6, 32)
	cr.Status.AtProvider.Price.Denom = bid.Price.Denom
}

// setStatusCondition sets a condition on the bid resource
func setStatusCondition(cr *akashv1alpha1.Bid, conditionType xpv1.ConditionType, status corev1.ConditionStatus, reason xpv1.ConditionReason, message string) {
	cr.SetConditions(xpv1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	})
}

