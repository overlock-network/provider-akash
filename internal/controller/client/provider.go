package client

import (
	"fmt"

	"github.com/overlock-network/provider-akash/internal/controller/client/cli"
)

func (ak *AkashClient) SendManifest(dseq string, provider string, manifestLocation string) (string, error) {

	cmd := cli.AkashCli(ak).SendManifest(manifestLocation).
		SetDseq(dseq).SetProvider(provider).SetHome(ak.Config.Home).
		SetKeyringBackend(ak.Config.KeyringBackend).SetFrom(ak.Config.KeyName).
		SetNode(ak.Config.Node).OutputJson()

	out, err := cmd.Raw()
	if err != nil {
		return "", err
	}

	fmt.Sprintf("Response content: %s", out)

	return string(out), nil
}
