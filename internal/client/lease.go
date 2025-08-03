package client

import (
	"github.com/overlock-network/provider-akash/internal/client/cli"
)

func (ak *AkashClient) CreateLease(seqs Seqs, provider string) (string, error) {
	cmd := cli.AkashCli(ak).Tx().Market().Lease().Create().
		SetDseq(seqs.Dseq).SetGseq(seqs.Gseq).SetOseq(seqs.Oseq).
		SetProvider(provider).SetOwner(ak.Config.AccountAddress).SetFrom(ak.Config.KeyName).
		DefaultGas().SetChainId(ak.Config.ChainId).SetKeyringBackend(ak.Config.KeyringBackend).
		SetNote(ak.transactionNote).AutoAccept().SetNode(ak.Config.Node).OutputJson()

	out, err := cmd.Raw()
	if err != nil {
		return "", err
	}

	return string(out), nil
}
