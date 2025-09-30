package client

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
)

// BroadcastWithMonitoring broadcasts a transaction with enhanced error handling
func (ak *AkashClient) BroadcastWithMonitoring(ctx context.Context, msgs []sdktypes.Msg) (*sdktypes.TxResponse, error) {
	client, err := ak.getNodeClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get node client: %w", err)
	}

	fmt.Println("\n🚀 Broadcasting transaction...")
	
	// Run pre-broadcast validation for each message
	for i, msg := range msgs {
		if valErr := ValidateTransactionPreBroadcast(msg, ak.Config.AccountAddress); valErr != nil {
			fmt.Printf("[ERROR] Pre-broadcast validation failed for message %d: %v\n", i, valErr)
			return nil, fmt.Errorf("pre-broadcast validation failed for message %d: %w", i, valErr)
		}
	}
	
	// Broadcast transaction
	txClient := client.Tx()
	resp, err := txClient.BroadcastMsgs(ctx, msgs...)
	if err != nil {
		debugInfo := DebugBroadcastError(err)
		fmt.Print(debugInfo)
		return nil, fmt.Errorf("broadcast failed: %w", err)
	}

	// Handle the response
	if resp != nil {
		if resp.TxHash != "" {
			fmt.Printf("📤 Transaction broadcasted with hash: %s\n", resp.TxHash)
			
			// Basic status check
			if resp.Code == 0 {
				fmt.Println("✅ Transaction successful!")
				return resp, nil
			} else {
				fmt.Printf("❌ Transaction failed with code %d\n", resp.Code)
				fmt.Printf("   Codespace: %s\n", resp.Codespace)
				fmt.Printf("   Error: %s\n", resp.RawLog)
				debugInfo := DebugBroadcastError(fmt.Errorf("code: %d, log: %s", resp.Code, resp.RawLog))
				fmt.Print(debugInfo)
				return resp, fmt.Errorf("transaction failed: %s", resp.RawLog)
			}
		}
		
		// If no hash, check if it's already processed
		if resp.Code == 0 {
			fmt.Println("✅ Transaction successful (synchronous mode)")
			return resp, nil
		} else {
			fmt.Printf("❌ Transaction failed immediately with code %d\n", resp.Code)
			debugInfo := DebugBroadcastError(fmt.Errorf("code: %d, log: %s", resp.Code, resp.RawLog))
			fmt.Print(debugInfo)
			return resp, fmt.Errorf("transaction failed: %s", resp.RawLog)
		}
	}
	
	// Should not reach here, but return error for completeness
	return nil, fmt.Errorf("unexpected response state")
}

// ExtractTxHashFromResponse attempts to extract transaction hash from broadcast response
func ExtractTxHashFromResponse(resp interface{}) string {
	switch v := resp.(type) {
	case *sdktypes.TxResponse:
		return v.TxHash
	default:
		// Try to extract hash from string representation if possible
		respStr := fmt.Sprintf("%v", resp)
		// Look for hex hash pattern (64 characters)
		if len(respStr) >= 64 {
			// Simple heuristic: look for a 64-char hex string
			for i := 0; i <= len(respStr)-64; i++ {
				candidate := respStr[i : i+64]
				if _, err := hex.DecodeString(candidate); err == nil {
					return candidate
				}
			}
		}
	}
	return ""
}

// WaitForTransactionConfirmation provides a simple wait mechanism
func (ak *AkashClient) WaitForTransactionConfirmation(txHash string, timeoutSeconds int) error {
	if txHash == "" {
		return fmt.Errorf("empty transaction hash")
	}
	
	fmt.Printf("⏳ Waiting for transaction %s to be confirmed...\n", txHash)
	
	// Simple time-based wait since we don't have easy access to tx querying
	// In a production system, you'd want to query the transaction status
	time.Sleep(time.Duration(timeoutSeconds) * time.Second)
	
	fmt.Printf("✅ Wait period completed for transaction %s\n", txHash)
	return nil
}

// LogTransactionDetails logs comprehensive transaction information
func LogTransactionDetails(txResp *sdktypes.TxResponse) {
	if txResp == nil {
		fmt.Println("[INFO] No transaction response to log")
		return
	}
	
	fmt.Println("\n=== TRANSACTION DETAILS ===")
	fmt.Printf("Hash: %s\n", txResp.TxHash)
	fmt.Printf("Height: %d\n", txResp.Height)
	fmt.Printf("Code: %d\n", txResp.Code)
	
	if txResp.Code == 0 {
		fmt.Println("Status: ✅ SUCCESS")
	} else {
		fmt.Printf("Status: ❌ FAILED (code: %d)\n", txResp.Code)
		fmt.Printf("Codespace: %s\n", txResp.Codespace)
		fmt.Printf("Error Log: %s\n", txResp.RawLog)
	}
	
	fmt.Printf("Gas Used: %d\n", txResp.GasUsed)
	fmt.Printf("Gas Wanted: %d\n", txResp.GasWanted)
	
	if len(txResp.Events) > 0 {
		fmt.Printf("Events Count: %d\n", len(txResp.Events))
		for i, event := range txResp.Events {
			fmt.Printf("  Event %d: %s\n", i+1, event.Type)
			for _, attr := range event.Attributes {
				fmt.Printf("    %s: %s\n", attr.Key, attr.Value)
			}
		}
	}
	
	fmt.Println("===========================")
}