package client

import (
	"fmt"
	"time"

	"github.com/overlock-network/provider-akash/internal/controller/client/cli"
	"github.com/overlock-network/provider-akash/internal/controller/client/types"
)

func (ak *AkashClient) GetBids(seqs Seqs, timeout time.Duration) (types.Bids, error) {
	bids := types.Bids{}
	for timeout > 0 && len(bids) == 0 {
		startTime := time.Now()
		currentBids, err := queryBidList(ak, seqs)
		if err != nil {
			fmt.Print(ak.ctx, "Failed to query bid list")

			return nil, err
		}
		fmt.Printf("Received %d bids", len(bids))
		bids = currentBids
		timeout -= time.Since(startTime)
	}

	return bids, nil
}

func queryBidList(ak *AkashClient, seqs Seqs) (types.Bids, error) {
	cmd := cli.AkashCli(ak).Query().Market().Bid().List().
		SetDseq(seqs.Dseq).SetGseq(seqs.Gseq).SetOseq(seqs.Oseq).
		SetOwner(ak.Config.AccountAddress).SetChainId(ak.Config.ChainId).SetNode(ak.Config.Node).OutputJson()

	bidsSliceWrapper := types.BidsSliceWrapper{}
	if err := cmd.DecodeJson(&bidsSliceWrapper); err != nil {
		return nil, err
	}

	bids := types.Bids{}
	for _, bidWrapper := range bidsSliceWrapper.BidWrappers {
		bids = append(bids, bidWrapper.Bid)
	}

	return bids, nil
}
