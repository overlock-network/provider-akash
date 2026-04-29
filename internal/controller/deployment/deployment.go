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
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
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

	"github.com/overlock-network/provider-akash/apis/resource/v1alpha1"
	apisv1alpha1 "github.com/overlock-network/provider-akash/apis/v1alpha1"
	client "github.com/overlock-network/provider-akash/internal/client"
	clienttypes "github.com/overlock-network/provider-akash/internal/client/types"
	"github.com/overlock-network/provider-akash/internal/features"
)

const (
	errNotDeployment = "managed resource is not a Deployment custom resource"
	errTrackPCUsage  = "cannot track ProviderConfig usage"
	errGetPC         = "cannot get ProviderConfig"
	errGetCreds      = "cannot get credentials"
	errNewClient     = "cannot create new Service"

	// Deployment-specific errors
	errNoExternalName    = "no external-name annotation found"
	errNoOwnerAddress    = "owner address not configured in provider"
	errObserveDeployment = "failed to observe deployment"
	errQueryDeployment   = "failed to query deployment"
	errCreateDeployment  = "failed to create deployment"
	errUpdateDeployment  = "failed to update deployment"
	errDeleteDeployment  = "failed to delete deployment"

	// Deployment operation constants
	balanceTolerance    = int64(100000) // 0.1 AKT tolerance for balance comparison
	tolerancePercentage = 10            // 10% tolerance for larger deposits
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

	// Add passphrase info if configured
	if pc.Spec.Passphrase != nil {
		pcInfo.PassphraseSource = &pc.Spec.Passphrase.Source
		pcInfo.PassphraseSelectors = &pc.Spec.Passphrase.CommonCredentialSelectors
	}

	// Create service with AkashClient - this handles everything internally
	svc, err := c.createDeploymentServiceFn(ctx, c.kubeClient, c.usage, mg, pcInfo)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{service: svc, kube: c.kubeClient}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	// A 'client' used to connect to the external resource API. In practice this
	// would be something like an AWS SDK client.
	service *DeploymentService
	// Kubernetes client for resolving SDL references
	kube kubeclient.Client
}

// Observe queries the current state of the deployment from the Akash network
func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Deployment)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotDeployment)
	}

	fmt.Printf("Observing deployment: %s\n", cr.Name)

	// Extract deployment ID from the external-name annotation
	externalName := cr.GetAnnotations()
	var dseq string
	if externalName != nil {
		dseq = externalName["crossplane.io/external-name"]
	}

	// If no external name is set, this resource hasn't been created yet
	if dseq == "" {
		fmt.Printf("No external-name annotation found, deployment not yet created\n")
		return managed.ExternalObservation{
			ResourceExists:   false,
			ResourceUpToDate: false,
		}, nil
	}

	// Check if external name is just the resource name (Crossplane default behavior)
	// If so, treat as not yet created since we need the actual DSEQ from Akash
	if dseq == cr.Name {
		fmt.Printf("External-name is resource name (%s), deployment not yet created with actual DSEQ\n", dseq)
		return managed.ExternalObservation{
			ResourceExists:   false,
			ResourceUpToDate: false,
		}, nil
	}

	// Use the account address from the client configuration as the owner
	owner := c.service.client.Config.AccountAddress
	if owner == "" {
		return managed.ExternalObservation{}, errors.New(errNoOwnerAddress)
	}

	fmt.Printf("Querying deployment with DSEQ: %s, Owner: %s\n", dseq, owner)
	deployment, err := c.service.client.GetDeployment(ctx, dseq, owner)
	if err != nil {
		fmt.Printf("Error querying deployment %s: %v\n", dseq, err)
		return managed.ExternalObservation{}, errors.Wrap(err, errQueryDeployment)
	}

	fmt.Printf("Found deployment: State=%s, DSEQ=%s\n", deployment.DeploymentInfo.State, deployment.DeploymentInfo.DeploymentId.Dseq)

	// Update the observed status with deployment information
	cr.Status.AtProvider.DeploymentId = deployment.DeploymentInfo.DeploymentId.Dseq
	cr.Status.AtProvider.State = deployment.DeploymentInfo.State
	cr.Status.AtProvider.Owner = deployment.DeploymentInfo.DeploymentId.Owner

	// Update escrow balance information if available
	if deployment.EscrowAccount.Balance.Denom != "" {
		cr.Status.AtProvider.EscrowBalance = &v1alpha1.BalanceStatus{
			Denom:  deployment.EscrowAccount.Balance.Denom,
			Amount: deployment.EscrowAccount.Balance.Amount,
		}
	}

	// Extract actual version/creation info from deployment response
	if deployment.DeploymentInfo.CreatedAt > 0 {
		cr.Status.AtProvider.Version = fmt.Sprintf("block-%d", deployment.DeploymentInfo.CreatedAt)
	} else {
		// Fallback: use current time if creation block height not available
		cr.Status.AtProvider.Version = ""
	}

	// Set Ready condition based on deployment state
	setReadyCondition(cr, deployment.DeploymentInfo.State)

	// Set Synced condition to indicate successful observation
	setStatusCondition(cr, xpv1.TypeSynced, corev1.ConditionTrue, xpv1.ReasonReconcileSuccess, "Successfully observed deployment")

	// Determine if the resource is up to date by comparing deployment properties
	isUpToDate := true // Start optimistic

	// Basic checks - if deployment is in an error state, consider it not up to date
	if deployment.DeploymentInfo.State == "paused" || deployment.DeploymentInfo.State == "insufficient_funds" {
		isUpToDate = false
	}

	// Compare escrow balance with desired deposit amount
	if cr.Spec.ForProvider.Deposit != nil {
		desiredDeposit := *cr.Spec.ForProvider.Deposit
		if deployment.EscrowAccount.Balance.Amount != "" {
			// Parse current balance string to int64
			currentBalance, err := strconv.ParseInt(deployment.EscrowAccount.Balance.Amount, 10, 64)
			if err != nil {
				// If we can't parse the balance, consider it not up to date
				isUpToDate = false
			} else {
				// Allow some tolerance (e.g., within 10% or 100000 uakt minimum)
				tolerance := balanceTolerance
				if desiredDeposit > tolerance {
					tolerance = desiredDeposit / tolerancePercentage
				}

				if currentBalance < (desiredDeposit - tolerance) {
					// Current balance is significantly less than desired
					isUpToDate = false
				}
			}
		}
	}

	// Compare desired currency with actual escrow denomination
	if cr.Spec.ForProvider.Currency != nil {
		desiredCurrency := *cr.Spec.ForProvider.Currency
		if deployment.EscrowAccount.Balance.Denom != desiredCurrency {
			isUpToDate = false
		}
	}

	// Compare actual deployment SDL hash with desired spec SDL  
	if deployment.DeploymentInfo.Hash != "" {
		// Get current SDL content to compare
		currentSDLContent, err := c.resolveSDLContent(ctx, cr)
		if err == nil {
			// Generate hash from current SDL spec
			desiredHash := generateSDLHashHex(currentSDLContent)

			// Compare with deployed hash
			if deployment.DeploymentInfo.Hash != desiredHash {
				fmt.Printf("SDL hash mismatch: desired=%s, actual=%s\n", desiredHash, deployment.DeploymentInfo.Hash)
				isUpToDate = false
			}
		}
	}

	// Check if deployment state indicates it needs attention
	if deployment.DeploymentInfo.State == "closed" {
		// Deployment is closed, mark as not existing
		setStatusCondition(cr, xpv1.TypeSynced, corev1.ConditionFalse, xpv1.ReasonDeleting, "Deployment is closed")
		return managed.ExternalObservation{
			ResourceExists:   false,
			ResourceUpToDate: false,
		}, nil
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate,
		ConnectionDetails: managed.ConnectionDetails{
			"deploymentId":  []byte(deployment.DeploymentInfo.DeploymentId.Dseq),
			"owner":         []byte(deployment.DeploymentInfo.DeploymentId.Owner),
			"state":         []byte(deployment.DeploymentInfo.State),
			"escrowBalance": []byte(deployment.EscrowAccount.Balance.Amount + deployment.EscrowAccount.Balance.Denom),
		},
	}, nil
}

// Create creates a new deployment on the Akash network using the CreateDeployment client method
func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Deployment)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotDeployment)
	}

	fmt.Printf("Creating deployment: %s\n", cr.Name)

	// Resolve SDL content from either direct SDL or SDLRef
	sdlContent, err := c.resolveSDLContent(ctx, cr)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "failed to resolve SDL content")
	}

	// Get deposit and currency from spec with defaults
	deposit := int64(v1alpha1.DefaultDepositAmount)
	if cr.Spec.ForProvider.Deposit != nil {
		deposit = *cr.Spec.ForProvider.Deposit
	}

	currency := v1alpha1.DefaultCurrency
	if cr.Spec.ForProvider.Currency != nil {
		currency = *cr.Spec.ForProvider.Currency
	}

	fmt.Printf("Creating deployment with SDL length: %d, deposit: %d %s\n", len(sdlContent), deposit, currency)

	// Create the deployment using the client
	req := clienttypes.DeploymentCreateRequest{
		SDL:      sdlContent,
		Deposit:  deposit,
		Currency: currency,
	}
	seqs, err := c.service.client.CreateDeployment(ctx, req)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateDeployment)
	}

	fmt.Printf("Successfully created deployment with DSEQ: %s\n", seqs.Dseq)

	// Set status conditions for successful creation
	setStatusCondition(cr, xpv1.TypeSynced, corev1.ConditionTrue, xpv1.ReasonReconcileSuccess, "Deployment created successfully")
	setStatusCondition(cr, xpv1.TypeReady, corev1.ConditionUnknown, xpv1.ReasonCreating, "Deployment is being initialized")

	// Set the external-name annotation for future operations
	if cr.GetAnnotations() == nil {
		cr.SetAnnotations(make(map[string]string))
	}
	annotations := cr.GetAnnotations()
	annotations["crossplane.io/external-name"] = seqs.Dseq
	cr.SetAnnotations(annotations)

	// Return connection details for the created deployment
	return managed.ExternalCreation{
		ConnectionDetails: managed.ConnectionDetails{
			"deploymentId": []byte(seqs.Dseq),
			"dseq":         []byte(seqs.Dseq),
			"gseq":         []byte(seqs.Gseq),
			"oseq":         []byte(seqs.Oseq),
			"owner":        []byte(c.service.client.Config.AccountAddress),
			"deposit":      []byte(fmt.Sprintf("%d", deposit)),
			"currency":     []byte(currency),
		},
	}, nil
}

// Update updates an existing deployment on the Akash network using the UpdateDeployment client method
func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Deployment)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotDeployment)
	}

	fmt.Printf("Updating deployment: %s\n", cr.Name)

	// Extract deployment ID from external-name annotation
	externalName := cr.GetAnnotations()
	var dseq string
	if externalName != nil {
		dseq = externalName["crossplane.io/external-name"]
	}

	if dseq == "" {
		return managed.ExternalUpdate{}, errors.New(errNoExternalName)
	}

	// Get owner address
	owner := c.service.client.Config.AccountAddress
	if owner == "" {
		return managed.ExternalUpdate{}, errors.New(errNoOwnerAddress)
	}

	// Resolve SDL content from either direct SDL or SDLRef
	sdlContent, err := c.resolveSDLContent(ctx, cr)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "failed to resolve SDL content")
	}

	// Generate hash from desired SDL content
	desiredHash := generateSDLHashHex(sdlContent)

	// Query current deployment to compare hashes
	fmt.Printf("Querying deployment %s to check if update is needed\n", dseq)
	deployment, err := c.service.client.GetDeployment(ctx, dseq, owner)
	if err != nil {
		// If we can't query the deployment, log and proceed with update
		fmt.Printf("Warning: Could not query deployment for comparison: %v\n", err)
		fmt.Printf("Proceeding with update to be safe\n")
	} else {
		// Compare hashes to determine if update is needed
		currentHash := deployment.DeploymentInfo.Hash
		fmt.Printf("Hash comparison - Current: %s, Desired: %s\n", currentHash, desiredHash)
		
		if currentHash == desiredHash {
			fmt.Printf("SDL hashes match, no update needed - skipping transaction\n")
			// Return success without sending transaction
			return managed.ExternalUpdate{
				ConnectionDetails: managed.ConnectionDetails{
					"deploymentId": []byte(dseq),
					"dseq":         []byte(dseq),
					"owner":        []byte(owner),
				},
			}, nil
		}
		
		fmt.Printf("SDL hashes differ, proceeding with update\n")
	}

	fmt.Printf("Updating deployment %s with SDL content (length: %d)\n", dseq, len(sdlContent))

	// Update the deployment using the client
	err = c.service.client.UpdateDeployment(ctx, dseq, sdlContent)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateDeployment)
	}

	fmt.Printf("Successfully updated deployment %s\n", dseq)

	// Return updated connection details
	return managed.ExternalUpdate{
		ConnectionDetails: managed.ConnectionDetails{
			"deploymentId": []byte(dseq),
			"dseq":         []byte(dseq),
			"owner":        []byte(owner),
		},
	}, nil
}

// Disconnect is called when the ExternalClient is no longer needed
func (c *external) Disconnect(ctx context.Context) error {
	// No cleanup needed for now
	return nil
}

// Delete closes/terminates a deployment on the Akash network using the CloseDeployment client method
func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.Deployment)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotDeployment)
	}

	fmt.Printf("Deleting deployment: %s\n", cr.Name)

	// Extract deployment ID from external-name annotation
	externalName := cr.GetAnnotations()
	var dseq string
	if externalName != nil {
		dseq = externalName["crossplane.io/external-name"]
	}

	// If no external name or it's just the resource name, deployment was never created
	if dseq == "" || dseq == cr.Name {
		fmt.Printf("No valid external deployment ID found, allowing deletion\n")
		return managed.ExternalDelete{}, nil
	}

	// Use the account address from the client configuration as the owner
	owner := c.service.client.Config.AccountAddress
	if owner == "" {
		return managed.ExternalDelete{}, errors.New(errNoOwnerAddress)
	}

	fmt.Printf("Closing deployment with DSEQ: %s, Owner: %s\n", dseq, owner)

	// Close the deployment using the client
	err := c.service.client.CloseDeployment(ctx, dseq, owner)
	if err != nil {
		fmt.Printf("Error closing deployment: %v\n", err)
		// Don't return error for already closed deployments to allow cleanup
		if isDeploymentNotFoundError(err) {
			fmt.Printf("Deployment %s not found, assuming already closed\n", dseq)
			return managed.ExternalDelete{}, nil
		}
		// Return error to prevent deletion if transaction failed
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteDeployment)
	}

	// Transaction sent successfully - deployment is considered closed
	// Allow deletion immediately to avoid spending coins on retries
	fmt.Printf("Successfully sent close deployment transaction for %s, allowing deletion\n", dseq)
	return managed.ExternalDelete{}, nil
}

// isDeploymentNotFoundError checks if the error indicates the deployment was not found
func isDeploymentNotFoundError(err error) bool {
	// This is a helper function to determine if the error indicates
	// that the deployment doesn't exist (already closed/deleted)
	// The exact error checking logic will depend on the Akash SDK error types
	errorStr := err.Error()
	return strings.Contains(errorStr, "not found") ||
		strings.Contains(errorStr, "does not exist") ||
		strings.Contains(errorStr, "deployment closed")
}

// setStatusCondition sets a condition on the deployment resource
func setStatusCondition(cr *v1alpha1.Deployment, conditionType xpv1.ConditionType, status corev1.ConditionStatus, reason xpv1.ConditionReason, message string) {
	cr.SetConditions(xpv1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	})
}

// setReadyCondition sets the Ready condition based on deployment state
func setReadyCondition(cr *v1alpha1.Deployment, state string) {
	switch state {
	case "active":
		setStatusCondition(cr, xpv1.TypeReady, corev1.ConditionTrue, xpv1.ReasonAvailable, "Deployment is active and running")
	case "paused":
		setStatusCondition(cr, xpv1.TypeReady, corev1.ConditionFalse, xpv1.ReasonUnavailable, "Deployment is paused")
	case "closed":
		setStatusCondition(cr, xpv1.TypeReady, corev1.ConditionFalse, xpv1.ReasonDeleting, "Deployment is closed")
	default:
		setStatusCondition(cr, xpv1.TypeReady, corev1.ConditionUnknown, xpv1.ReasonCreating, fmt.Sprintf("Deployment state: %s", state))
	}
}

// generateSDLHashHex creates a consistent hex hash from SDL content
func generateSDLHashHex(sdl string) string {
	// Normalize SDL content by removing extra whitespace and ensuring consistent formatting
	normalizedSDL := strings.TrimSpace(sdl)

	// Create SHA256 hash of the normalized SDL content
	hash := sha256.Sum256([]byte(normalizedSDL))
	return fmt.Sprintf("%x", hash[:])
}

// resolveSDLContent resolves SDL content from SDLRef
func (c *external) resolveSDLContent(ctx context.Context, cr *v1alpha1.Deployment) (string, error) {
	// Resolve SDL from SDLRef
	sdlRef := cr.Spec.ForProvider.SDLRef
	sdlNamespace := sdlRef.Namespace
	if sdlNamespace == "" {
		sdlNamespace = cr.Namespace
	}

	// Get the SDL resource
	sdlResource := &v1alpha1.SDL{}
	if err := c.kube.Get(ctx, types.NamespacedName{
		Name:      sdlRef.Name,
		Namespace: sdlNamespace,
	}, sdlResource); err != nil {
		return "", errors.Wrapf(err, "failed to get SDL resource %s/%s", sdlNamespace, sdlRef.Name)
	}

	// Check if SDL is ready/validated
	if !sdlResource.Status.AtProvider.Validated || len(sdlResource.Status.AtProvider.ValidationErrors) > 0 {
		return "", errors.Errorf("SDL resource %s/%s is not validated or has validation errors: %v", 
			sdlNamespace, sdlRef.Name, sdlResource.Status.AtProvider.ValidationErrors)
	}

	// Convert SDL spec to YAML - now compatible with internal types
	sdlYAML, err := yaml.Marshal(sdlResource.Spec.ForProvider)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal SDL to YAML")
	}

	return string(sdlYAML), nil
}

