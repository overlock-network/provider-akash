package client

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	clienttypes "github.com/overlock-network/provider-akash/internal/client/types"
	bidv1 "pkg.akt.dev/go/node/market/v1"
	marketv1beta5 "pkg.akt.dev/go/node/market/v1beta5"
)

// GetBids retrieves all bids on chain for the given owner+dseq.
// Queries the market module via chain-sdk gRPC.
func (ak *AkashClient) GetBids(ctx context.Context, dseq string, owner string) (clienttypes.Bids, error) {
	dseqUint, err := strconv.ParseUint(dseq, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid dseq: %w", err)
	}

	nodeClient, err := ak.getNodeClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get node client: %w", err)
	}

	resp, err := nodeClient.Query().Market().Bids(ctx, &marketv1beta5.QueryBidsRequest{
		Filters: marketv1beta5.BidFilters{
			Owner: owner,
			DSeq:  dseqUint,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query bids: %w", err)
	}

	bids := make(clienttypes.Bids, 0, len(resp.Bids))
	for _, b := range resp.Bids {
		bids = append(bids, mapChainBidToClient(b.Bid))
	}
	return bids, nil
}

// GetBid retrieves a specific bid by its bidId (format: owner-dseq-gseq-oseq-provider)
func (ak *AkashClient) GetBid(ctx context.Context, bidId string) (*clienttypes.Bid, error) {
	parts := strings.Split(bidId, "-")
	if len(parts) != 5 {
		return nil, fmt.Errorf("invalid bidId format, expected owner-dseq-gseq-oseq-provider")
	}
	owner, dseq, gseq, oseq, provider := parts[0], parts[1], parts[2], parts[3], parts[4]

	bids, err := ak.GetBids(ctx, dseq, owner)
	if err != nil {
		return nil, fmt.Errorf("failed to get bids: %w", err)
	}

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

// mapChainBidToClient converts a chain-sdk market.v1beta5.Bid into the internal
// clienttypes.Bid representation used by the controllers.
func mapChainBidToClient(b marketv1beta5.Bid) clienttypes.Bid {
	id := b.GetID()
	priceAmount, _ := strconv.ParseFloat(b.Price.Amount.String(), 32)
	return clienttypes.Bid{
		Id: clienttypes.BidId{
			Owner:    id.Owner,
			Dseq:     fmt.Sprintf("%d", id.DSeq),
			Gseq:     fmt.Sprintf("%d", id.GSeq),
			Oseq:     fmt.Sprintf("%d", id.OSeq),
			Provider: id.Provider,
		},
		Price: clienttypes.BidPrice{
			Denom:  b.Price.Denom,
			Amount: float32(priceAmount),
		},
		State:     bidStateString(b.State),
		CreatedAt: b.CreatedAt,
	}
}

// bidStateString maps a chain Bid_State enum to the lowercase strings the
// existing controllers and tests expect ("open", "active", "lost", "closed").
func bidStateString(s marketv1beta5.Bid_State) string {
	switch s {
	case marketv1beta5.BidOpen:
		return "open"
	case marketv1beta5.BidActive:
		return "active"
	case marketv1beta5.BidLost:
		return "lost"
	case marketv1beta5.BidClosed:
		return "closed"
	default:
		return "invalid"
	}
}

// ensure bidv1 is referenced to avoid an unused-import warning on builds where
// only the message types from market/v1beta5 are touched. The Bid wraps a
// market/v1.BidID under the hood.
var _ = bidv1.BidID{}
