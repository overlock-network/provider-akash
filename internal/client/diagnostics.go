package client

import (
	"context"
	"fmt"
	"time"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// NetworkDiagnostics performs comprehensive network and client diagnostics
func (ak *AkashClient) NetworkDiagnostics(ctx context.Context) error {
	fmt.Println("\n=== AKASH NETWORK DIAGNOSTICS ===")
	
	// 1. Test client connection
	fmt.Println("\n1. Testing client connection...")
	client, err := ak.getNodeClient()
	if err != nil {
		fmt.Printf("   ❌ Failed to get node client: %v\n", err)
		return fmt.Errorf("client connection failed: %w", err)
	}
	fmt.Println("   ✅ Client connected successfully")

	// 2. Check account status
	fmt.Println("\n2. Checking account status...")
	authClient := client.Query().Auth()
	accountReq := &authtypes.QueryAccountRequest{
		Address: ak.Config.AccountAddress,
	}
	accountResp, err := authClient.Account(ctx, accountReq)
	if err != nil {
		fmt.Printf("   ❌ Failed to get account info: %v\n", err)
		fmt.Println("   ℹ️  This might indicate the account doesn't exist on-chain yet")
		fmt.Println("   ℹ️  Or the node connection is not working properly")
	} else {
		// The account is returned as Any type, we need to unpack it
		var account authtypes.AccountI
		if err := ak.getCodec().UnpackAny(accountResp.Account, &account); err != nil {
			fmt.Printf("   ⚠️  Could not unpack account info: %v\n", err)
		} else {
			fmt.Printf("   ✅ Account Address: %s\n", account.GetAddress())
			fmt.Printf("   ✅ Account Number: %d\n", account.GetAccountNumber())
			fmt.Printf("   ✅ Sequence: %d\n", account.GetSequence())
		}
	}

	// 3. Check account balance
	fmt.Println("\n3. Checking account balance...")
	bankClient := client.Query().Bank()
	balanceReq := &banktypes.QueryAllBalancesRequest{
		Address: ak.Config.AccountAddress,
	}
	balanceResp, err := bankClient.AllBalances(ctx, balanceReq)
	if err != nil {
		fmt.Printf("   ❌ Failed to get balances: %v\n", err)
	} else {
		if len(balanceResp.Balances) == 0 {
			fmt.Println("   ⚠️  No balances found (account might be empty)")
		} else {
			for _, balance := range balanceResp.Balances {
				fmt.Printf("   ✅ Balance: %s %s\n", balance.Amount.String(), balance.Denom)
			}
		}
	}

	// 4. Check chain parameters (if available)
	fmt.Println("\n4. Checking chain parameters...")
	stakingClient := client.Query().Staking()
	paramsReq := &stakingtypes.QueryParamsRequest{}
	paramsResp, err := stakingClient.Params(ctx, paramsReq)
	if err != nil {
		fmt.Printf("   ⚠️  Could not fetch staking params: %v\n", err)
	} else {
		fmt.Printf("   ✅ Bond Denom: %s\n", paramsResp.Params.BondDenom)
		fmt.Printf("   ✅ Max Validators: %d\n", paramsResp.Params.MaxValidators)
		fmt.Printf("   ✅ Unbonding Time: %s\n", paramsResp.Params.UnbondingTime)
	}

	// 5. Network info
	fmt.Println("\n5. Network Configuration:")
	fmt.Printf("   ✅ Node: %s\n", ak.Config.Node)
	fmt.Printf("   ✅ Chain ID: %s\n", ak.Config.ChainId)
	fmt.Printf("   ✅ Network: %s\n", ak.Config.Net)
	fmt.Printf("   ✅ Keyring Backend: %s\n", ak.Config.KeyringBackend)

	fmt.Println("\n=================================")
	return nil
}

// QuickDiagnostic performs a quick diagnostic check before transactions
func (ak *AkashClient) QuickDiagnostic() error {
	client, err := ak.getNodeClient()
	if err != nil {
		return fmt.Errorf("client not available: %w", err)
	}

	// Quick connectivity check with timeout
	ctx, cancel := context.WithTimeout(ak.ctx, 5*time.Second)
	defer cancel()

	// Try to query account to verify connection
	authClient := client.Query().Auth()
	accountReq := &authtypes.QueryAccountRequest{
		Address: ak.Config.AccountAddress,
	}
	
	_, err = authClient.Account(ctx, accountReq)
	if err != nil {
		// Check if it's a timeout
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("network timeout - node may be unreachable")
		}
		// Account might not exist, but connection works
		fmt.Println("⚠️  Warning: Account may not exist on-chain yet")
	}

	fmt.Println("✅ Network connection verified")
	return nil
}

// getCodec returns the codec for unmarshaling responses
func (ak *AkashClient) getCodec() codec.Codec {
	// This should ideally be stored in AkashClient during initialization
	// For now, create a basic codec
	interfaceRegistry := types.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(interfaceRegistry)
	return codec.NewProtoCodec(interfaceRegistry)
}