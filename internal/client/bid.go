package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/overlock-network/provider-akash/internal/client/cli"
	clienttypes "github.com/overlock-network/provider-akash/internal/client/types"
)

// GetBids retrieves all bids for a specific deployment
func (ak *AkashClient) GetBids(ctx context.Context, dseq string, owner string) (clienttypes.Bids, error) {
	seqs := clienttypes.Seqs{Dseq: dseq, Gseq: "1", Oseq: "1"}
	
	cmd := cli.AkashCli(ak).Query().Market().Bid().List().
		SetDseq(seqs.Dseq).SetGseq(seqs.Gseq).SetOseq(seqs.Oseq).
		SetOwner(owner).SetChainId(ak.Config.ChainId).SetNode(ak.Config.Node).OutputJson()

	bidsSliceWrapper := clienttypes.BidsSliceWrapper{}
	if err := cmd.DecodeJson(&bidsSliceWrapper); err != nil {
		return nil, fmt.Errorf("failed to decode bids JSON: %w", err)
	}

	bids := clienttypes.Bids{}
	for _, bidWrapper := range bidsSliceWrapper.BidWrappers {
		bids = append(bids, bidWrapper.Bid)
	}

	return bids, nil
}

// GetBid retrieves a specific bid by its bidId (format: owner-dseq-gseq-oseq-provider)
func (ak *AkashClient) GetBid(ctx context.Context, bidId string) (*clienttypes.Bid, error) {
	// Parse bidId format: owner-dseq-gseq-oseq-provider
	parts := strings.Split(bidId, "-")
	if len(parts) != 5 {
		return nil, fmt.Errorf("invalid bidId format, expected owner-dseq-gseq-oseq-provider")
	}
	
	owner := parts[0]
	dseq := parts[1]
	gseq := parts[2]
	oseq := parts[3] 
	provider := parts[4]
	
	// Get all bids for the deployment
	bids, err := ak.GetBids(ctx, dseq, owner)
	if err != nil {
		return nil, fmt.Errorf("failed to get bids: %w", err)
	}

	// Find the specific bid
	for _, bid := range bids {
		if bid.Id.Dseq == dseq && 
		   bid.Id.Gseq == gseq && 
		   bid.Id.Oseq == oseq && 
		   bid.Id.Provider == provider &&
		   bid.Id.Owner == owner {
			return &bid, nil
		}
	}

	return nil, fmt.Errorf("bid not found for bidId %s", bidId)
}

