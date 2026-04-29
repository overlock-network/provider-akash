package client

import (
	"context"
	"crypto/sha256"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/overlock-network/provider-akash/internal/client/types"
	"gopkg.in/yaml.v3"
	dv1 "pkg.akt.dev/go/node/deployment/v1"
	deploymenttypes "pkg.akt.dev/go/node/deployment/v1beta4"
	depositv1 "pkg.akt.dev/go/node/types/deposit/v1"
	atypes "pkg.akt.dev/go/node/types/attributes/v1"
	rtypes "pkg.akt.dev/go/node/types/resources/v1beta4"
	"pkg.akt.dev/go/node/types/unit"
)

// DepositDenom is the deposit denomination required by node v2 BME for new deployments.
const DepositDenom = "uact"

func (ak *AkashClient) GetDeployments(owner string) ([]clienttypes.DeploymentId, error) {
	client, err := ak.getNodeClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get node client: %w", err)
	}

	queryClient := client.Query()
	deploymentQuery := queryClient.Deployment()

	deploymentsResp, err := deploymentQuery.Deployments(ak.ctx, &deploymenttypes.QueryDeploymentsRequest{
		Filters: deploymenttypes.DeploymentFilters{
			Owner: owner,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query deployments: %w", err)
	}

	var deployments []clienttypes.DeploymentId
	for _, deploymentResp := range deploymentsResp.Deployments {
		id := deploymentResp.Deployment.GetID()
		deployments = append(deployments, clienttypes.DeploymentId{
			Dseq:  fmt.Sprintf("%d", id.DSeq),
			Owner: id.Owner,
		})
	}

	return deployments, nil
}

// GetDeployment retrieves deployment details by DSEQ and owner
func (ak *AkashClient) GetDeployment(ctx context.Context, dseq string, owner string) (clienttypes.Deployment, error) {
	dseqUint, err := strconv.ParseUint(dseq, 10, 64)
	if err != nil {
		return clienttypes.Deployment{}, fmt.Errorf("invalid dseq: %w", err)
	}

	client, err := ak.getNodeClient()
	if err != nil {
		return clienttypes.Deployment{}, fmt.Errorf("failed to get node client: %w", err)
	}

	deploymentID := dv1.DeploymentID{
		DSeq:  dseqUint,
		Owner: owner,
	}

	queryClient := client.Query()
	deploymentQuery := queryClient.Deployment()

	deploymentResp, err := deploymentQuery.Deployment(ak.ctx, &deploymenttypes.QueryDeploymentRequest{
		ID: deploymentID,
	})
	if err != nil {
		return clienttypes.Deployment{}, fmt.Errorf("deployment not found: %w", err)
	}

	id := deploymentResp.Deployment.GetID()
	escrowState := deploymentResp.EscrowAccount.State

	balance := clienttypes.EscrowAccountBalance{Denom: "uakt"}
	if len(escrowState.Funds) > 0 {
		balance.Denom = escrowState.Funds[0].Denom
		balance.Amount = escrowState.Funds[0].Amount.String()
	}

	return clienttypes.Deployment{
		DeploymentInfo: clienttypes.DeploymentInfo{
			State: deploymentResp.Deployment.State.String(),
			DeploymentId: clienttypes.DeploymentId{
				Dseq:  fmt.Sprintf("%d", id.DSeq),
				Owner: id.Owner,
			},
			Hash:      fmt.Sprintf("%x", deploymentResp.Deployment.Hash),
			CreatedAt: deploymentResp.Deployment.CreatedAt,
		},
		EscrowAccount: clienttypes.EscrowAccount{
			Owner:   escrowState.Owner,
			State:   escrowState.State.String(),
			Balance: balance,
		},
	}, nil
}

// CreateDeployment creates a new deployment with SDL content and deposit amount.
// The deposit denom is fixed to uact (DepositDenom) per node v2 BME.
func (ak *AkashClient) CreateDeployment(ctx context.Context, req clienttypes.DeploymentCreateRequest) (clienttypes.Seqs, error) {
	fmt.Printf("Creating deployment with SDL content (length: %d), deposit: %d %s\n", len(req.SDL), req.Deposit, DepositDenom)

	client, err := ak.getNodeClient()
	if err != nil {
		return clienttypes.Seqs{}, fmt.Errorf("failed to get node client: %w", err)
	}

	groups, err := parseSDLToGroupSpecs(req.SDL)
	if err != nil {
		return clienttypes.Seqs{}, fmt.Errorf("failed to parse SDL content: %w", err)
	}

	depositCoin := sdktypes.NewInt64Coin(DepositDenom, req.Deposit)

	// Generate a unique DSEQ based on current timestamp
	dseq := uint64(time.Now().Unix())

	msg := &deploymenttypes.MsgCreateDeployment{
		ID: dv1.DeploymentID{
			Owner: ak.Config.AccountAddress,
			DSeq:  dseq,
		},
		Groups: groups,
		Hash:   generateSDLHash(req.SDL),
		Deposit: depositv1.Deposit{
			Amount:  depositCoin,
			Sources: depositv1.Sources{depositv1.SourceBalance},
		},
	}

	if valErr := ValidateTransactionPreBroadcast(msg, ak.Config.AccountAddress); valErr != nil {
		return clienttypes.Seqs{}, fmt.Errorf("pre-broadcast validation failed: %w", valErr)
	}

	resp, err := client.Tx().BroadcastMsgs(ak.ctx, msg)
	if err != nil {
		fmt.Print(DebugBroadcastError(err))
		return clienttypes.Seqs{}, fmt.Errorf("failed to broadcast MsgCreateDeployment: %w", err)
	}
	if resp.Code != 0 {
		return clienttypes.Seqs{}, fmt.Errorf("MsgCreateDeployment rejected by chain: code=%d codespace=%s log=%s", resp.Code, resp.Codespace, resp.RawLog)
	}

	fmt.Printf("Create deployment transaction succeeded - DSEQ: %d, txhash: %s\n", dseq, resp.TxHash)

	dseqStr := fmt.Sprintf("%d", dseq)
	return clienttypes.Seqs{
		Dseq: dseqStr,
		Gseq: "1",
		Oseq: "1",
	}, nil
}

// Helper function for Go versions that don't have min built-in
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// generateSDLHash creates a consistent hash from SDL content
func generateSDLHash(sdl string) []byte {
	// Normalize SDL content by removing extra whitespace and ensuring consistent formatting
	normalizedSDL := strings.TrimSpace(sdl)

	// Create SHA256 hash of the normalized SDL content
	hash := sha256.Sum256([]byte(normalizedSDL))
	return hash[:]
}

// parseSDLToGroupSpecs parses SDL content and converts it to GroupSpec format
func parseSDLToGroupSpecs(sdl string) ([]deploymenttypes.GroupSpec, error) {
	// Validate input
	if strings.TrimSpace(sdl) == "" {
		return nil, fmt.Errorf("SDL content is empty")
	}

	// Parse YAML SDL content
	var sdlManifest clienttypes.SDL
	if err := yaml.Unmarshal([]byte(sdl), &sdlManifest); err != nil {
		return nil, fmt.Errorf("failed to parse SDL YAML: %w", err)
	}

	// Validate SDL schema
	if err := validateSDL(&sdlManifest); err != nil {
		return nil, fmt.Errorf("SDL validation failed: %w", err)
	}

	// Convert SDL to GroupSpec format
	groups, err := convertSDLToGroupSpecs(&sdlManifest)
	if err != nil {
		return nil, fmt.Errorf("failed to convert SDL to GroupSpec: %w", err)
	}

	return groups, nil
}

// validateSDL performs basic validation of SDL structure
func validateSDL(sdl *clienttypes.SDL) error {
	if sdl.Version == "" {
		return fmt.Errorf("SDL version is required")
	}

	if len(sdl.Services) == 0 {
		return fmt.Errorf("SDL must define at least one service")
	}

	if len(sdl.Profiles.Compute) == 0 {
		return fmt.Errorf("SDL must define at least one compute profile")
	}

	if len(sdl.Profiles.Placement) == 0 {
		return fmt.Errorf("SDL must define at least one placement profile")
	}

	if len(sdl.Deployment) == 0 {
		return fmt.Errorf("SDL must define at least one deployment group")
	}

	// Validate that all services referenced in deployment exist
	for groupName, deployGroup := range sdl.Deployment {
		if _, exists := sdl.Services[groupName]; !exists {
			return fmt.Errorf("deployment group '%s' references undefined service", groupName)
		}

		if _, exists := sdl.Profiles.Placement[deployGroup.Profile]; !exists {
			return fmt.Errorf("deployment group '%s' references undefined placement profile '%s'", groupName, deployGroup.Profile)
		}

		// Validate that compute profile exists for the service
		if _, exists := sdl.Profiles.Compute[groupName]; !exists {
			return fmt.Errorf("deployment group '%s' requires compute profile '%s'", groupName, groupName)
		}
	}

	return nil
}

// convertSDLToGroupSpecs converts parsed SDL to Akash GroupSpec format
func convertSDLToGroupSpecs(sdl *clienttypes.SDL) ([]deploymenttypes.GroupSpec, error) {
	var groups []deploymenttypes.GroupSpec

	// Create a group for each deployment group
	for groupName, deployGroup := range sdl.Deployment {
		service, exists := sdl.Services[groupName]
		if !exists {
			continue // Should have been caught in validation
		}

		// Use compute profile that matches the service/group name
		computeProfile, exists := sdl.Profiles.Compute[groupName]
		if !exists {
			continue // Should have been caught in validation
		}

		// Convert CPU, Memory, Storage from SDL to Akash units
		resources, err := convertSDLResourcesToAkash(computeProfile.Resources)
		if err != nil {
			return nil, fmt.Errorf("failed to convert resources for group '%s': %w", groupName, err)
		}

		// Convert placement requirements from SDL placement profiles
		// Look for placement profile by group name first, then by compute profile name
		var placementProfile clienttypes.SDLPlacementProfile
		var placementProfileExists bool

		if profile, exists := sdl.Profiles.Placement[groupName]; exists {
			placementProfile = profile
			placementProfileExists = true
		} else if profile, exists := sdl.Profiles.Placement[deployGroup.Profile]; exists {
			placementProfile = profile
			placementProfileExists = true
		}

		if !placementProfileExists {
			return nil, fmt.Errorf("placement profile not found for group '%s' (checked '%s' and '%s')", groupName, groupName, deployGroup.Profile)
		}

		placementRequirements, err := convertSDLPlacementToAkash(placementProfile)
		if err != nil {
			return nil, fmt.Errorf("failed to convert placement requirements for group '%s': %w", groupName, err)
		}

		// Get pricing from placement profile for this service
		pricing, exists := placementProfile.Pricing[groupName]
		if !exists {
			return nil, fmt.Errorf("pricing not found for service '%s' in placement profile", groupName)
		}

		priceCoin := sdktypes.NewInt64DecCoin(DepositDenom, pricing.Amount)

		// Create GroupSpec
		groupSpec := deploymenttypes.GroupSpec{
			Name:         groupName,
			Requirements: *placementRequirements,
			Resources: deploymenttypes.ResourceUnits{
				{
					Resources: *resources,
					Count:     uint32(deployGroup.Count),
					Price:     priceCoin,
				},
			},
		}

		// Add service-specific configurations like endpoints from expose spec
		endpoints, err := convertSDLExposeToEndpoints(service.Expose)
		if err != nil {
			return nil, fmt.Errorf("failed to convert expose specs to endpoints for group '%s': %w", groupName, err)
		}
		if len(endpoints) > 0 {
			groupSpec.Resources[0].Resources.Endpoints = endpoints
		}

		groups = append(groups, groupSpec)
	}

	if len(groups) == 0 {
		return nil, fmt.Errorf("no valid deployment groups found")
	}

	return groups, nil
}

// convertSDLResourcesToAkash converts SDL resource specifications to Akash resource format
func convertSDLResourcesToAkash(sdlResources clienttypes.SDLResources) (*rtypes.Resources, error) {
	resources := &rtypes.Resources{
		ID: 1, // Required: Resource ID must be > 0
	}

	// Convert CPU
	cpuVal, err := parseResourceValue(sdlResources.CPU.Units)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CPU units '%s': %w", sdlResources.CPU.Units, err)
	}
	resources.CPU = &rtypes.CPU{
		Units: rtypes.NewResourceValue(cpuVal),
	}

	// Convert Memory
	memoryVal, err := parseMemoryValue(sdlResources.Memory.Size)
	if err != nil {
		return nil, fmt.Errorf("failed to parse memory size '%s': %w", sdlResources.Memory.Size, err)
	}
	resources.Memory = &rtypes.Memory{
		Quantity: rtypes.NewResourceValue(memoryVal),
	}

	// Convert Storage
	var storages []rtypes.Storage
	if sdlResources.Storage.Size != "" {
		// Use storage entry for the conversion
		storageVal, err := parseStorageValue(sdlResources.Storage.Size)
		if err != nil {
			return nil, fmt.Errorf("failed to parse storage size '%s': %w", sdlResources.Storage.Size, err)
		}
		storage := rtypes.Storage{
			Name:     "default",
			Quantity: rtypes.NewResourceValue(storageVal),
		}
		storages = append(storages, storage)
	} else {
		// Default storage if none specified
		storageVal, err := parseStorageValue("1Gi")
		if err != nil {
			return nil, fmt.Errorf("failed to parse default storage size: %w", err)
		}
		storage := rtypes.Storage{
			Name:     "default",
			Quantity: rtypes.NewResourceValue(storageVal),
		}
		storages = append(storages, storage)
	}
	resources.Storage = storages

	// Add GPU field (required in v1beta3, even if 0 units)
	resources.GPU = &rtypes.GPU{
		Units: rtypes.NewResourceValue(0), // 0 GPU units = no GPU required
	}

	return resources, nil
}

// parseResourceValue parses resource value strings (e.g., "0.5", "1", "100m")
// Returns value in millicores as expected by Akash network
func parseResourceValue(value string) (uint64, error) {
	// Handle millicores (e.g., "100m" = 100 millicores)
	if strings.HasSuffix(value, "m") {
		milliValue, err := strconv.ParseUint(strings.TrimSuffix(value, "m"), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid millicores value: %w", err)
		}
		// Value is already in millicores
		return milliValue, nil
	}

	// Handle floating point values (e.g., "0.5")
	if strings.Contains(value, ".") {
		floatVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid float value: %w", err)
		}
		// Convert to millicores (1 core = 1,000 millicores)
		return uint64(floatVal * 1000), nil
	}

	// Handle integer values
	intVal, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid integer value: %w", err)
	}
	// Convert to millicores (1 core = 1,000 millicores)
	return intVal * 1000, nil
}

// parseMemoryValue parses memory size strings (e.g., "512Mi", "1Gi", "1024M")
func parseMemoryValue(size string) (uint64, error) {
	// Regular expression to match number and unit
	re := regexp.MustCompile(`^(\d+(?:\.\d+)?)(Ki|Mi|Gi|Ti|K|M|G|T|B)?$`)
	matches := re.FindStringSubmatch(size)
	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid memory size format: %s", size)
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid numeric value in memory size: %w", err)
	}

	unitStr := matches[2]
	var multiplier uint64 = 1

	switch unitStr {
	case "Ki":
		multiplier = unit.Ki
	case "Mi":
		multiplier = unit.Mi
	case "Gi":
		multiplier = unit.Gi
	case "Ti":
		multiplier = unit.Ti
	case "K":
		multiplier = unit.K
	case "M":
		multiplier = unit.M
	case "G":
		multiplier = unit.G
	case "T":
		multiplier = unit.T
	case "B", "":
		multiplier = 1
	default:
		return 0, fmt.Errorf("unknown memory unit: %s", unitStr)
	}

	return uint64(value * float64(multiplier)), nil
}

// parseStorageValue parses storage size strings (e.g., "1Gi", "100Mi", "5T")
func parseStorageValue(size string) (uint64, error) {
	// Use the same logic as memory parsing since storage units are the same
	return parseMemoryValue(size)
}

// convertSDLPlacementToAkash converts SDL placement profiles to Akash placement requirements
func convertSDLPlacementToAkash(placement clienttypes.SDLPlacementProfile) (*atypes.PlacementRequirements, error) {
	requirements := &atypes.PlacementRequirements{}

	// Convert SignedBy requirements
	if len(placement.SignedBy.AnyOf) > 0 || len(placement.SignedBy.AllOf) > 0 {
		requirements.SignedBy = atypes.SignedBy{
			AnyOf: placement.SignedBy.AnyOf,
			AllOf: placement.SignedBy.AllOf,
		}
	}

	// Convert attributes
	var attributes atypes.Attributes
	for key, value := range placement.Attributes {
		if strValue, ok := value.(string); ok {
			attributes = append(attributes, atypes.NewStringAttribute(key, strValue))
		}
	}
	requirements.Attributes = attributes

	return requirements, nil
}

// convertSDLExposeToEndpoints converts SDL expose specifications to Akash endpoints
func convertSDLExposeToEndpoints(exposeSpecs []clienttypes.SDLExposeSpec) (rtypes.Endpoints, error) {
	var endpoints rtypes.Endpoints

	for _, spec := range exposeSpecs {
		endpoint := rtypes.Endpoint{
			SequenceNumber: uint32(spec.Port),
			Kind:           rtypes.Endpoint_RANDOM_PORT, // Use RANDOM_PORT to ensure kind is included
		}

		// Determine endpoint kind based on the 'to' field
		for _, to := range spec.To {
			if to.Global {
				// For global endpoints, use RANDOM_PORT which has value 1
				endpoint.Kind = rtypes.Endpoint_RANDOM_PORT
				break
			}
		}

		endpoints = append(endpoints, endpoint)
	}

	return endpoints, nil
}

// extractDeploymentSeqsFromResponse attempts to extract DSEQ, GSEQ, OSEQ from transaction response
func extractDeploymentSeqsFromResponse(resp interface{}) (clienttypes.Seqs, error) {
	// Cast response to TxResponse
	txResp, ok := resp.(*sdktypes.TxResponse)
	if !ok {
		return clienttypes.Seqs{}, fmt.Errorf("response is not a TxResponse, got: %T", resp)
	}

	// Check if transaction was successful
	if txResp.Code != 0 {
		fmt.Printf("[ERROR] Transaction failed with code %d\n", txResp.Code)
		fmt.Printf("[ERROR] Codespace: %s\n", txResp.Codespace)
		fmt.Printf("[ERROR] Raw log: %s\n", txResp.RawLog)
		fmt.Printf("[ERROR] Info: %s\n", txResp.Info)
		fmt.Printf("[ERROR] Data: %s\n", txResp.Data)
		fmt.Printf("[ERROR] Events: %+v\n", txResp.Events)
		return clienttypes.Seqs{}, fmt.Errorf("transaction failed with code %d (codespace: %s): %s", txResp.Code, txResp.Codespace, txResp.RawLog)
	}

	// Parse events to find deployment creation event
	var dseq string
	for _, event := range txResp.Events {
		if event.Type == "akash.v1.DeploymentCreated" || event.Type == "deployment_created" {
			// Look for deployment ID in event attributes
			for _, attr := range event.Attributes {
				if string(attr.Key) == "dseq" || string(attr.Key) == "deployment_id" {
					dseq = string(attr.Value)
					break
				}
			}
			if dseq != "" {
				break
			}
		}
	}

	// If we didn't find DSEQ in events, try parsing from logs
	if dseq == "" && len(txResp.Logs) > 0 {
		for _, log := range txResp.Logs {
			for _, event := range log.Events {
				if event.Type == "akash.v1.DeploymentCreated" || event.Type == "deployment_created" {
					for _, attr := range event.Attributes {
						if attr.Key == "dseq" || attr.Key == "deployment_id" {
							dseq = attr.Value
							break
						}
					}
					if dseq != "" {
						break
					}
				}
			}
			if dseq != "" {
				break
			}
		}
	}

	// If still no DSEQ found, return error with helpful information
	if dseq == "" {
		return clienttypes.Seqs{}, fmt.Errorf("could not extract DSEQ from transaction response. Events: %+v", txResp.Events)
	}

	// For now, we set default values for GSEQ and OSEQ
	// In a real implementation, these would be parsed from the group structure
	return clienttypes.Seqs{
		Dseq: strings.Trim(dseq, "\""), // Remove quotes if present
		Gseq: "1",                      // Default group sequence
		Oseq: "1",                      // Default order sequence
	}, nil
}

// CloseDeployment closes an existing deployment
func (ak *AkashClient) CloseDeployment(ctx context.Context, dseq string, owner string) error {
	dseqUint, err := strconv.ParseUint(dseq, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid dseq: %w", err)
	}

	client, err := ak.getNodeClient()
	if err != nil {
		return fmt.Errorf("failed to get node client: %w", err)
	}

	msg := &deploymenttypes.MsgCloseDeployment{
		ID: dv1.DeploymentID{
			DSeq:  dseqUint,
			Owner: owner,
		},
	}

	txClient := client.Tx()
	resp, err := txClient.BroadcastMsgs(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to broadcast close deployment transaction: %w", err)
	}

	fmt.Printf("Deployment closed successfully: %+v\n", resp)
	return nil
}

// AddFunds adds funds to a deployment's escrow account
func (ak *AkashClient) AddFunds(ctx context.Context, dseq string, amount int64) error {
	dseqUint, err := strconv.ParseUint(dseq, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid dseq: %w", err)
	}

	// In Akash, funds are typically added to deployments by closing and recreating
	// with a higher deposit, or through market mechanics (bids, leases).
	// There isn't a direct "add funds" message for existing deployments.

	// For now, we validate the parameters and provide guidance
	if amount <= 0 {
		return fmt.Errorf("amount must be positive, got: %d", amount)
	}

	depositCoin := sdktypes.NewInt64Coin("uakt", amount)

	// Log what would be attempted
	fmt.Printf("Adding funds functionality: would attempt to deposit %s to deployment %d escrow account\n",
		depositCoin.String(), dseqUint)
	fmt.Printf("Note: Akash typically requires closing and recreating deployments with higher deposits\n")
	fmt.Printf("Consider using UpdateDeployment with new resources or creating a new deployment\n")

	// Return informative error for now
	return fmt.Errorf("direct fund deposit not supported by Akash protocol - consider recreating deployment with higher deposit of %s for deployment %d",
		depositCoin.String(), dseqUint)
}

// UpdateDeployment updates an existing deployment with new SDL content
func (ak *AkashClient) UpdateDeployment(ctx context.Context, dseq string, sdl string) error {
	dseqUint, err := strconv.ParseUint(dseq, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid dseq: %w", err)
	}

	client, err := ak.getNodeClient()
	if err != nil {
		return fmt.Errorf("failed to get node client: %w", err)
	}

	// Generate proper hash from SDL content
	msg := &deploymenttypes.MsgUpdateDeployment{
		ID: dv1.DeploymentID{
			DSeq:  dseqUint,
			Owner: ak.Config.AccountAddress,
		},
		Hash: generateSDLHash(sdl),
	}

	txClient := client.Tx()
	resp, err := txClient.BroadcastMsgs(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to broadcast update deployment transaction: %w", err)
	}

	fmt.Printf("Deployment updated successfully: %+v\n", resp)
	return nil
}
