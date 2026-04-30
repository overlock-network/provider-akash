package client

// Provider-side lease helpers. These talk to the deployed provider's HTTP
// service (not the Akash chain), so they continue to use the akash CLI wrapper
// for now. Migrating them away from the CLI is tracked separately and is not
// required for the on-chain bid/lease/order visibility used to derive the
// Deployment Phase.

import (
	"github.com/overlock-network/provider-akash/internal/client/cli"
	clienttypes "github.com/overlock-network/provider-akash/internal/client/types"
)

func (ak *AkashClient) GetLeaseServices(seqs clienttypes.Seqs, provider string) (string, error) {
	cmd := cli.AkashCli(ak).LeaseStatus().
		SetDseq(seqs.Dseq).SetGseq(seqs.Gseq).SetOseq(seqs.Oseq).
		SetProvider(provider).SetFrom(ak.Config.KeyName).
		SetChainId(ak.Config.ChainId).SetKeyringBackend(ak.Config.KeyringBackend).
		SetNode(ak.Config.Node).OutputJson()

	out, err := cmd.Raw()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (ak *AkashClient) GetServiceStatus(seqs clienttypes.Seqs, provider, serviceName string) (string, error) {
	cmd := cli.AkashCli(ak).ServiceStatus().
		SetDseq(seqs.Dseq).SetGseq(seqs.Gseq).SetOseq(seqs.Oseq).
		SetProvider(provider).SetService(serviceName).SetFrom(ak.Config.KeyName).
		SetChainId(ak.Config.ChainId).SetKeyringBackend(ak.Config.KeyringBackend).
		SetNode(ak.Config.Node).OutputJson()

	out, err := cmd.Raw()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (ak *AkashClient) GetLeaseManifest(seqs clienttypes.Seqs, provider string) (string, error) {
	cmd := cli.AkashCli(ak).GetManifest().
		SetDseq(seqs.Dseq).SetGseq(seqs.Gseq).SetOseq(seqs.Oseq).
		SetProvider(provider).SetFrom(ak.Config.KeyName).
		SetChainId(ak.Config.ChainId).SetKeyringBackend(ak.Config.KeyringBackend).
		SetNode(ak.Config.Node).OutputJson()

	out, err := cmd.Raw()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
