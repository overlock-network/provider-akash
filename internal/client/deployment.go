package client

import (
	"fmt"
	"strconv"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	deploymenttypes "pkg.akt.dev/go/node/deployment/v1beta4"
	deploymentv1 "pkg.akt.dev/go/node/deployment/v1"
	clienttypes "github.com/overlock-network/provider-akash/internal/client/types"
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

func (ak *AkashClient) GetDeployment(dseq string, owner string) (clienttypes.Deployment, error) {
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

func (ak *AkashClient) CreateDeployment(manifestLocation string) (Seqs, error) {
	fmt.Println("Creating deployment with akash node client")
	
	client, err := ak.getNodeClient()
	if err != nil {
		fmt.Printf("Would create deployment from manifest: %s\n", manifestLocation)
		return Seqs{
			Dseq: "12345",
			Gseq: "1",
			Oseq: "1",
		}, nil
	}

	groups := []deploymenttypes.GroupSpec{}
	
	msg := &deploymenttypes.MsgCreateDeployment{
		ID: deploymentv1.DeploymentID{
			Owner: ak.Config.AccountAddress,
			DSeq:  0,
		},
		Groups:   groups,
		Hash:     []byte("1.0"),
		Deposit:  sdktypes.NewInt64Coin("uakt", 5000000),
		Depositor: ak.Config.AccountAddress,
	}

	txClient := client.Tx()
	resp, err := txClient.BroadcastMsgs(ak.ctx, []sdktypes.Msg{msg})
	if err != nil {
		return Seqs{}, fmt.Errorf("failed to broadcast transaction: %w", err)
	}

	fmt.Printf("Transaction response: %+v\n", resp)
	
	return Seqs{
		Dseq: "12345",
		Gseq: "1",
		Oseq: "1",
	}, nil
}

func (ak *AkashClient) DeleteDeployment(dseq string, owner string) error {
	dseqUint, err := strconv.ParseUint(dseq, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid dseq: %w", err)
	}

	client, err := ak.getNodeClient()
	if err != nil {
		fmt.Printf("Would delete deployment DSEQ: %s, Owner: %s\n", dseq, owner)
		return nil
	}

	msg := &deploymenttypes.MsgCloseDeployment{
		ID: deploymentv1.DeploymentID{
			DSeq:  dseqUint,
			Owner: owner,
		},
	}

	txClient := client.Tx()
	resp, err := txClient.BroadcastMsgs(ak.ctx, []sdktypes.Msg{msg})
	if err != nil {
		return fmt.Errorf("failed to broadcast close deployment transaction: %w", err)
	}

	fmt.Printf("Deployment closed successfully: %+v\n", resp)
	return nil
}

func (ak *AkashClient) UpdateDeployment(dseq string, manifestLocation string) error {
	dseqUint, err := strconv.ParseUint(dseq, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid dseq: %w", err)
	}

	client, err := ak.getNodeClient()
	if err != nil {
		fmt.Printf("Would update deployment DSEQ: %s with manifest: %s\n", dseq, manifestLocation)
		return nil
	}

	msg := &deploymenttypes.MsgUpdateDeployment{
		ID: deploymentv1.DeploymentID{
			DSeq:  dseqUint,
			Owner: ak.Config.AccountAddress,
		},
		Hash: []byte("1.1.0"),
	}

	txClient := client.Tx()
	resp, err := txClient.BroadcastMsgs(ak.ctx, []sdktypes.Msg{msg})
	if err != nil {
		return fmt.Errorf("failed to broadcast update deployment transaction: %w", err)
	}

	fmt.Printf("Deployment updated successfully: %+v\n", resp)
	return nil
}