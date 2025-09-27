package client

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
)

// DebugBroadcastError provides detailed debugging information for broadcast errors
func DebugBroadcastError(err error) string {
	if err == nil {
		return "No error"
	}

	var debugInfo strings.Builder
	debugInfo.WriteString("\n=== BROADCAST ERROR DEBUG ===\n")
	debugInfo.WriteString(fmt.Sprintf("Error Type: %T\n", err))
	debugInfo.WriteString(fmt.Sprintf("Error Message: %v\n", err))

	// Common error patterns and solutions
	debugInfo.WriteString("\n=== COMMON ERROR PATTERNS ===\n")
	errorStr := err.Error()
	
	switch {
	case strings.Contains(errorStr, "insufficient funds"):
		debugInfo.WriteString("ISSUE: Insufficient funds in account\n")
		debugInfo.WriteString("SOLUTION: Check account balance and ensure sufficient AKT tokens\n")
		
	case strings.Contains(errorStr, "account sequence mismatch"):
		debugInfo.WriteString("ISSUE: Account sequence number mismatch\n")
		debugInfo.WriteString("SOLUTION: Wait a moment and retry, or query current account sequence\n")
		
	case strings.Contains(errorStr, "signature verification failed"):
		debugInfo.WriteString("ISSUE: Invalid signature\n")
		debugInfo.WriteString("SOLUTION: Check keyring configuration and account keys\n")
		
	case strings.Contains(errorStr, "invalid chain-id"):
		debugInfo.WriteString("ISSUE: Wrong chain ID\n")
		debugInfo.WriteString("SOLUTION: Verify network configuration (mainnet vs testnet)\n")
		
	case strings.Contains(errorStr, "gas"):
		debugInfo.WriteString("ISSUE: Gas-related error\n")
		debugInfo.WriteString("SOLUTION: Adjust gas fees or gas limit in transaction\n")
		
	case strings.Contains(errorStr, "timeout"):
		debugInfo.WriteString("ISSUE: Transaction timeout\n")
		debugInfo.WriteString("SOLUTION: Check network connectivity or increase timeout\n")
		
	case strings.Contains(errorStr, "invalid deployment"):
		debugInfo.WriteString("ISSUE: Invalid deployment parameters\n")
		debugInfo.WriteString("SOLUTION: Verify SDL content and deployment configuration\n")
		
	case strings.Contains(errorStr, "already exists"):
		debugInfo.WriteString("ISSUE: Resource already exists\n")
		debugInfo.WriteString("SOLUTION: Use a different DSEQ or close existing deployment\n")
		
	case strings.Contains(errorStr, "not found"):
		debugInfo.WriteString("ISSUE: Resource not found\n")
		debugInfo.WriteString("SOLUTION: Verify the resource ID and ensure it exists\n")
		
	case strings.Contains(errorStr, "invalid msg"):
		debugInfo.WriteString("ISSUE: Invalid message format\n")
		debugInfo.WriteString("SOLUTION: Check message structure and required fields\n")
		
	case strings.Contains(errorStr, "unable to resolve type URL"):
		debugInfo.WriteString("ISSUE: Message type not registered in codec\n")
		debugInfo.WriteString("SOLUTION: Ensure all Akash modules are registered in encoding config\n")
		
	case strings.Contains(errorStr, "tx parse error"):
		debugInfo.WriteString("ISSUE: Transaction parsing failed\n")
		debugInfo.WriteString("SOLUTION: Check message encoding and codec configuration\n")
	}

	// Try to extract and parse raw log if present
	if strings.Contains(errorStr, "raw_log:") {
		parts := strings.Split(errorStr, "raw_log:")
		if len(parts) > 1 {
			rawLog := strings.TrimSpace(parts[1])
			parsedInfo := ParseAkashError(rawLog)
			if len(parsedInfo) > 0 {
				debugInfo.WriteString("\n=== PARSED ERROR DETAILS ===\n")
				for key, value := range parsedInfo {
					debugInfo.WriteString(fmt.Sprintf("%s: %s\n", key, value))
				}
			}
		}
	}

	debugInfo.WriteString("============================\n")
	return debugInfo.String()
}

// ValidateTransactionPreBroadcast performs pre-flight checks before broadcasting
func ValidateTransactionPreBroadcast(msg sdktypes.Msg, accountAddress string) error {
	// Basic nil check
	if msg == nil {
		return fmt.Errorf("message is nil")
	}

	// Verify account address format
	if accountAddress == "" {
		return fmt.Errorf("account address is empty")
	}

	// Check if address is valid bech32
	_, err := sdktypes.AccAddressFromBech32(accountAddress)
	if err != nil {
		return fmt.Errorf("invalid account address format: %w", err)
	}

	// Log message type for debugging
	fmt.Printf("[DEBUG] Message type: %T\n", msg)
	
	// Marshal message to JSON for inspection
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("[WARN] Could not marshal message to JSON: %v\n", err)
	} else {
		fmt.Printf("[DEBUG] Message content: %s\n", string(msgJSON))
	}

	return nil
}

// ParseAkashError extracts meaningful information from Akash-specific errors
func ParseAkashError(rawLog string) map[string]string {
	result := make(map[string]string)
	
	// Common Akash error patterns
	patterns := map[string]string{
		"module":      `module=(\w+)`,
		"code":        `code=(\d+)`,
		"codespace":   `codespace=(\w+)`,
		"msg":         `msg="([^"]+)"`,
		"dseq":        `dseq=(\d+)`,
		"gseq":        `gseq=(\d+)`,
		"oseq":        `oseq=(\d+)`,
		"provider":    `provider=(\w+)`,
		"amount":      `amount=([0-9]+\w+)`,
		"height":      `height=(\d+)`,
		"tx_hash":     `tx_hash=([a-fA-F0-9]+)`,
	}

	for key, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindStringSubmatch(rawLog); len(match) > 1 {
			result[key] = match[1]
		}
	}

	// Extract error message if present
	if strings.Contains(rawLog, "error:") {
		parts := strings.Split(rawLog, "error:")
		if len(parts) > 1 {
			result["error_detail"] = strings.TrimSpace(parts[1])
		}
	}

	// Extract failed message if present
	if strings.Contains(rawLog, "failed") {
		re := regexp.MustCompile(`failed[^:]*:\s*(.+)`)
		if match := re.FindStringSubmatch(rawLog); len(match) > 1 {
			result["failure_reason"] = match[1]
		}
	}

	return result
}

// ExtractTxResponseFromError attempts to extract transaction response details from an error
func ExtractTxResponseFromError(err error) *sdktypes.TxResponse {
	if err == nil {
		return nil
	}

	// Create a basic TxResponse with error information
	txResp := &sdktypes.TxResponse{
		Code:   1, // Non-zero indicates error
		RawLog: err.Error(),
	}

	// Try to extract code from error message
	re := regexp.MustCompile(`code\s*=\s*(\d+)`)
	if match := re.FindStringSubmatch(err.Error()); len(match) > 1 {
		var code uint32
		fmt.Sscanf(match[1], "%d", &code)
		txResp.Code = code
	}

	// Try to extract codespace
	re = regexp.MustCompile(`codespace\s*=\s*(\w+)`)
	if match := re.FindStringSubmatch(err.Error()); len(match) > 1 {
		txResp.Codespace = match[1]
	}

	return txResp
}