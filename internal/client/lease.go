package client

import (
	"github.com/overlock-network/provider-akash/internal/client/cli"
	clienttypes "github.com/overlock-network/provider-akash/internal/client/types"
)

func (ak *AkashClient) CreateLease(seqs clienttypes.Seqs, provider string) (string, error) {
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

func (ak *AkashClient) CloseLease(seqs clienttypes.Seqs, provider string) (string, error) {
	cmd := cli.AkashCli(ak).Tx().Market().Lease().Close().
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

func (ak *AkashClient) GetLease(seqs clienttypes.Seqs, provider string) (string, error) {
	cmd := cli.AkashCli(ak).Query().Market().Lease().Get().
		SetDseq(seqs.Dseq).SetGseq(seqs.Gseq).SetOseq(seqs.Oseq).
		SetProvider(provider).SetOwner(ak.Config.AccountAddress).
		SetChainId(ak.Config.ChainId).SetNode(ak.Config.Node).OutputJson()

	out, err := cmd.Raw()
	if err != nil {
		return "", err
	}

	return string(out), nil
}

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
