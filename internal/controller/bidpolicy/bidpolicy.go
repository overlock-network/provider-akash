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

package bidpolicy

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	"github.com/overlock-network/provider-akash/internal/features"
)

const (
	errNotBidPolicy = "managed resource is not a BidPolicy custom resource"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"
	errNewClient    = "cannot create new Service"

	// BidPolicy-specific errors
	errGetDeployment     = "failed to get referenced deployment"
	errObserveBids       = "failed to observe bids"
	errEvaluateBids      = "failed to evaluate bids"
	errAcceptBid         = "failed to accept bid"
	errCreateActiveBid   = "failed to create ActiveBid"
	errInvalidDeployment = "referenced deployment is not ready or does not exist"
	errNoQualifyingBids  = "no bids meet the specified criteria"

	// Default values
	defaultMaxBids           = 10
	defaultSelectionStrategy = "lowest-price"

	// Requeue timing for bid collection (using K8s controller-runtime mechanisms)
	bidCollectionInterval = 30 * time.Second // Check for new bids every 30s
	bidCollectionTimeout  = 2 * time.Minute  // Stop waiting after 2 minutes

	// BidPolicy states
	stateActive         = "active"
	statePaused         = "paused"
	stateFailed         = "failed"
	stateWaitingForBids = "waiting-for-bids"
)

type BidPolicyService struct {
	client     *client.AkashClient
	kubeClient kubeclient.Client
}

// EvaluateBids evaluates ActiveBid CRs against policy criteria
func (s *BidPolicyService) EvaluateBids(ctx context.Context, policy *akashv1alpha1.BidPolicy, activeBids []akashv1alpha1.ActiveBid) ([]akashv1alpha1.ActiveBid, error) {
	var eligibleBids []akashv1alpha1.ActiveBid

	for _, bid := range activeBids {
		if s.bidMeetsPolicy(policy, &bid) {
			eligibleBids = append(eligibleBids, bid)
		}
	}

	return eligibleBids, nil
}

// SelectBid selects the best bid based on policy strategy
func (s *BidPolicyService) SelectBid(ctx context.Context, policy *akashv1alpha1.BidPolicy, activeBids []akashv1alpha1.ActiveBid) (*akashv1alpha1.ActiveBid, string, error) {
	if len(activeBids) == 0 {
		return nil, "", errors.New("no bids to select from")
	}

	strategy := policy.Spec.ForProvider.SelectionStrategy
	if strategy == "" {
		strategy = defaultSelectionStrategy
	}

	switch strategy {
	case "lowest-price":
		return s.selectLowestPrice(activeBids)
	case "best-score":
		return s.selectBestScore(activeBids)
	case "preferred-first":
		return s.selectPreferredFirst(policy, activeBids)
	default:
		return nil, "", fmt.Errorf("unknown selection strategy: %s", strategy)
	}
}

// ValidatePolicy validates policy configuration
func (s *BidPolicyService) ValidatePolicy(ctx context.Context, policy *akashv1alpha1.BidPolicy) error {
	// Validate that either selector or deploymentRef is specified
	if policy.Spec.ForProvider.Selector == nil && policy.Spec.ForProvider.DeploymentRef == nil {
		return errors.New("either selector or deploymentRef must be specified")
	}

	// Validate selection strategy
	validStrategies := []string{"lowest-price", "best-score", "preferred-first"}
	strategy := policy.Spec.ForProvider.SelectionStrategy
	if strategy == "" {
		strategy = defaultSelectionStrategy
	}

	valid := false
	for _, v := range validStrategies {
		if strategy == v {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid selection strategy: %s", strategy)
	}

	return nil
}

// newBidPolicyService creates BidPolicyService with AkashClient and Kubernetes client
var newBidPolicyService = func(ctx context.Context, kubeClient kubeclient.Client, usage resource.Tracker, mg resource.Managed, pcInfo client.ProviderConfigInfo) (*BidPolicyService, error) {
	c, err := client.NewFromManagedResource(ctx, kubeClient, usage, mg, pcInfo)
	if err != nil {
		return nil, err
	}
	return &BidPolicyService{client: c, kubeClient: kubeClient}, nil
}

// Setup adds a controller that reconciles BidPolicy managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(akashv1alpha1.BidPolicyGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(akashv1alpha1.BidPolicyGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kubeClient:               mgr.GetClient(),
			usage:                    resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			createBidPolicyServiceFn: newBidPolicyService}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&akashv1alpha1.BidPolicy{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method is called.
type connector struct {
	kubeClient               kubeclient.Client
	usage                    resource.Tracker
	createBidPolicyServiceFn func(ctx context.Context, kubeClient kubeclient.Client, usage resource.Tracker, mg resource.Managed, pcInfo client.ProviderConfigInfo) (*BidPolicyService, error)
}

// Connect produces an ExternalClient with ready-to-use AkashClient
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*akashv1alpha1.BidPolicy)
	if !ok {
		return nil, errors.New(errNotBidPolicy)
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
	svc, err := c.createBidPolicyServiceFn(ctx, c.kubeClient, c.usage, mg, pcInfo)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{service: svc}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an external resource
type external struct {
	service *BidPolicyService
}

// Observe monitors ActiveBid CRs for deployments matched by selector
func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*akashv1alpha1.BidPolicy)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotBidPolicy)
	}

	fmt.Printf("Observing BidPolicy: %s\n", cr.Name)

	// Validate policy configuration
	if err := c.service.ValidatePolicy(ctx, cr); err != nil {
		c.setFailedState(cr, fmt.Sprintf("Invalid policy configuration: %v", err))
		return managed.ExternalObservation{}, err
	}

	// Update last evaluation time
	now := metav1.Now()
	cr.Status.AtProvider.LastEvaluated = &now

	// Find deployments to monitor
	deployments, err := c.findMatchedDeployments(ctx, cr)
	if err != nil {
		c.setFailedState(cr, fmt.Sprintf("Failed to find deployments: %v", err))
		return managed.ExternalObservation{}, err
	}

	// Update matched deployments in status
	cr.Status.AtProvider.MatchedDeployments = deployments

	if len(deployments) == 0 {
		cr.Status.AtProvider.State = stateActive
		cr.SetConditions(xpv1.ReconcileSuccess())
		cr.SetConditions(xpv1.Available().WithMessage("Policy active, no deployments to monitor"))
		return managed.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: true,
		}, nil
	}

	fmt.Printf("Found %d matched deployments\n", len(deployments))

	// Find ActiveBid CRs for the matched deployments
	activeBids, err := c.findActiveBidsForDeployments(ctx, deployments)
	if err != nil {
		c.setFailedState(cr, fmt.Sprintf("Failed to find ActiveBids: %v", err))
		return managed.ExternalObservation{}, err
	}

	// Update status with found ActiveBids
	cr.Status.AtProvider.ActiveBidsManaged = c.buildActiveBidReferences(activeBids)
	cr.Status.AtProvider.TotalBidsReceived = int32(len(activeBids))

	// Evaluate bids against policy criteria
	eligibleBids, err := c.service.EvaluateBids(ctx, cr, activeBids)
	if err != nil {
		c.setFailedState(cr, fmt.Sprintf("Failed to evaluate bids: %v", err))
		return managed.ExternalObservation{}, err
	}

	cr.Status.AtProvider.EligibleBids = int32(len(eligibleBids))

	fmt.Printf("Found %d eligible bids out of %d total\n", len(eligibleBids), len(activeBids))

	// Process bid selection for each deployment
	shouldRequeue, requeueAfter, err := c.processBidSelections(ctx, cr, deployments, eligibleBids)
	if err != nil {
		c.setFailedState(cr, fmt.Sprintf("Failed to process bid selections: %v", err))
		return managed.ExternalObservation{}, err
	}

	// Handle requeue for waiting state
	if shouldRequeue {
		cr.Status.AtProvider.State = stateWaitingForBids
		cr.SetConditions(xpv1.ReconcileSuccess().WithMessage("Waiting for more bids"))
		cr.SetConditions(xpv1.Unavailable().WithMessage("Waiting for bid collection to complete"))

		return managed.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: false, // Trigger requeue
			ConnectionDetails: managed.ConnectionDetails{
				"state":              []byte(cr.Status.AtProvider.State),
				"matchedDeployments": []byte(fmt.Sprintf("%d", len(deployments))),
				"totalBids":          []byte(fmt.Sprintf("%d", len(activeBids))),
				"eligibleBids":       []byte(fmt.Sprintf("%d", len(eligibleBids))),
				"requeueAfter":       []byte(requeueAfter.String()),
			},
		}, nil
	}

	// Set successful state
	cr.Status.AtProvider.State = stateActive
	cr.SetConditions(xpv1.ReconcileSuccess().WithMessage("Successfully processed bid policy"))
	cr.SetConditions(xpv1.Available().WithMessage("Bid policy active"))

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
		ConnectionDetails: managed.ConnectionDetails{
			"state":              []byte(cr.Status.AtProvider.State),
			"matchedDeployments": []byte(fmt.Sprintf("%d", len(deployments))),
			"totalBids":          []byte(fmt.Sprintf("%d", len(activeBids))),
			"eligibleBids":       []byte(fmt.Sprintf("%d", len(eligibleBids))),
		},
	}, nil
}

// Create initializes a BidPolicy (no external resource to create)
func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*akashv1alpha1.BidPolicy)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotBidPolicy)
	}

	fmt.Printf("BidPolicy monitoring started for: %s\n", cr.Name)

	// Set initial state
	cr.Status.AtProvider.State = stateActive
	cr.SetConditions(xpv1.ReconcileSuccess().WithMessage("BidPolicy monitoring started"))

	return managed.ExternalCreation{
		ConnectionDetails: managed.ConnectionDetails{
			"status": []byte(stateActive),
		},
	}, nil
}

// Update handles policy updates
func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*akashv1alpha1.BidPolicy)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotBidPolicy)
	}

	fmt.Printf("Updating BidPolicy: %s\n", cr.Name)

	// Reset last evaluation time to trigger immediate re-evaluation
	cr.Status.AtProvider.LastEvaluated = nil

	return managed.ExternalUpdate{
		ConnectionDetails: managed.ConnectionDetails{
			"lastUpdated": []byte(fmt.Sprintf("%d", metav1.Now().Unix())),
			"state":       []byte(cr.Status.AtProvider.State),
		},
	}, nil
}

// Disconnect is called when the ExternalClient is no longer needed
func (c *external) Disconnect(ctx context.Context) error {
	return nil
}

// Delete cleans up BidPolicy resources
func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*akashv1alpha1.BidPolicy)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotBidPolicy)
	}

	fmt.Printf("Deleting BidPolicy: %s\n", cr.Name)

	// Clean up any ActiveBid resources created by this policy
	for _, activeBidRef := range cr.Status.AtProvider.ActiveBidsManaged {
		activeBid := &akashv1alpha1.ActiveBid{}
		err := c.service.kubeClient.Get(ctx, types.NamespacedName{
			Name:      activeBidRef.Name,
			Namespace: activeBidRef.Namespace,
		}, activeBid)
		if err == nil {
			// Delete the ActiveBid
			err = c.service.kubeClient.Delete(ctx, activeBid)
			if err != nil {
				fmt.Printf("Warning: Failed to delete ActiveBid %s: %v\n", activeBidRef.Name, err)
			}
		}
	}

	return managed.ExternalDelete{}, nil
}

// bidMeetsPolicy checks if an ActiveBid meets the policy criteria.
// Bid prices are stored as decimal-formatted strings (e.g. "2.372595") because
// chain DecCoin amounts carry sub-uact precision; integer parsing silently
// skipped every bid before #21 was reworked.
func (s *BidPolicyService) bidMeetsPolicy(policy *akashv1alpha1.BidPolicy, bid *akashv1alpha1.ActiveBid) bool {
	spec := policy.Spec.ForProvider

	// Reject bids that aren't currently open. A "lost" or "closed" bid is
	// no longer actionable on chain.
	if bid.Status.AtProvider.State != "" && bid.Status.AtProvider.State != "open" {
		return false
	}

	if spec.MaxPrice != nil && bid.Status.AtProvider.Price != nil {
		if price, err := strconv.ParseFloat(bid.Status.AtProvider.Price.Amount, 64); err == nil {
			if price > float64(*spec.MaxPrice) {
				return false
			}
		}
	}

	for _, excluded := range spec.ExcludedProviders {
		if bid.Status.AtProvider.Provider == excluded {
			return false
		}
	}

	return true
}

// selectLowestPrice selects the bid with the lowest price. Prices are decimal
// strings ("2.372595") so we parse as float; selection compares numerically.
func (s *BidPolicyService) selectLowestPrice(bids []akashv1alpha1.ActiveBid) (*akashv1alpha1.ActiveBid, string, error) {
	var selectedBid *akashv1alpha1.ActiveBid
	lowestPrice := -1.0

	for i := range bids {
		bid := &bids[i]
		if bid.Status.AtProvider.Price == nil {
			continue
		}
		price, err := strconv.ParseFloat(bid.Status.AtProvider.Price.Amount, 64)
		if err != nil {
			continue
		}
		if lowestPrice < 0 || price < lowestPrice {
			lowestPrice = price
			selectedBid = bid
		}
	}

	if selectedBid == nil {
		return nil, "", errors.New("no valid bids with pricing information")
	}

	denom := "uact"
	if selectedBid.Status.AtProvider.Price != nil && selectedBid.Status.AtProvider.Price.Denom != "" {
		denom = selectedBid.Status.AtProvider.Price.Denom
	}
	reason := fmt.Sprintf("Selected lowest price bid: %s %s from provider %s",
		selectedBid.Status.AtProvider.Price.Amount, denom, selectedBid.Status.AtProvider.Provider)
	return selectedBid, reason, nil
}

// selectBestScore selects the bid with the best provider score
func (s *BidPolicyService) selectBestScore(bids []akashv1alpha1.ActiveBid) (*akashv1alpha1.ActiveBid, string, error) {
	if len(bids) == 0 {
		return nil, "", errors.New("no bids available")
	}

	// TODO: Implement actual provider scoring when provider reputation data is available
	// For now, fall back to lowest price selection
	return s.selectLowestPrice(bids)
}

// selectPreferredFirst selects based on preferred provider list
func (s *BidPolicyService) selectPreferredFirst(policy *akashv1alpha1.BidPolicy, bids []akashv1alpha1.ActiveBid) (*akashv1alpha1.ActiveBid, string, error) {
	preferredProviders := policy.Spec.ForProvider.PreferredProviders

	// First, try to find a preferred provider
	for _, preferred := range preferredProviders {
		for i := range bids {
			bid := &bids[i]
			if bid.Status.AtProvider.Provider == preferred {
				reason := fmt.Sprintf("Selected preferred provider %s", preferred)
				return bid, reason, nil
			}
		}
	}

	// If no preferred provider found, fall back to lowest price
	return s.selectLowestPrice(bids)
}

// Helper methods for the new controller logic

// setFailedState sets the BidPolicy to failed state
func (c *external) setFailedState(cr *akashv1alpha1.BidPolicy, message string) {
	cr.Status.AtProvider.State = stateFailed
	cr.SetConditions(xpv1.ReconcileError(errors.New(message)))
	cr.SetConditions(xpv1.Unavailable().WithMessage("BidPolicy in failed state"))
}

// findMatchedDeployments finds deployments matching the policy selector or reference
func (c *external) findMatchedDeployments(ctx context.Context, cr *akashv1alpha1.BidPolicy) ([]akashv1alpha1.DeploymentReference, error) {
	var deployments []akashv1alpha1.DeploymentReference

	// If deploymentRef is specified, use it directly
	if cr.Spec.ForProvider.DeploymentRef != nil {
		namespace := cr.Namespace
		if cr.Spec.ForProvider.DeploymentRef.Namespace != nil {
			namespace = *cr.Spec.ForProvider.DeploymentRef.Namespace
		}
		deployments = append(deployments, akashv1alpha1.DeploymentReference{
			Name:      cr.Spec.ForProvider.DeploymentRef.Name,
			Namespace: &namespace,
		})
		return deployments, nil
	}

	// Use selector to find deployments
	if cr.Spec.ForProvider.Selector != nil {
		selector, err := metav1.LabelSelectorAsSelector(cr.Spec.ForProvider.Selector)
		if err != nil {
			return nil, fmt.Errorf("invalid label selector: %w", err)
		}

		deploymentList := &v1alpha1.DeploymentList{}
		err = c.service.kubeClient.List(ctx, deploymentList, &kubeclient.ListOptions{
			LabelSelector: selector,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list deployments: %w", err)
		}

		for _, dep := range deploymentList.Items {
			deployments = append(deployments, akashv1alpha1.DeploymentReference{
				Name:      dep.Name,
				Namespace: &dep.Namespace,
			})
		}
	}

	return deployments, nil
}

// findActiveBidsForDeployments finds ActiveBid CRs for the given deployments
func (c *external) findActiveBidsForDeployments(ctx context.Context, deployments []akashv1alpha1.DeploymentReference) ([]akashv1alpha1.ActiveBid, error) {
	var allActiveBids []akashv1alpha1.ActiveBid

	for _, depRef := range deployments {
		namespace := depRef.Namespace
		if namespace == nil {
			// Use current namespace if not specified
			ns := "default" // or get from context
			namespace = &ns
		}

		// Find ActiveBids for this deployment
		activeBidList := &akashv1alpha1.ActiveBidList{}
		err := c.service.kubeClient.List(ctx, activeBidList, &kubeclient.ListOptions{
			Namespace: *namespace,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list ActiveBids: %w", err)
		}

		// Filter for ActiveBids referencing this deployment
		for _, activeBid := range activeBidList.Items {
			if activeBid.Spec.ForProvider.DeploymentRef.Name == depRef.Name &&
				(activeBid.Spec.ForProvider.DeploymentRef.Namespace == nil ||
					*activeBid.Spec.ForProvider.DeploymentRef.Namespace == *namespace) {
				allActiveBids = append(allActiveBids, activeBid)
			}
		}
	}

	return allActiveBids, nil
}

// buildActiveBidReferences creates references from ActiveBid CRs
func (c *external) buildActiveBidReferences(activeBids []akashv1alpha1.ActiveBid) []akashv1alpha1.ActiveBidReference {
	var refs []akashv1alpha1.ActiveBidReference

	for _, bid := range activeBids {
		price := int64(0)
		if bid.Status.AtProvider.Price != nil {
			if p, err := strconv.ParseInt(bid.Status.AtProvider.Price.Amount, 10, 64); err == nil {
				price = p
			}
		}

		ref := akashv1alpha1.ActiveBidReference{
			Name:      bid.Name,
			Namespace: bid.Namespace,
			BidId:     bid.Spec.ForProvider.BidId,
			Provider:  bid.Status.AtProvider.Provider,
			Price:     price,
		}
		if bid.Status.AtProvider.CreatedAt != 0 {
			createdAt := metav1.NewTime(metav1.Unix(bid.Status.AtProvider.CreatedAt, 0).Time)
			ref.CreatedAt = &createdAt
		}
		refs = append(refs, ref)
	}

	return refs
}

// processBidSelections processes bid selection for each deployment
func (c *external) processBidSelections(ctx context.Context, cr *akashv1alpha1.BidPolicy, deployments []akashv1alpha1.DeploymentReference, eligibleBids []akashv1alpha1.ActiveBid) (bool, time.Duration, error) {
	// Initialize maps if nil
	if cr.Status.AtProvider.SelectedBids == nil {
		cr.Status.AtProvider.SelectedBids = make(map[string]akashv1alpha1.ActiveBidReference)
	}
	if cr.Status.AtProvider.SelectionReasons == nil {
		cr.Status.AtProvider.SelectionReasons = make(map[string]string)
	}
	if cr.Status.AtProvider.CreatedLeases == nil {
		cr.Status.AtProvider.CreatedLeases = make(map[string]akashv1alpha1.LeaseReference)
	}

	// Group eligible bids by deployment
	bidsByDeployment := make(map[string][]akashv1alpha1.ActiveBid)
	for _, bid := range eligibleBids {
		depName := bid.Spec.ForProvider.DeploymentRef.Name
		bidsByDeployment[depName] = append(bidsByDeployment[depName], bid)
	}

	// Process each deployment
	for _, depRef := range deployments {
		bids := bidsByDeployment[depRef.Name]
		if len(bids) == 0 {
			continue
		}

		// Check if bid selection was already made for this deployment
		if _, alreadySelected := cr.Status.AtProvider.SelectedBids[depRef.Name]; alreadySelected {
			continue
		}

		// Get deployment to check its creation time
		deployment := &v1alpha1.Deployment{}
		namespace := depRef.Namespace
		if namespace == nil {
			ns := "default"
			namespace = &ns
		}

		err := c.service.kubeClient.Get(ctx, types.NamespacedName{
			Name:      depRef.Name,
			Namespace: *namespace,
		}, deployment)
		if err != nil {
			return false, 0, fmt.Errorf("failed to get deployment %s: %w", depRef.Name, err)
		}

		// Calculate time since deployment creation
		timeSinceCreation := time.Since(deployment.CreationTimestamp.Time)

		maxBids := defaultMaxBids
		if cr.Spec.ForProvider.MaxBids != nil {
			maxBids = int(*cr.Spec.ForProvider.MaxBids)
		}

		shouldSelect := len(bids) >= maxBids || timeSinceCreation >= bidCollectionTimeout

		if !shouldSelect {
			// We should wait for more bids - return requeue information
			fmt.Printf("Deployment %s: waiting for more bids (%d/%d collected, %v elapsed)\n",
				depRef.Name, len(bids), maxBids, timeSinceCreation)
			return true, bidCollectionInterval, nil
		}

		// Proceed with bid selection
		selectedBid, reason, err := c.service.SelectBid(ctx, cr, bids)
		if err != nil {
			return false, 0, fmt.Errorf("failed to select bid for deployment %s: %w", depRef.Name, err)
		}

		// Update status
		ref := akashv1alpha1.ActiveBidReference{
			Name:      selectedBid.Name,
			Namespace: selectedBid.Namespace,
			BidId:     selectedBid.Spec.ForProvider.BidId,
			Provider:  selectedBid.Status.AtProvider.Provider,
			CreatedAt: &metav1.Time{Time: metav1.Now().Time},
		}
		if selectedBid.Status.AtProvider.Price != nil {
			if price, err := strconv.ParseInt(selectedBid.Status.AtProvider.Price.Amount, 10, 64); err == nil {
				ref.Price = price
			}
		}

		cr.Status.AtProvider.SelectedBids[depRef.Name] = ref
		cr.Status.AtProvider.SelectionReasons[depRef.Name] = reason

		fmt.Printf("Selected bid %s for deployment %s: %s\n", selectedBid.Spec.ForProvider.BidId, depRef.Name, reason)

		// Auto-accept if enabled
		if cr.Spec.ForProvider.AutoAccept {
			err = c.createLeaseForBid(ctx, cr, depRef.Name, selectedBid)
			if err != nil {
				return false, 0, fmt.Errorf("failed to create lease for deployment %s: %w", depRef.Name, err)
			}
		}
	}

	return false, 0, nil
}

// createLeaseForBid creates a Lease CR pointing at the selected ActiveBid.
// The Lease controller's Create then broadcasts MsgCreateLease on chain.
//
// Lease is a regular Crossplane managed resource, so we just write the CR;
// the rest of the lifecycle (broadcast, observe, delete) lives in the lease
// module. Idempotent: if a Lease for this deployment already exists we
// return nil without re-creating it.
func (c *external) createLeaseForBid(ctx context.Context, cr *akashv1alpha1.BidPolicy, deploymentName string, selectedBid *akashv1alpha1.ActiveBid) error {
	leaseName := fmt.Sprintf("%s-lease", deploymentName)
	depNs := ""
	if selectedBid.Spec.ForProvider.DeploymentRef.Namespace != nil {
		depNs = *selectedBid.Spec.ForProvider.DeploymentRef.Namespace
	}

	existing := &akashv1alpha1.Lease{}
	err := c.service.kubeClient.Get(ctx, kubeclient.ObjectKey{Name: leaseName}, existing)
	if err == nil {
		// Already created on a previous reconcile — record it in status
		// for observability and stop.
		cr.Status.AtProvider.CreatedLeases[deploymentName] = akashv1alpha1.LeaseReference{
			Name:      existing.Name,
			Namespace: existing.Namespace,
			LeaseId:   existing.Status.AtProvider.LeaseId,
		}
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	// Anchor the Lease to the parent Deployment so K8s GC cascades when
	// the user deletes the Deployment. We can't depend on the Deployment
	// CR's UID directly without fetching it, so do that here.
	dep := &v1alpha1.Deployment{}
	if err := c.service.kubeClient.Get(ctx, kubeclient.ObjectKey{Name: deploymentName, Namespace: depNs}, dep); err != nil {
		return fmt.Errorf("failed to fetch parent Deployment for ownerRef: %w", err)
	}
	truthy := true

	lease := &akashv1alpha1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name: leaseName,
			Labels: map[string]string{
				"akash.overlock.network/deployment": deploymentName,
				"akash.overlock.network/policy":     cr.Name,
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         v1alpha1.SchemeGroupVersion.String(),
				Kind:               v1alpha1.DeploymentKind,
				Name:               dep.Name,
				UID:                dep.UID,
				Controller:         &truthy,
				BlockOwnerDeletion: &truthy,
			}},
		},
		Spec: akashv1alpha1.LeaseSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: cr.Spec.ProviderConfigReference,
			},
			ForProvider: akashv1alpha1.LeaseParameters{
				DeploymentRef: akashv1alpha1.DeploymentReference{
					Name:      deploymentName,
					Namespace: &depNs,
				},
				ActiveBidRef: akashv1alpha1.ActiveBidReference{
					Name:      selectedBid.Name,
					Namespace: selectedBid.Namespace,
					BidId:     selectedBid.Spec.ForProvider.BidId,
					Provider:  selectedBid.Status.AtProvider.Provider,
				},
			},
		},
	}
	if err := c.service.kubeClient.Create(ctx, lease); err != nil {
		return fmt.Errorf("failed to create Lease %s: %w", leaseName, err)
	}

	cr.Status.AtProvider.CreatedLeases[deploymentName] = akashv1alpha1.LeaseReference{
		Name:      leaseName,
		Namespace: lease.Namespace,
		CreatedAt: &metav1.Time{Time: metav1.Now().Time},
	}
	fmt.Printf("Created Lease %s for deployment %s, bid from provider %s\n",
		leaseName, deploymentName, selectedBid.Status.AtProvider.Provider)
	return nil
}
