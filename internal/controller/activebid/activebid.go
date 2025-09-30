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

package activebid

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
	errNotActiveBid = "managed resource is not an ActiveBid custom resource"
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
	name := managed.ControllerName(akashv1alpha1.ActiveBidGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(akashv1alpha1.ActiveBidGroupVersionKind),
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
		For(&akashv1alpha1.ActiveBid{}).
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
	cr, ok := mg.(*akashv1alpha1.ActiveBid)
	if !ok {
		return nil, errors.New(errNotActiveBid)
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
	cr, ok := mg.(*akashv1alpha1.ActiveBid)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotActiveBid)
	}

	fmt.Printf("Observing ActiveBids for deployment reference: %s\n", cr.Spec.ForProvider.DeploymentRef.Name)

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

	// Update status with deployment information
	cr.Status.AtProvider.Dseq = dseq
	cr.Status.AtProvider.Owner = owner

	// Get the bidId from spec (set by BidPolicy when creating ActiveBid)
	bidId := cr.Spec.ForProvider.BidId
	if bidId != "" {
		fmt.Printf("Fetching specific bid: %s\n", bidId)
		bid, err := c.service.client.GetBid(ctx, bidId)
		if err != nil {
			fmt.Printf("Error fetching bid %s: %v\n", bidId, err)
			// Keep state as pending if we can't fetch the bid yet
			if cr.Status.AtProvider.State == "" {
				cr.Status.AtProvider.State = "pending"
			}
			setStatusCondition(cr, xpv1.TypeSynced, corev1.ConditionFalse, xpv1.ReasonReconcileError, fmt.Sprintf("Failed to fetch bid: %v", err))
			return managed.ExternalObservation{
				ResourceExists:   true,
				ResourceUpToDate: false,
			}, nil
		}

		// Update status with bid information
		c.updateStatusWithBid(cr, bid, dseq)
		// Set the bidId in status as well
		cr.Status.AtProvider.BidId = bidId
		
		// Transition from pending to received once we successfully fetch the bid
		if cr.Status.AtProvider.State == "" || cr.Status.AtProvider.State == "pending" {
			cr.Status.AtProvider.State = "received"
			// Set receivedAt timestamp
			if cr.Status.AtProvider.ReceivedAt == 0 {
				cr.Status.AtProvider.ReceivedAt = metav1.Now().Unix()
			}
		}

		setStatusCondition(cr, xpv1.TypeSynced, corev1.ConditionTrue, xpv1.ReasonReconcileSuccess, "Successfully fetched bid")
		setStatusCondition(cr, xpv1.TypeReady, corev1.ConditionTrue, xpv1.ReasonAvailable, "Bid data available")
		
		return managed.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: true,
			ConnectionDetails: managed.ConnectionDetails{
				"dseq":     []byte(dseq),
				"provider": []byte(bid.Id.Provider),
				"price":    []byte(fmt.Sprintf("%.2f", bid.Price.Amount)),
				"gseq":     []byte(bid.Id.Gseq),
				"oseq":     []byte(bid.Id.Oseq),
				"bidId":    []byte(bidId),
			},
		}, nil
	}

	// If no bidId is set, this is an error condition
	// ActiveBids should be created with a bidId by BidPolicy
	fmt.Printf("Error: ActiveBid has no bidId set\n")
	cr.Status.AtProvider.State = "pending"
	setStatusCondition(cr, xpv1.TypeSynced, corev1.ConditionFalse, xpv1.ReasonReconcileError, "No bidId specified")
	setStatusCondition(cr, xpv1.TypeReady, corev1.ConditionFalse, xpv1.ReasonUnavailable, "Waiting for bidId")
	
	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: false,
	}, nil
}

// Create is not applicable for ActiveBids as they are auto-created observation resources
func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*akashv1alpha1.ActiveBid)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotActiveBid)
	}

	fmt.Printf("ActiveBid observation started for: %s\n", cr.Name)

	// ActiveBids are observation-only resources, no creation needed
	// Set initial state to pending
	cr.Status.AtProvider.State = "pending"
	setStatusCondition(cr, xpv1.TypeSynced, corev1.ConditionTrue, xpv1.ReasonReconcileSuccess, "ActiveBid observation started")

	return managed.ExternalCreation{
		ConnectionDetails: managed.ConnectionDetails{
			"status": []byte("pending"),
		},
	}, nil
}

// Update handles manual bid acceptance (for cases without BidPolicy)
func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*akashv1alpha1.ActiveBid)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotActiveBid)
	}

	fmt.Printf("Updating ActiveBid for: %s\n", cr.Name)

	// ActiveBids are primarily observation-only
	// Manual bid acceptance will be handled by BidPolicy controller
	// For now, just update the connection details

	return managed.ExternalUpdate{
		ConnectionDetails: managed.ConnectionDetails{
			"lastUpdated": []byte(fmt.Sprintf("%d", metav1.Now().Unix())),
			"dseq":        []byte(cr.Status.AtProvider.Dseq),
			"provider":    []byte(cr.Status.AtProvider.Provider),
			"state":       []byte(cr.Status.AtProvider.State),
		},
	}, nil
}

// Disconnect is called when the ExternalClient is no longer needed
func (c *external) Disconnect(ctx context.Context) error {
	// No cleanup needed for now
	return nil
}

// Delete is a no-op for ActiveBids since they are observations, not resources we create
func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*akashv1alpha1.ActiveBid)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotActiveBid)
	}

	fmt.Printf("Deleting ActiveBid observation: %s\n", cr.Name)
	// Nothing to delete - ActiveBids are observations of the Akash network state
	return managed.ExternalDelete{}, nil
}

// getReferencedDeployment retrieves the Deployment CR referenced by the ActiveBid
// Supports cross-namespace references and validates the deployment is ready
func (c *external) getReferencedDeployment(ctx context.Context, cr *akashv1alpha1.ActiveBid) (*v1alpha1.Deployment, error) {
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
func (c *external) updateStatusWithBid(cr *akashv1alpha1.ActiveBid, bid *clienttypes.Bid, dseq string) {
	// BidId is set from spec, not computed here
	cr.Status.AtProvider.Provider = bid.Id.Provider
	cr.Status.AtProvider.Gseq = bid.Id.Gseq
	cr.Status.AtProvider.Oseq = bid.Id.Oseq
	// Don't override state here - it's managed by the controller's lifecycle
	cr.Status.AtProvider.CreatedAt = bid.CreatedAt
	
	if cr.Status.AtProvider.Price == nil {
		cr.Status.AtProvider.Price = &akashv1alpha1.ActiveBidPriceStatus{}
	}
	cr.Status.AtProvider.Price.Amount = strconv.FormatFloat(float64(bid.Price.Amount), 'f', 6, 32)
	cr.Status.AtProvider.Price.Denom = bid.Price.Denom
}

// setStatusCondition sets a condition on the ActiveBid resource
func setStatusCondition(cr *akashv1alpha1.ActiveBid, conditionType xpv1.ConditionType, status corev1.ConditionStatus, reason xpv1.ConditionReason, message string) {
	cr.SetConditions(xpv1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	})
}

