package client

import (
	"context"
	"fmt"
	"strconv"

	clienttypes "github.com/overlock-network/provider-akash/internal/client/types"
	bidv1 "pkg.akt.dev/go/node/market/v1"
	marketv1beta5 "pkg.akt.dev/go/node/market/v1beta5"
)

// CreateLease broadcasts MsgCreateLease for the given bid (owner from client
// configuration, sequence numbers + provider identify the bid). Returns the
// transaction hash on success.
func (ak *AkashClient) CreateLease(ctx context.Context, seqs clienttypes.Seqs, provider string) (string, error) {
	bidID, err := buildBidID(ak.Config.AccountAddress, seqs, provider)
	if err != nil {
		return "", err
	}

	nodeClient, err := ak.getNodeClient()
	if err != nil {
		return "", fmt.Errorf("failed to get node client: %w", err)
	}

	resp, err := nodeClient.Tx().BroadcastMsgs(ctx, &marketv1beta5.MsgCreateLease{BidID: bidID})
	if err != nil {
		return "", fmt.Errorf("failed to broadcast MsgCreateLease: %w", err)
	}
	if resp.Code != 0 {
		return "", fmt.Errorf("MsgCreateLease rejected by chain: code=%d codespace=%s log=%s", resp.Code, resp.Codespace, resp.RawLog)
	}
	return resp.TxHash, nil
}

// CloseLease broadcasts MsgCloseLease for the lease identified by the given
// sequence numbers + provider. The reason is recorded as Owner-initiated close.
func (ak *AkashClient) CloseLease(ctx context.Context, seqs clienttypes.Seqs, provider string) (string, error) {
	bidID, err := buildBidID(ak.Config.AccountAddress, seqs, provider)
	if err != nil {
		return "", err
	}

	nodeClient, err := ak.getNodeClient()
	if err != nil {
		return "", fmt.Errorf("failed to get node client: %w", err)
	}

	resp, err := nodeClient.Tx().BroadcastMsgs(ctx, &marketv1beta5.MsgCloseLease{
		ID:     bidv1.MakeLeaseID(bidID),
		Reason: bidv1.LeaseClosedReasonOwner,
	})
	if err != nil {
		return "", fmt.Errorf("failed to broadcast MsgCloseLease: %w", err)
	}
	if resp.Code != 0 {
		return "", fmt.Errorf("MsgCloseLease rejected by chain: code=%d codespace=%s log=%s", resp.Code, resp.Codespace, resp.RawLog)
	}
	return resp.TxHash, nil
}

// GetLease queries the chain for a single lease and returns its state string
// ("active" | "insufficient_funds" | "closed" | "reclaiming" | "invalid").
func (ak *AkashClient) GetLease(ctx context.Context, seqs clienttypes.Seqs, provider string) (string, error) {
	bidID, err := buildBidID(ak.Config.AccountAddress, seqs, provider)
	if err != nil {
		return "", err
	}

	nodeClient, err := ak.getNodeClient()
	if err != nil {
		return "", fmt.Errorf("failed to get node client: %w", err)
	}

	resp, err := nodeClient.Query().Market().Lease(ctx, &marketv1beta5.QueryLeaseRequest{
		ID: bidv1.MakeLeaseID(bidID),
	})
	if err != nil {
		return "", fmt.Errorf("failed to query lease: %w", err)
	}
	return resp.Lease.State.String(), nil
}

// GetLeases returns all leases on chain for the given owner+dseq. Used by the
// Deployment controller to derive the deployment phase (e.g., active lease
// implies "leased"; no leases + closed orders implies "expired").
func (ak *AkashClient) GetLeases(ctx context.Context, dseq string, owner string) ([]bidv1.Lease, error) {
	dseqUint, err := strconv.ParseUint(dseq, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid dseq: %w", err)
	}

	nodeClient, err := ak.getNodeClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get node client: %w", err)
	}

	resp, err := nodeClient.Query().Market().Leases(ctx, &marketv1beta5.QueryLeasesRequest{
		Filters: bidv1.LeaseFilters{
			Owner: owner,
			DSeq:  dseqUint,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query leases: %w", err)
	}

	leases := make([]bidv1.Lease, 0, len(resp.Leases))
	for _, l := range resp.Leases {
		leases = append(leases, l.Lease)
	}
	return leases, nil
}

// GetOrders returns all orders on chain for the given owner+dseq. Used by the
// Deployment controller to derive whether a bid window is currently open.
func (ak *AkashClient) GetOrders(ctx context.Context, dseq string, owner string) ([]marketv1beta5.Order, error) {
	dseqUint, err := strconv.ParseUint(dseq, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid dseq: %w", err)
	}

	nodeClient, err := ak.getNodeClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get node client: %w", err)
	}

	resp, err := nodeClient.Query().Market().Orders(ctx, &marketv1beta5.QueryOrdersRequest{
		Filters: marketv1beta5.OrderFilters{
			Owner: owner,
			DSeq:  dseqUint,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query orders: %w", err)
	}

	orders := make([]marketv1beta5.Order, 0, len(resp.Orders))
	orders = append(orders, resp.Orders...)
	return orders, nil
}

// buildBidID converts the legacy (Seqs, provider) parameters used throughout
// the controllers into a chain-sdk market.v1.BidID.
func buildBidID(owner string, seqs clienttypes.Seqs, provider string) (bidv1.BidID, error) {
	dseq, err := strconv.ParseUint(seqs.Dseq, 10, 64)
	if err != nil {
		return bidv1.BidID{}, fmt.Errorf("invalid dseq '%s': %w", seqs.Dseq, err)
	}
	gseq, err := strconv.ParseUint(seqs.Gseq, 10, 32)
	if err != nil {
		return bidv1.BidID{}, fmt.Errorf("invalid gseq '%s': %w", seqs.Gseq, err)
	}
	oseq, err := strconv.ParseUint(seqs.Oseq, 10, 32)
	if err != nil {
		return bidv1.BidID{}, fmt.Errorf("invalid oseq '%s': %w", seqs.Oseq, err)
	}
	return bidv1.BidID{
		Owner:    owner,
		DSeq:     dseq,
		GSeq:     uint32(gseq),
		OSeq:     uint32(oseq),
		Provider: provider,
	}, nil
}
