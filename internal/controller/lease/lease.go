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
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
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

	marketv1beta4 "pkg.akt.dev/go/node/market/v1"
	providerv1 "pkg.akt.dev/go/provider/lease/v1"

	akashv1alpha1 "github.com/overlock-network/provider-akash/apis/akash/v1alpha1"
	resourcev1alpha1 "github.com/overlock-network/provider-akash/apis/resource/v1alpha1"
	apisv1alpha1 "github.com/overlock-network/provider-akash/apis/v1alpha1"
	client "github.com/overlock-network/provider-akash/internal/client"
	clienttypes "github.com/overlock-network/provider-akash/internal/client/types"
	"github.com/overlock-network/provider-akash/internal/features"
)

const (
	errNotLease     = "managed resource is not a Lease custom resource"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"
	errNewClient    = "cannot create new Service"

	// Lease-specific errors
	errGetActiveBid     = "failed to get referenced ActiveBid"
	errCreateLease      = "failed to create lease"
	errQueryLease       = "failed to query lease status"
	errCloseLease       = "failed to close lease"
	errInvalidActiveBid = "referenced ActiveBid is not ready or does not exist"
	errLeaseNotFound    = "lease not found on Akash network"

	// Requeue timing
	statusCheckInterval = 30 * time.Second // Check lease status every 30s
)

// Akash lease states from SDK
var (
	stateActive            = marketv1beta4.Lease_State_name[int32(marketv1beta4.LeaseActive)]
	stateClosed            = marketv1beta4.Lease_State_name[int32(marketv1beta4.LeaseClosed)]
	stateInsufficientFunds = marketv1beta4.Lease_State_name[int32(marketv1beta4.LeaseInsufficientFunds)]
	statePending           = "pending" // This might be a custom state for provider
	statePaused            = "paused"  // For backward compatibility with tests
)

type LeaseService struct {
	client     *client.AkashClient
	kubeClient kubeclient.Client
}

// CreateLeaseFromBid creates a new lease from owner, dseq, gseq, oseq, provider
func (s *LeaseService) CreateLeaseFromBid(ctx context.Context, owner, dseq, gseq, oseq, provider string) (string, error) {
	// Validate sequence numbers for early error detection
	if _, err := strconv.ParseUint(dseq, 10, 64); err != nil {
		return "", fmt.Errorf("invalid dseq '%s': %w", dseq, err)
	}

	if _, err := strconv.ParseUint(gseq, 10, 32); err != nil {
		return "", fmt.Errorf("invalid gseq '%s': %w", gseq, err)
	}

	if _, err := strconv.ParseUint(oseq, 10, 32); err != nil {
		return "", fmt.Errorf("invalid oseq '%s': %w", oseq, err)
	}

	seqs := clienttypes.Seqs{
		Dseq: dseq,
		Gseq: gseq,
		Oseq: oseq,
	}

	// Broadcast MsgCreateLease via chain-sdk
	txhash, err := s.client.CreateLease(ctx, seqs, provider)
	if err != nil {
		return "", fmt.Errorf("failed to create lease: %w", err)
	}

	return txhash, nil
}

// CreateLeaseFromStructuredID creates a lease using structured LeaseID (more efficient)
func (s *LeaseService) CreateLeaseFromStructuredID(ctx context.Context, leaseID *marketv1beta4.LeaseID) (string, error) {
	return s.CreateLeaseFromBid(ctx,
		leaseID.Owner,
		fmt.Sprintf("%d", leaseID.DSeq),
		fmt.Sprintf("%d", leaseID.GSeq),
		fmt.Sprintf("%d", leaseID.OSeq),
		leaseID.Provider)
}

// GetLease gets lease details
func (s *LeaseService) GetLease(ctx context.Context, leaseId string) (*akashv1alpha1.LeaseObservation, error) {
	// Parse lease ID to get components
	owner, dseq, gseq, oseq, provider, err := parseLeaseId(leaseId)
	if err != nil {
		return nil, err
	}

	// Create sequence numbers struct
	seqs := clienttypes.Seqs{
		Dseq: dseq,
		Gseq: gseq,
		Oseq: oseq,
	}

	// Query lease state from chain
	state := stateActive
	if s.client != nil {
		chainState, err := s.client.GetLease(ctx, seqs, provider)
		if err != nil {
			return nil, fmt.Errorf("failed to query lease %s on chain: %w", leaseId, err)
		}
		state = chainState
	}

	lease := &akashv1alpha1.LeaseObservation{
		LeaseId:  leaseId,
		Owner:    owner,
		Dseq:     dseq,
		Gseq:     gseq,
		Oseq:     oseq,
		Provider: provider,
		State:    state,
	}

	return lease, nil
}

// CloseLease closes/terminates a lease
func (s *LeaseService) CloseLease(ctx context.Context, leaseId string) error {
	// Parse lease ID to get components
	_, dseq, gseq, oseq, provider, err := parseLeaseId(leaseId)
	if err != nil {
		return fmt.Errorf("failed to parse lease ID %s: %w", leaseId, err)
	}

	// Create sequence numbers struct
	seqs := clienttypes.Seqs{
		Dseq: dseq,
		Gseq: gseq,
		Oseq: oseq,
	}

	if s.client == nil {
		fmt.Printf("Would close lease: %s (client not available)\n", leaseId)
		return nil
	}

	txhash, err := s.client.CloseLease(ctx, seqs, provider)
	if err != nil {
		return fmt.Errorf("failed to close lease %s: %w", leaseId, err)
	}
	fmt.Printf("Lease %s closed successfully (tx %s)\n", leaseId, txhash)
	return nil
}

// GetLeaseServices gets services running under a lease
func (s *LeaseService) GetLeaseServices(ctx context.Context, leaseId string) ([]akashv1alpha1.LeaseService, error) {
	_, dseq, gseq, oseq, provider, err := parseLeaseId(leaseId)
	if err != nil {
		return nil, fmt.Errorf("failed to parse lease ID %s: %w", leaseId, err)
	}

	seqs := clienttypes.Seqs{
		Dseq: dseq,
		Gseq: gseq,
		Oseq: oseq,
	}

	// Query lease services via provider-services CLI (if client is available)
	if s.client != nil {
		result, err := s.client.GetLeaseServices(seqs, provider)
		if err != nil {
			// If query fails, log error and return empty list
			fmt.Printf("Failed to query lease services via CLI: %v\n", err)
		} else {
			// Parse JSON result and convert to []akashv1alpha1.LeaseService
			services, parseErr := parseLeaseServicesFromJSON(result)
			if parseErr != nil {
				fmt.Printf("Failed to parse lease services JSON: %v\n", parseErr)
				fmt.Printf("Raw response: %s\n", result)
			} else {
				return services, nil
			}
		}
	}

	// Return empty services list (when client not available or parsing fails)
	return []akashv1alpha1.LeaseService{}, nil
}

// newLeaseService creates LeaseService with AkashClient and Kubernetes client
var newLeaseService = func(ctx context.Context, kubeClient kubeclient.Client, usage resource.Tracker, mg resource.Managed, pcInfo client.ProviderConfigInfo) (*LeaseService, error) {
	c, err := client.NewFromManagedResource(ctx, kubeClient, usage, mg, pcInfo)
	if err != nil {
		return nil, err
	}
	return &LeaseService{client: c, kubeClient: kubeClient}, nil
}

// Setup adds a controller that reconciles Lease managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(akashv1alpha1.LeaseGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(akashv1alpha1.LeaseGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kubeClient:           mgr.GetClient(),
			usage:                resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			createLeaseServiceFn: newLeaseService}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&akashv1alpha1.Lease{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method is called.
type connector struct {
	kubeClient           kubeclient.Client
	usage                resource.Tracker
	createLeaseServiceFn func(ctx context.Context, kubeClient kubeclient.Client, usage resource.Tracker, mg resource.Managed, pcInfo client.ProviderConfigInfo) (*LeaseService, error)
}

// Connect produces an ExternalClient with ready-to-use AkashClient
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*akashv1alpha1.Lease)
	if !ok {
		return nil, errors.New(errNotLease)
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

	svc, err := c.createLeaseServiceFn(ctx, c.kubeClient, c.usage, mg, pcInfo)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{service: svc}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an external resource
type external struct {
	service *LeaseService
}

// Observe monitors the lease status on Akash network
func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*akashv1alpha1.Lease)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotLease)
	}

	fmt.Printf("Observing Lease: %s\n", cr.Name)

	if cr.GetDeletionTimestamp() != nil {
		if cr.Status.AtProvider.LeaseId != "" {
			if lease, err := c.service.GetLease(ctx, cr.Status.AtProvider.LeaseId); err == nil {
				cr.Status.AtProvider.State = lease.State
			}
		}
		if cr.Status.AtProvider.State == stateActive {
			return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
		}
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	err := c.resolveReferences(ctx, cr)
	if err != nil {
		if apierrors.IsNotFound(errors.Cause(err)) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		c.setFailedState(cr, fmt.Sprintf("Failed to resolve references: %v", err))
		return managed.ExternalObservation{}, err
	}

	if cr.Status.AtProvider.LeaseId != "" {
		lease, err := c.service.GetLease(ctx, cr.Status.AtProvider.LeaseId)
		if err != nil {
			fmt.Printf("Lease not found on network, needs to be created: %v\n", err)
			cr.SetConditions(xpv1.ReconcileSuccess().WithMessage("Lease needs to be created"))
			cr.SetConditions(xpv1.Unavailable().WithMessage("Lease not yet created"))
			return managed.ExternalObservation{
				ResourceExists:   false,
				ResourceUpToDate: false,
			}, nil
		}

		cr.Status.AtProvider = *lease

		services, err := c.service.GetLeaseServices(ctx, cr.Status.AtProvider.LeaseId)
		if err == nil {
			cr.Status.AtProvider.Services = services
		}

		switch cr.Status.AtProvider.State {
		case stateActive:
			cr.SetConditions(xpv1.ReconcileSuccess().WithMessage("Lease active"))
			cr.SetConditions(xpv1.Available().WithMessage("Lease is active"))
			if err := c.ensureCertificate(ctx, cr); err != nil {
				fmt.Printf("ensureCertificate failed: %v\n", err)
			}
			if err := c.ensureManifest(ctx, cr); err != nil {
				fmt.Printf("ensureManifest failed: %v\n", err)
			}
		case stateClosed:
			cr.SetConditions(xpv1.ReconcileSuccess().WithMessage("Lease closed"))
			cr.SetConditions(xpv1.Unavailable().WithMessage("Lease is closed"))
		default:
			cr.SetConditions(xpv1.ReconcileSuccess().WithMessage(fmt.Sprintf("Lease state: %s", cr.Status.AtProvider.State)))
			cr.SetConditions(xpv1.Available().WithMessage("Lease status updated"))
		}
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
		ConnectionDetails: managed.ConnectionDetails{
			"leaseId":  []byte(cr.Status.AtProvider.LeaseId),
			"state":    []byte(cr.Status.AtProvider.State),
			"provider": []byte(cr.Status.AtProvider.Provider),
		},
	}, nil
}

// Create creates a new lease on Akash network
func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*akashv1alpha1.Lease)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotLease)
	}

	fmt.Printf("Creating Lease: %s\n", cr.Name)

	// Resolve references to get lease parameters
	err := c.resolveReferences(ctx, cr)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "failed to resolve references")
	}

	result, err := c.service.CreateLeaseFromBid(ctx,
		cr.Status.AtProvider.Owner,
		cr.Status.AtProvider.Dseq,
		cr.Status.AtProvider.Gseq,
		cr.Status.AtProvider.Oseq,
		cr.Status.AtProvider.Provider)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateLease)
	}

	fmt.Printf("Lease created successfully: %s\n", result)

	cr.Status.AtProvider.CreatedAt = metav1.Now().Unix()
	cr.SetConditions(xpv1.ReconcileSuccess().WithMessage("Lease created"))

	return managed.ExternalCreation{
		ConnectionDetails: managed.ConnectionDetails{
			"status":   []byte(statePending),
			"leaseId":  []byte(cr.Status.AtProvider.LeaseId),
			"provider": []byte(cr.Status.AtProvider.Provider),
		},
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*akashv1alpha1.Lease)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotLease)
	}

	fmt.Printf("Updating Lease: %s\n", cr.Name)

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

// Delete closes the lease on Akash network
func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*akashv1alpha1.Lease)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotLease)
	}

	fmt.Printf("Deleting Lease: %s\n", cr.Name)

	if cr.Status.AtProvider.LeaseId != "" {
		if cr.Status.AtProvider.State == stateActive {
			err := c.service.CloseLease(ctx, cr.Status.AtProvider.LeaseId)
			if err != nil {
				fmt.Printf("Warning: Failed to close lease %s: %v\n", cr.Status.AtProvider.LeaseId, err)
			}
		}
	}

	return managed.ExternalDelete{}, nil
}

// setFailedState sets the Lease to failed state
func (c *external) setFailedState(cr *akashv1alpha1.Lease, message string) {
	cr.SetConditions(xpv1.ReconcileError(errors.New(message)))
	cr.SetConditions(xpv1.Unavailable().WithMessage("Lease in failed state"))
}

// resolveReferences resolves Deployment and ActiveBid references and populates lease information
func (c *external) resolveReferences(ctx context.Context, cr *akashv1alpha1.Lease) error {
	deployment, err := c.getReferencedDeployment(ctx, cr)
	if err != nil {
		return fmt.Errorf("failed to get Deployment: %w", err)
	}

	activeBid, err := c.getReferencedActiveBid(ctx, cr)
	if err != nil {
		return fmt.Errorf("failed to get ActiveBid: %w", err)
	}

	// Populate lease information from references
	if cr.Status.AtProvider.LeaseId == "" {
		cr.Status.AtProvider.Owner = deployment.Status.AtProvider.Owner
		cr.Status.AtProvider.Dseq = deployment.Status.AtProvider.DeploymentId
		cr.Status.AtProvider.Gseq = activeBid.Status.AtProvider.Gseq
		cr.Status.AtProvider.Oseq = activeBid.Status.AtProvider.Oseq
		cr.Status.AtProvider.Provider = activeBid.Status.AtProvider.Provider
		cr.Status.AtProvider.Price = activeBid.Status.AtProvider.Price

		leaseId := fmt.Sprintf("%s-%s-%s-%s-%s",
			cr.Status.AtProvider.Owner,
			cr.Status.AtProvider.Dseq,
			cr.Status.AtProvider.Gseq,
			cr.Status.AtProvider.Oseq,
			cr.Status.AtProvider.Provider)
		cr.Status.AtProvider.LeaseId = leaseId
	}

	return nil
}

// getReferencedDeployment gets the Deployment referenced by the lease
func (c *external) getReferencedDeployment(ctx context.Context, cr *akashv1alpha1.Lease) (*resourcev1alpha1.Deployment, error) {
	deploymentRef := cr.Spec.ForProvider.DeploymentRef

	namespace := deploymentRef.Namespace
	if namespace == nil {
		ns := cr.Namespace
		namespace = &ns
	}

	deployment := &resourcev1alpha1.Deployment{}
	err := c.service.kubeClient.Get(ctx, types.NamespacedName{
		Name:      deploymentRef.Name,
		Namespace: *namespace,
	}, deployment)
	if err != nil {
		return nil, fmt.Errorf("failed to get Deployment %s/%s: %w", *namespace, deploymentRef.Name, err)
	}

	if deployment.Status.AtProvider.Owner == "" || deployment.Status.AtProvider.DeploymentId == "" {
		return nil, errors.New("Deployment does not have required owner/deploymentId information")
	}

	return deployment, nil
}

// getReferencedActiveBid gets the ActiveBid referenced by the lease
func (c *external) getReferencedActiveBid(ctx context.Context, cr *akashv1alpha1.Lease) (*akashv1alpha1.ActiveBid, error) {
	activeBidRef := cr.Spec.ForProvider.ActiveBidRef

	namespace := activeBidRef.Namespace
	if namespace == "" {
		namespace = cr.Namespace
	}

	activeBid := &akashv1alpha1.ActiveBid{}
	err := c.service.kubeClient.Get(ctx, types.NamespacedName{
		Name:      activeBidRef.Name,
		Namespace: namespace,
	}, activeBid)
	if err != nil {
		return nil, fmt.Errorf("failed to get ActiveBid %s/%s: %w", namespace, activeBidRef.Name, err)
	}

	if activeBid.Status.AtProvider.Provider == "" {
		return nil, errors.New("ActiveBid does not have provider information")
	}

	return activeBid, nil
}

// parseLeaseId parses a lease ID into its components (legacy function for backward compatibility)
func parseLeaseId(leaseId string) (owner, dseq, gseq, oseq, provider string, err error) {
	leaseIDStruct, err := parseLeaseIdToStruct(leaseId)
	if err != nil {
		return "", "", "", "", "", err
	}

	return leaseIDStruct.Owner,
		fmt.Sprintf("%d", leaseIDStruct.DSeq),
		fmt.Sprintf("%d", leaseIDStruct.GSeq),
		fmt.Sprintf("%d", leaseIDStruct.OSeq),
		leaseIDStruct.Provider,
		nil
}

// parseLeaseServicesFromJSON parses JSON response from provider-services lease-status into LeaseService slice
func parseLeaseServicesFromJSON(jsonData string) ([]akashv1alpha1.LeaseService, error) {
	if strings.TrimSpace(jsonData) == "" {
		return []akashv1alpha1.LeaseService{}, nil
	}

	if strings.TrimSpace(jsonData) == "{}" {
		return []akashv1alpha1.LeaseService{}, nil
	}

	var rawServicesMap map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonData), &rawServicesMap); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if servicesRaw, ok := rawServicesMap["services"]; ok {
		return parseLeaseServicesFromJSON(string(servicesRaw))
	}

	if len(rawServicesMap) == 0 {
		return []akashv1alpha1.LeaseService{}, nil
	}

	var testServicesMap map[string]struct {
		Name      string   `json:"name,omitempty"`
		Available bool     `json:"available,omitempty"`
		URIs      []string `json:"uris,omitempty"`
	}
	if err := json.Unmarshal([]byte(jsonData), &testServicesMap); err == nil {
		if len(testServicesMap) > 0 {
			return convertTestServicesToLeaseServices(testServicesMap), nil
		}
	}

	var akashServicesMap map[string]providerv1.LeaseServiceStatus
	if err := json.Unmarshal([]byte(jsonData), &akashServicesMap); err == nil {
		return convertAkashServicesToLeaseServices(akashServicesMap), nil
	}

	return nil, fmt.Errorf("failed to parse JSON as any supported service status format")
}

// convertAkashServicesToLeaseServices converts Akash SDK services to our format
func convertAkashServicesToLeaseServices(servicesMap map[string]providerv1.LeaseServiceStatus) []akashv1alpha1.LeaseService {
	var leaseServices []akashv1alpha1.LeaseService
	for serviceName, akashService := range servicesMap {
		leaseService := akashv1alpha1.LeaseService{
			Name:      serviceName,
			Available: akashService.Available > 0, // Convert int32 to bool
			URIs:      akashService.Uris,          // Use SDK field directly
		}
		leaseService.Ports = extractPortsFromURIsOptimized(akashService.Uris)
		leaseServices = append(leaseServices, leaseService)
	}
	return leaseServices
}

// convertTestServicesToLeaseServices converts test format services to our format
func convertTestServicesToLeaseServices(servicesMap map[string]struct {
	Name      string   `json:"name,omitempty"`
	Available bool     `json:"available,omitempty"`
	URIs      []string `json:"uris,omitempty"`
}) []akashv1alpha1.LeaseService {
	var leaseServices []akashv1alpha1.LeaseService
	for serviceName, testService := range servicesMap {
		name := serviceName
		if testService.Name != "" {
			name = testService.Name
		}

		leaseService := akashv1alpha1.LeaseService{
			Name:      name,
			Available: testService.Available,
			URIs:      testService.URIs,
		}
		leaseService.Ports = extractPortsFromURIsOptimized(testService.URIs)
		leaseServices = append(leaseServices, leaseService)
	}
	return leaseServices
}

// extractPortsFromURIsOptimized extracts port information from URI strings using standard library
func extractPortsFromURIsOptimized(uris []string) []akashv1alpha1.ServicePort {
	var ports []akashv1alpha1.ServicePort

	for _, uri := range uris {
		if parsedURL, err := url.Parse(uri); err == nil {
			host := parsedURL.Hostname()
			portStr := parsedURL.Port()

			if portStr != "" {
				if port, err := strconv.ParseInt(portStr, 10, 32); err == nil && port > 0 {
					servicePort := akashv1alpha1.ServicePort{
						Port:         int32(port),
						ExternalPort: int32(port),
						Protocol:     "TCP",
						Host:         host,
					}
					ports = append(ports, servicePort)
				}
			}
		}
	}

	return ports
}

// parseLeaseIdToStruct parses a lease ID string into Akash SDK LeaseID struct
func parseLeaseIdToStruct(leaseId string) (*marketv1beta4.LeaseID, error) {
	parts := strings.Split(leaseId, "-")
	if len(parts) != 5 {
		return nil, fmt.Errorf("invalid lease ID format: %s", leaseId)
	}

	dseq, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid dseq in lease ID %s: %w", leaseId, err)
	}

	gseq, err := strconv.ParseUint(parts[2], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid gseq in lease ID %s: %w", leaseId, err)
	}

	oseq, err := strconv.ParseUint(parts[3], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid oseq in lease ID %s: %w", leaseId, err)
	}

	return &marketv1beta4.LeaseID{
		Owner:    parts[0],
		DSeq:     dseq,
		GSeq:     uint32(gseq),
		OSeq:     uint32(oseq),
		Provider: parts[4],
	}, nil
}

// generateLeaseID creates a structured LeaseID from components
func generateLeaseID(owner, dseq, gseq, oseq, provider string) (*marketv1beta4.LeaseID, error) {
	dseqUint, err := strconv.ParseUint(dseq, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid dseq '%s': %w", dseq, err)
	}

	gseqUint, err := strconv.ParseUint(gseq, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid gseq '%s': %w", gseq, err)
	}

	oseqUint, err := strconv.ParseUint(oseq, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid oseq '%s': %w", oseq, err)
	}

	return &marketv1beta4.LeaseID{
		Owner:    owner,
		DSeq:     dseqUint,
		GSeq:     uint32(gseqUint),
		OSeq:     uint32(oseqUint),
		Provider: provider,
	}, nil
}

// leaseIDToString converts structured LeaseID to string format
func leaseIDToString(leaseID *marketv1beta4.LeaseID) string {
	return fmt.Sprintf("%s-%d-%d-%d-%s",
		leaseID.Owner,
		leaseID.DSeq,
		leaseID.GSeq,
		leaseID.OSeq,
		leaseID.Provider)
}

// extractPortsFromURIs - legacy function for backward compatibility with tests
func extractPortsFromURIs(uris []string) []akashv1alpha1.ServicePort {
	return extractPortsFromURIsOptimized(uris)
}

// extractHostFromURI - legacy function for backward compatibility with tests
func extractHostFromURI(uri string) string {
	if parsedURL, err := url.Parse(uri); err == nil && parsedURL.Hostname() != "" {
		return parsedURL.Hostname()
	}

	original := uri

	if idx := strings.Index(uri, "://"); idx != -1 {
		uri = uri[idx+3:]
	}

	if idx := strings.Index(uri, ":"); idx != -1 {
		return uri[:idx]
	}
	if idx := strings.Index(uri, "/"); idx != -1 {
		return uri[:idx]
	}

	if uri == original && !strings.Contains(uri, "/") {
		return uri
	}

	return uri
}

// certResourceName returns the auto-created Certificate CR name for a ProviderConfig.
func certResourceName(providerConfigName string) string {
	return providerConfigName + "-cert"
}

// certSecretName returns the Secret name holding the Certificate's tls.crt/tls.key.
func certSecretName(providerConfigName string) string {
	return providerConfigName + "-cert-secret"
}

// ensureCertificate creates the Certificate CR for the Lease's ProviderConfig if missing.
func (c *external) ensureCertificate(ctx context.Context, lease *akashv1alpha1.Lease) error {
	pcRef := lease.GetProviderConfigReference()
	if pcRef == nil || pcRef.Name == "" {
		return fmt.Errorf("Lease has no providerConfigRef")
	}
	name := certResourceName(pcRef.Name)
	existing := &akashv1alpha1.Certificate{}
	err := c.service.kubeClient.Get(ctx, kubeclient.ObjectKey{Name: name}, existing)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get Certificate %s: %w", name, err)
	}

	secretNS := lease.GetNamespace()
	if secretNS == "" {
		secretNS = "default"
	}

	cert := &akashv1alpha1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"akash.overlock.network/providerconfig": pcRef.Name,
				"akash.overlock.network/managed-by":     "lease",
			},
		},
		Spec: akashv1alpha1.CertificateSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: pcRef,
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Name:      certSecretName(pcRef.Name),
					Namespace: secretNS,
				},
			},
			ForProvider: akashv1alpha1.CertificateParameters{
				Domains:      []string{"example.com"},
				AutoRenew:    func() *bool { b := false; return &b }(),
				ValidityDays: func() *int32 { v := int32(365); return &v }(),
			},
		},
	}
	if err := c.service.kubeClient.Create(ctx, cert); err != nil {
		return fmt.Errorf("create Certificate %s: %w", name, err)
	}
	fmt.Printf("Auto-created Certificate %s for ProviderConfig %s\n", name, pcRef.Name)
	return nil
}

// ensureManifest creates a Manifest CR for the Lease if missing, owned by it.
func (c *external) ensureManifest(ctx context.Context, lease *akashv1alpha1.Lease) error {
	pcRef := lease.GetProviderConfigReference()
	if pcRef == nil || pcRef.Name == "" {
		return fmt.Errorf("Lease has no providerConfigRef")
	}
	name := lease.Name + "-manifest"
	existing := &akashv1alpha1.Manifest{}
	err := c.service.kubeClient.Get(ctx, kubeclient.ObjectKey{Name: name}, existing)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get Manifest %s: %w", name, err)
	}

	secretNS := lease.GetNamespace()
	if secretNS == "" {
		secretNS = "default"
	}
	truthy := true

	manifest := &akashv1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"akash.overlock.network/lease":          lease.Name,
				"akash.overlock.network/providerconfig": pcRef.Name,
				"akash.overlock.network/managed-by":     "lease",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         akashv1alpha1.SchemeGroupVersion.String(),
				Kind:               akashv1alpha1.LeaseKind,
				Name:               lease.Name,
				UID:                lease.UID,
				Controller:         &truthy,
				BlockOwnerDeletion: &truthy,
			}},
		},
		Spec: akashv1alpha1.ManifestSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: pcRef,
			},
			ForProvider: akashv1alpha1.ManifestParameters{
				LeaseRef: akashv1alpha1.ManifestLeaseReference{
					Name: lease.Name,
				},
				CertificateSecretRef: akashv1alpha1.ManifestSecretReference{
					Name:      certSecretName(pcRef.Name),
					Namespace: secretNS,
				},
			},
		},
	}
	if err := c.service.kubeClient.Create(ctx, manifest); err != nil {
		return fmt.Errorf("create Manifest %s: %w", name, err)
	}
	fmt.Printf("Auto-created Manifest %s for Lease %s\n", name, lease.Name)
	return nil
}
