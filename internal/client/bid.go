package client

import (
	"fmt"
	"time"

	"github.com/overlock-network/provider-akash/internal/client/cli"
	clienttypes "github.com/overlock-network/provider-akash/internal/client/types"
)

func (ak *AkashClient) GetBids(dseq string) (clienttypes.Seqs, error) {
	bids := clienttypes.Bids{}
	timeout := 30 * time.Second                                // Add missing timeout variable
	seqs := clienttypes.Seqs{Dseq: dseq, Gseq: "1", Oseq: "1"} // Add missing seqs variable
	for timeout > 0 && len(bids) == 0 {
		startTime := time.Now()
		currentBids, err := queryBidList(ak, seqs)
		if err != nil {
			fmt.Print(ak.ctx, "Failed to query bid list")

			return clienttypes.Seqs{}, err // Return empty Seqs instead of nil
		}
		fmt.Printf("Received %d bids", len(bids))
		bids = currentBids
		timeout -= time.Since(startTime)
	}

	return seqs, nil // Return seqs instead of bids
}

func queryBidList(ak *AkashClient, seqs clienttypes.Seqs) (clienttypes.Bids, error) {
	cmd := cli.AkashCli(ak).Query().Market().Bid().List().
		SetDseq(seqs.Dseq).SetGseq(seqs.Gseq).SetOseq(seqs.Oseq).
		SetOwner(ak.Config.AccountAddress).SetChainId(ak.Config.ChainId).SetNode(ak.Config.Node).OutputJson()

	bidsSliceWrapper := clienttypes.BidsSliceWrapper{}
	if err := cmd.DecodeJson(&bidsSliceWrapper); err != nil {
		return nil, err
	}

	bids := clienttypes.Bids{}
	for _, bidWrapper := range bidsSliceWrapper.BidWrappers {
		bids = append(bids, bidWrapper.Bid)
	}

	return bids, nil
}
