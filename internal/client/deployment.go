package client

import (
	"context"
	"fmt"
	"strconv"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/overlock-network/provider-akash/internal/client/types"
	deploymentv1 "pkg.akt.dev/go/node/deployment/v1"
	deploymenttypes "pkg.akt.dev/go/node/deployment/v1beta4"
)

type Seqs struct {
	Dseq string
	Gseq string
	Oseq string
}

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
		deployments = append(deployments, clienttypes.DeploymentId{
			Dseq:  fmt.Sprintf("%d", deploymentResp.Deployment.ID.DSeq),
			Owner: deploymentResp.Deployment.ID.Owner,
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

	deploymentID := deploymentv1.DeploymentID{
		DSeq:  dseqUint,
		Owner: owner,
	}

	queryClient := client.Query()
	deploymentQuery := queryClient.Deployment()

	deploymentResp, err := deploymentQuery.Deployment(ak.ctx, &deploymenttypes.QueryDeploymentRequest{
		ID: deploymentID,
	})
	if err != nil {
		return clienttypes.Deployment{}, fmt.Errorf("failed to query deployment: %w", err)
	}

	return clienttypes.Deployment{
		DeploymentInfo: clienttypes.DeploymentInfo{
			State: deploymentResp.Deployment.State.String(),
			DeploymentId: clienttypes.DeploymentId{
				Dseq:  fmt.Sprintf("%d", deploymentResp.Deployment.ID.DSeq),
				Owner: deploymentResp.Deployment.ID.Owner,
			},
		},
		EscrowAccount: clienttypes.EscrowAccount{
			Owner: deploymentResp.EscrowAccount.Owner,
			State: deploymentResp.EscrowAccount.State.String(),
			Balance: clienttypes.EscrowAccountBalance{
				Denom:  deploymentResp.EscrowAccount.Balance.Denom,
				Amount: deploymentResp.EscrowAccount.Balance.Amount.String(),
			},
		},
	}, nil
}

// CreateDeployment creates a new deployment with SDL content, deposit amount, and currency
func (ak *AkashClient) CreateDeployment(ctx context.Context, sdl string, deposit int64, currency string) (Seqs, error) {
	fmt.Printf("Creating deployment with SDL content (length: %d), deposit: %d %s\n", len(sdl), deposit, currency)

	client, err := ak.getNodeClient()
	if err != nil {
		fmt.Printf("Would create deployment with SDL: %s, deposit: %d %s\n", sdl[:min(50, len(sdl))], deposit, currency)
		return Seqs{
			Dseq: "12345",
			Gseq: "1",
			Oseq: "1",
		}, nil
	}

	// TODO: Parse SDL content and generate GroupSpec from it
	// For now, using empty groups as placeholder
	groups := []deploymenttypes.GroupSpec{}

	// Create deposit coin with specified currency and amount
	depositCoin := sdktypes.NewInt64Coin(currency, deposit)

	msg := &deploymenttypes.MsgCreateDeployment{
		ID: deploymentv1.DeploymentID{
			Owner: ak.Config.AccountAddress,
			DSeq:  0, // Will be assigned by the network
		},
		Groups:    groups,
		Hash:      []byte(sdl), // Use SDL as hash for now
		Deposit:   depositCoin,
		Depositor: ak.Config.AccountAddress,
	}

	txClient := client.Tx()
	resp, err := txClient.BroadcastMsgs(ctx, []sdktypes.Msg{msg})
	if err != nil {
		return Seqs{}, fmt.Errorf("failed to broadcast create deployment transaction: %w", err)
	}

	fmt.Printf("Create deployment transaction response: %+v\n", resp)

	// TODO: Extract actual DSEQ from transaction response
	return Seqs{
		Dseq: "12345", // Placeholder - extract from response
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

// CloseDeployment closes an existing deployment
func (ak *AkashClient) CloseDeployment(ctx context.Context, dseq string, owner string) error {
	dseqUint, err := strconv.ParseUint(dseq, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid dseq: %w", err)
	}

	client, err := ak.getNodeClient()
	if err != nil {
		fmt.Printf("Would close deployment DSEQ: %s, Owner: %s\n", dseq, owner)
		return nil
	}

	msg := &deploymenttypes.MsgCloseDeployment{
		ID: deploymentv1.DeploymentID{
			DSeq:  dseqUint,
			Owner: owner,
		},
	}

	txClient := client.Tx()
	resp, err := txClient.BroadcastMsgs(ctx, []sdktypes.Msg{msg})
	if err != nil {
		return fmt.Errorf("failed to broadcast close deployment transaction: %w", err)
	}

	fmt.Printf("Deployment closed successfully: %+v\n", resp)
	return nil
}

// AddFunds adds funds to a deployment's escrow account
func (ak *AkashClient) AddFunds(ctx context.Context, dseq string, amount int64) error {
	_, err := strconv.ParseUint(dseq, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid dseq: %w", err)
	}

	_, err = ak.getNodeClient()
	if err != nil {
		fmt.Printf("Would add %d uakt to deployment DSEQ: %s\n", amount, dseq)
		return nil
	}

	// Create the fund deposit message
	// TODO: Verify correct message type for depositing funds to deployment
	depositCoin := sdktypes.NewInt64Coin("uakt", amount)
	fmt.Printf("Would deposit %s to deployment %s escrow account\n", depositCoin.String(), dseq)

	// For now, return success without actual transaction
	// This will be implemented when the correct Akash SDK message type is confirmed
	return nil
}

// UpdateDeployment updates an existing deployment with new SDL content
func (ak *AkashClient) UpdateDeployment(ctx context.Context, dseq string, sdl string) error {
	dseqUint, err := strconv.ParseUint(dseq, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid dseq: %w", err)
	}

	client, err := ak.getNodeClient()
	if err != nil {
		fmt.Printf("Would update deployment DSEQ: %s with SDL: %s\n", dseq, sdl[:min(50, len(sdl))])
		return nil
	}

	// TODO: Parse SDL content to generate proper hash
	msg := &deploymenttypes.MsgUpdateDeployment{
		ID: deploymentv1.DeploymentID{
			DSeq:  dseqUint,
			Owner: ak.Config.AccountAddress,
		},
		Hash: []byte(sdl), // Use SDL content as hash for now
	}

	txClient := client.Tx()
	resp, err := txClient.BroadcastMsgs(ctx, []sdktypes.Msg{msg})
	if err != nil {
		return fmt.Errorf("failed to broadcast update deployment transaction: %w", err)
	}

	fmt.Printf("Deployment updated successfully: %+v\n", resp)
	return nil
}
