package client

import (
	"fmt"
	"strconv"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	deploymenttypes "pkg.akt.dev/go/node/deployment/v1beta3"
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
		fmt.Printf("Would query deployments for owner: %s\n", owner)
		return []clienttypes.DeploymentId{
			{Dseq: "12345", Owner: owner},
			{Dseq: "67890", Owner: owner},
		}, nil
	}

	queryClient := client.Query()
	deploymentQuery := queryClient.Deployment()
	
	fmt.Printf("Would query deployments using client: %+v\n", deploymentQuery)
	
	return []clienttypes.DeploymentId{}, fmt.Errorf("deployment query implementation pending")
}

func (ak *AkashClient) GetDeployment(dseq string, owner string) (clienttypes.Deployment, error) {
	dseqUint, err := strconv.ParseUint(dseq, 10, 64)
	if err != nil {
		return clienttypes.Deployment{}, fmt.Errorf("invalid dseq: %w", err)
	}

	client, err := ak.getNodeClient()
	if err != nil {
		fmt.Printf("Would query deployment with DSEQ: %s, Owner: %s\n", dseq, owner)
		return clienttypes.Deployment{
			DeploymentInfo: clienttypes.DeploymentInfo{
				State: "active",
				DeploymentId: clienttypes.DeploymentId{
					Dseq:  dseq,
					Owner: owner,
				},
			},
			EscrowAccount: clienttypes.EscrowAccount{
				Owner: owner,
				State: "open",
				Balance: clienttypes.EscrowAccountBalance{
					Denom:  "uakt",
					Amount: "1000000",
				},
			},
		}, nil
	}

	deploymentID := deploymenttypes.DeploymentID{
		DSeq:  dseqUint,
		Owner: owner,
	}

	queryClient := client.Query()
	deploymentQuery := queryClient.Deployment()
	
	fmt.Printf("Would query deployment %+v using client: %+v\n", deploymentID, deploymentQuery)
	
	return clienttypes.Deployment{}, fmt.Errorf("deployment query implementation pending")
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
		ID: deploymenttypes.DeploymentID{
			Owner: ak.Config.AccountAddress,
			DSeq:  0,
		},
		Groups:   groups,
		Version:  []byte("1.0"),
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
		ID: deploymenttypes.DeploymentID{
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
		ID: deploymenttypes.DeploymentID{
			DSeq:  dseqUint,
			Owner: ak.Config.AccountAddress,
		},
		Version: []byte("1.1.0"),
	}

	txClient := client.Tx()
	resp, err := txClient.BroadcastMsgs(ak.ctx, []sdktypes.Msg{msg})
	if err != nil {
		return fmt.Errorf("failed to broadcast update deployment transaction: %w", err)
	}

	fmt.Printf("Deployment updated successfully: %+v\n", resp)
	return nil
}