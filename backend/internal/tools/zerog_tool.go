package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const zerogChainscanAPI = "https://chainscan.0g.ai/open/api"

// chainscanResponse mirrors the Etherscan-compatible JSON envelope.
type chainscanResponse struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Result  json.RawMessage `json:"result"`
}

// tokenTransfer represents a single ERC-20 transfer from the Chainscan API.
type tokenTransfer struct {
	BlockNumber     string `json:"blockNumber"`
	Hash            string `json:"hash"`
	Timestamp       string `json:"timestamp"`
	From            string `json:"from"`
	To              string `json:"to"`
	Value           string `json:"value"`
	ContractAddress string `json:"contractAddress"`
	TokenName       string `json:"tokenName"`
	TokenSymbol     string `json:"tokenSymbol"`
	TokenDecimal    string `json:"tokenDecimal"`
}

// validateAddress checks that an address looks like a valid 0x hex address.
func validateAddress(addr string) error {
	if !strings.HasPrefix(addr, "0x") && !strings.HasPrefix(addr, "0X") {
		return fmt.Errorf("address must start with 0x, got: %s", addr)
	}
	if len(addr) != 42 {
		return fmt.Errorf("address must be 42 characters (0x + 40 hex), got %d characters", len(addr))
	}
	return nil
}

// ──────────────────────────────────────────────────────────────
// Tool 1: get_0g_token_transfers
// ──────────────────────────────────────────────────────────────

// NewZeroGTokenTransfersTool creates the get_0g_token_transfers tool.
func NewZeroGTokenTransfersTool() *Tool {
	return &Tool{
		Name:        "get_0g_token_transfers",
		DisplayName: "0G Token Transfers",
		Description: "Fetch ERC-20 token transfer history for a wallet or contract on the 0G network (Chain ID: 16661). Returns token name, symbol, decimals, value, from/to addresses, and tx hash. Use get_0g_native_transactions for native $0G transfers.",
		Icon:        "ArrowLeftRight",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"wallet_address": map[string]interface{}{
					"type":        "string",
					"description": "The 0x address of the wallet or contract to query transfers for",
				},
				"token_address": map[string]interface{}{
					"type":        "string",
					"description": "Optional: filter by a specific ERC-20 token contract address",
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number for pagination (default: 1)",
					"default":     1,
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Number of results per page (default: 50, max: 100)",
					"default":     50,
				},
			},
			"required": []string{"wallet_address"},
		},
		Execute:  executeGetZeroGTokenTransfers,
		Source:   ToolSourceBuiltin,
		Category: "blockchain",
		Keywords: []string{"0g", "blockchain", "token", "transfer", "web3", "crypto", "wallet", "erc20", "transaction", "on-chain"},
	}
}

func executeGetZeroGTokenTransfers(args map[string]interface{}) (string, error) {
	walletAddress, ok := args["wallet_address"].(string)
	if !ok || walletAddress == "" {
		return "", fmt.Errorf("wallet_address parameter is required")
	}
	walletAddress = strings.TrimSpace(walletAddress)
	if err := validateAddress(walletAddress); err != nil {
		return "", fmt.Errorf("invalid wallet_address: %w", err)
	}

	log.Printf("🔗 [0G-TRANSFERS] Fetching token transfers for: %s", walletAddress)

	// Build query parameters
	params := url.Values{}
	params.Set("module", "account")
	params.Set("action", "tokentx")
	params.Set("address", walletAddress)
	params.Set("sort", "desc")

	if tokenAddr, ok := args["token_address"].(string); ok && tokenAddr != "" {
		tokenAddr = strings.TrimSpace(tokenAddr)
		if err := validateAddress(tokenAddr); err != nil {
			return "", fmt.Errorf("invalid token_address: %w", err)
		}
		params.Set("contractaddress", tokenAddr)
	}

	page := 1
	if p, ok := args["page"].(float64); ok && p > 0 {
		page = int(p)
	}
	params.Set("page", fmt.Sprintf("%d", page))

	limit := 50
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
		if limit > 100 {
			limit = 100
		}
	}
	params.Set("offset", fmt.Sprintf("%d", limit))

	requestURL := fmt.Sprintf("%s?%s", zerogChainscanAPI, params.Encode())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(requestURL)
	if err != nil {
		return "", fmt.Errorf("failed to reach 0G Chainscan API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("0G Chainscan API returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp chainscanResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse API response: %w", err)
	}

	if apiResp.Status != "1" {
		return fmt.Sprintf("No token transfers found for %s. API message: %s", walletAddress, apiResp.Message), nil
	}

	var transfers []tokenTransfer
	if err := json.Unmarshal(apiResp.Result, &transfers); err != nil {
		return "", fmt.Errorf("failed to parse transfer data: %w", err)
	}

	if len(transfers) == 0 {
		return fmt.Sprintf("No token transfers found for address %s on the 0G network.", walletAddress), nil
	}

	// Format results
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d token transfer(s) for %s on 0G Network (page %d):\n\n", len(transfers), walletAddress, page))

	for i, tx := range transfers {
		sb.WriteString(fmt.Sprintf("[%d] %s (%s)\n", i+1, tx.TokenName, tx.TokenSymbol))
		sb.WriteString(fmt.Sprintf("    Hash:      %s\n", tx.Hash))
		sb.WriteString(fmt.Sprintf("    From:      %s\n", tx.From))
		sb.WriteString(fmt.Sprintf("    To:        %s\n", tx.To))
		sb.WriteString(fmt.Sprintf("    Value:     %s (raw, %s decimals)\n", tx.Value, tx.TokenDecimal))
		sb.WriteString(fmt.Sprintf("    Block:     %s\n", tx.BlockNumber))
		sb.WriteString(fmt.Sprintf("    Timestamp: %s\n", tx.Timestamp))
		sb.WriteString("\n")
	}

	log.Printf("✅ [0G-TRANSFERS] Found %d transfers for %s", len(transfers), walletAddress)
	return sb.String(), nil
}

// ──────────────────────────────────────────────────────────────
// Tool 2: get_0g_contract_abi
// ──────────────────────────────────────────────────────────────

// NewZeroGContractABITool creates the get_0g_contract_abi tool.
func NewZeroGContractABITool() *Tool {
	return &Tool{
		Name:        "get_0g_contract_abi",
		DisplayName: "0G Contract ABI",
		Description: "Retrieve the verified ABI of a smart contract on the 0G network (Chain ID: 16661). Returns the full JSON ABI with function signatures. Use get_0g_native_transactions with the contract address to see its transaction history.",
		Icon:        "FileCode",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"contract_address": map[string]interface{}{
					"type":        "string",
					"description": "The 0x address of the deployed smart contract",
				},
			},
			"required": []string{"contract_address"},
		},
		Execute:  executeGetZeroGContractABI,
		Source:   ToolSourceBuiltin,
		Category: "blockchain",
		Keywords: []string{"0g", "blockchain", "contract", "abi", "web3", "smart contract", "solidity", "verified", "interface"},
	}
}

func executeGetZeroGContractABI(args map[string]interface{}) (string, error) {
	contractAddress, ok := args["contract_address"].(string)
	if !ok || contractAddress == "" {
		return "", fmt.Errorf("contract_address parameter is required")
	}
	contractAddress = strings.TrimSpace(contractAddress)
	if err := validateAddress(contractAddress); err != nil {
		return "", fmt.Errorf("invalid contract_address: %w", err)
	}

	log.Printf("🔗 [0G-ABI] Fetching contract ABI for: %s", contractAddress)

	params := url.Values{}
	params.Set("module", "contract")
	params.Set("action", "getabi")
	params.Set("address", contractAddress)

	requestURL := fmt.Sprintf("%s?%s", zerogChainscanAPI, params.Encode())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(requestURL)
	if err != nil {
		return "", fmt.Errorf("failed to reach 0G Chainscan API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("0G Chainscan API returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp chainscanResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse API response: %w", err)
	}

	if apiResp.Status != "1" {
		// The API returns a human-readable message like "Contract source code not verified"
		var resultMsg string
		if err := json.Unmarshal(apiResp.Result, &resultMsg); err != nil {
			resultMsg = string(apiResp.Result)
		}
		return fmt.Sprintf("Could not retrieve ABI for contract %s: %s", contractAddress, resultMsg), nil
	}

	// Result is the ABI JSON string — try to pretty-print it
	var abiRaw string
	if err := json.Unmarshal(apiResp.Result, &abiRaw); err != nil {
		return "", fmt.Errorf("failed to parse ABI string: %w", err)
	}

	// Try to pretty-format the ABI for readability
	var abiParsed interface{}
	if err := json.Unmarshal([]byte(abiRaw), &abiParsed); err == nil {
		prettyABI, err := json.MarshalIndent(abiParsed, "", "  ")
		if err == nil {
			abiRaw = string(prettyABI)
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Contract ABI for %s on 0G Network:\n\n", contractAddress))
	sb.WriteString(abiRaw)

	log.Printf("✅ [0G-ABI] Successfully retrieved ABI for %s", contractAddress)
	return sb.String(), nil
}

// ──────────────────────────────────────────────────────────────
// Tool 3: get_0g_native_transactions
// ──────────────────────────────────────────────────────────────

// nativeTransaction represents a standard transaction from the Chainscan txlist API.
type nativeTransaction struct {
	BlockNumber string `json:"blockNumber"`
	Hash        string `json:"hash"`
	TimeStamp   string `json:"timeStamp"`
	From        string `json:"from"`
	To          string `json:"to"`
	Value       string `json:"value"`
	Gas         string `json:"gas"`
	GasPrice    string `json:"gasPrice"`
	GasUsed     string `json:"gasUsed"`
	Input       string `json:"input"`
	IsError     string `json:"isError"`
}

// NewZeroGNativeTransactionsTool creates the get_0g_native_transactions tool.
func NewZeroGNativeTransactionsTool() *Tool {
	return &Tool{
		Name:        "get_0g_native_transactions",
		DisplayName: "0G Native Transactions",
		Description: "Fetch native transaction history for any address (wallet or contract) on the 0G network (Chain ID: 16661). Returns native $0G transfers, contract calls with method selectors, gas used, and success/failure status. Also works for contract addresses to see all interactions with that contract.",
		Icon:        "Activity",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"wallet_address": map[string]interface{}{
					"type":        "string",
					"description": "The 0x address of the wallet to query",
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number for pagination (default: 1)",
					"default":     1,
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Number of results per page (default: 25, max: 100)",
					"default":     25,
				},
			},
			"required": []string{"wallet_address"},
		},
		Execute:  executeGetZeroGNativeTransactions,
		Source:   ToolSourceBuiltin,
		Category: "blockchain",
		Keywords: []string{"0g", "blockchain", "transaction", "native", "transfer", "web3", "crypto", "wallet", "history", "gas", "0g coin"},
	}
}

func executeGetZeroGNativeTransactions(args map[string]interface{}) (string, error) {
	walletAddress, ok := args["wallet_address"].(string)
	if !ok || walletAddress == "" {
		return "", fmt.Errorf("wallet_address parameter is required")
	}
	walletAddress = strings.TrimSpace(walletAddress)
	if err := validateAddress(walletAddress); err != nil {
		return "", fmt.Errorf("invalid wallet_address: %w", err)
	}

	log.Printf("🔗 [0G-NATIVE-TX] Fetching native transactions for: %s", walletAddress)

	params := url.Values{}
	params.Set("module", "account")
	params.Set("action", "txlist")
	params.Set("address", walletAddress)
	params.Set("startblock", "0")
	params.Set("endblock", "99999999")
	params.Set("sort", "desc")

	page := 1
	if p, ok := args["page"].(float64); ok && p > 0 {
		page = int(p)
	}
	params.Set("page", fmt.Sprintf("%d", page))

	limit := 25
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
		if limit > 100 {
			limit = 100
		}
	}
	params.Set("offset", fmt.Sprintf("%d", limit))

	requestURL := fmt.Sprintf("%s?%s", zerogChainscanAPI, params.Encode())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(requestURL)
	if err != nil {
		return "", fmt.Errorf("failed to reach 0G Chainscan API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("0G Chainscan API returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp chainscanResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse API response: %w", err)
	}

	if apiResp.Status != "1" {
		return fmt.Sprintf("No transactions found for %s. API message: %s", walletAddress, apiResp.Message), nil
	}

	var txs []nativeTransaction
	if err := json.Unmarshal(apiResp.Result, &txs); err != nil {
		return "", fmt.Errorf("failed to parse transaction data: %w", err)
	}

	if len(txs) == 0 {
		return fmt.Sprintf("No native transactions found for address %s on the 0G network.", walletAddress), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d native transaction(s) for %s on 0G Network (page %d):\n\n", len(txs), walletAddress, page))

	for i, tx := range txs {
		// Determine transaction type from input data
		txType := "Native Transfer"
		if tx.Input != "" && tx.Input != "0x" {
			txType = "Contract Call"
			if len(tx.Input) >= 10 {
				txType = fmt.Sprintf("Contract Call (method: %s)", tx.Input[:10])
			}
		}

		status := "Success"
		if tx.IsError == "1" {
			status = "Failed"
		}

		sb.WriteString(fmt.Sprintf("[%d] %s — %s\n", i+1, txType, status))
		sb.WriteString(fmt.Sprintf("    Hash:      %s\n", tx.Hash))
		sb.WriteString(fmt.Sprintf("    From:      %s\n", tx.From))
		sb.WriteString(fmt.Sprintf("    To:        %s\n", tx.To))
		sb.WriteString(fmt.Sprintf("    Value:     %s (wei)\n", tx.Value))
		sb.WriteString(fmt.Sprintf("    Gas Used:  %s (price: %s)\n", tx.GasUsed, tx.GasPrice))
		sb.WriteString(fmt.Sprintf("    Block:     %s\n", tx.BlockNumber))
		sb.WriteString(fmt.Sprintf("    Timestamp: %s\n", tx.TimeStamp))
		sb.WriteString("\n")
	}

	log.Printf("✅ [0G-NATIVE-TX] Found %d transactions for %s", len(txs), walletAddress)
	return sb.String(), nil
}

// ──────────────────────────────────────────────────────────────
// Tool 4: get_0g_network_stats
// ──────────────────────────────────────────────────────────────

const zerogChainscanStats = "https://chainscan.0g.ai/open/statistics"

// statResponse wraps the inner result from the statistics API (inside the Etherscan envelope).
type statResponse struct {
	Total        int             `json:"total"`
	List         json.RawMessage `json:"list"`
	IntervalType string          `json:"intervalType,omitempty"`
}

// NewZeroGNetworkStatsTool creates the get_0g_network_stats tool.
func NewZeroGNetworkStatsTool() *Tool {
	return &Tool{
		Name:        "get_0g_network_stats",
		DisplayName: "0G Network Stats",
		Description: "Fetch macro-level network analytics for the 0G blockchain. Returns 30 entries per page (sorted newest first). Use 'page' param to get older data: page 1 = most recent 30 days, page 2 = days 31-60, page 3 = days 61-90, etc. Supports 22 metrics: tps, transactions, active_accounts, account_growth, contracts, gas_used, base_fee, priority_fee, top_gas_users, token_transfers, token_holders, token_unique_senders, token_unique_receivers, token_unique_participants, top_miners, top_tx_senders, top_tx_receivers, top_token_transfers, top_token_senders, top_token_receivers, top_token_participants, txs_by_type.",
		Icon:        "BarChart3",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"metric": map[string]interface{}{
					"type":        "string",
					"description": "The metric to query",
					"enum":        []string{"tps", "transactions", "active_accounts", "account_growth", "contracts", "gas_used", "base_fee", "priority_fee", "top_gas_users", "token_transfers", "token_holders", "token_unique_senders", "token_unique_receivers", "token_unique_participants", "top_miners", "top_tx_senders", "top_tx_receivers", "top_token_transfers", "top_token_senders", "top_token_receivers", "top_token_participants", "txs_by_type"},
				},
				"days": map[string]interface{}{
					"type":        "integer",
					"description": "Total number of days of data to fetch from the API (default: 90, max: 365). The results are then paginated at 30 entries per page.",
					"default":     90,
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number (default: 1). Each page has 30 entries. Page 1 = newest 30 entries, page 2 = next 30, etc.",
					"default":     1,
				},
			},
			"required": []string{"metric"},
		},
		Execute:  executeGetZeroGNetworkStats,
		Source:   ToolSourceBuiltin,
		Category: "blockchain",
		Keywords: []string{"0g", "blockchain", "network", "statistics", "tps", "transactions", "active wallets", "gas", "analytics", "dashboard", "growth", "contracts"},
	}
}

func executeGetZeroGNetworkStats(args map[string]interface{}) (string, error) {
	metric, ok := args["metric"].(string)
	if !ok || metric == "" {
		return "", fmt.Errorf("metric parameter is required (one of: tps, transactions, active_accounts, account_growth, contracts, gas_used, base_fee, priority_fee, top_gas_users)")
	}

	days := 90
	if d, ok := args["days"].(float64); ok && d > 0 {
		days = int(d)
		if days > 365 {
			days = 365
		}
	}

	page := 1
	if p, ok := args["page"].(float64); ok && p > 0 {
		page = int(p)
	}

	const pageSize = 30

	// Calculate timestamp range
	now := time.Now()
	minTimestamp := now.AddDate(0, 0, -days).Unix()
	maxTimestamp := now.Unix()

	// Map metric name to API endpoint path and display name
	var endpoint, displayName string
	switch metric {
	case "tps":
		endpoint = "/tps"
		displayName = "Transactions Per Second"
	case "transactions":
		endpoint = "/transaction"
		displayName = "Daily Transaction Count"
	case "active_accounts":
		endpoint = "/account/active"
		displayName = "Daily Active Accounts"
	case "account_growth":
		endpoint = "/account/growth"
		displayName = "Account Growth"
	case "contracts":
		endpoint = "/contract"
		displayName = "Contract Deployments"
	case "gas_used":
		endpoint = "/block/gas-used"
		displayName = "Gas Used"
	case "base_fee":
		endpoint = "/block/base-fee"
		displayName = "Base Fee Per Gas"
	case "priority_fee":
		endpoint = "/block/avg-priority-fee"
		displayName = "Average Priority Fee"
	case "top_gas_users":
		endpoint = "/top/gas/used"
		displayName = "Top Gas Consumers"
	case "token_transfers":
		endpoint = "/token/transfer"
		displayName = "Token Transfer Count"
	case "token_holders":
		endpoint = "/token/holder"
		displayName = "Token Holder Count"
	case "token_unique_senders":
		endpoint = "/token/unique/sender"
		displayName = "Unique Token Senders"
	case "token_unique_receivers":
		endpoint = "/token/unique/receiver"
		displayName = "Unique Token Receivers"
	case "token_unique_participants":
		endpoint = "/token/unique/participant"
		displayName = "Unique Token Participants"
	case "top_miners":
		endpoint = "/top/miner"
		displayName = "Top Miners"
	case "top_tx_senders":
		endpoint = "/top/transaction/sender"
		displayName = "Top Transaction Senders"
	case "top_tx_receivers":
		endpoint = "/top/transaction/receiver"
		displayName = "Top Transaction Receivers"
	case "top_token_transfers":
		endpoint = "/top/token/transfer"
		displayName = "Top Token Transfers"
	case "top_token_senders":
		endpoint = "/top/token/sender"
		displayName = "Top Token Senders"
	case "top_token_receivers":
		endpoint = "/top/token/receiver"
		displayName = "Top Token Receivers"
	case "top_token_participants":
		endpoint = "/top/token/participant"
		displayName = "Top Token Participants"
	case "txs_by_type":
		endpoint = "/block/txs-by-type"
		displayName = "Transactions by Type"
	default:
		return "", fmt.Errorf("unknown metric '%s'. Valid: tps, transactions, active_accounts, account_growth, contracts, gas_used, base_fee, priority_fee, top_gas_users, token_transfers, token_holders, token_unique_senders, token_unique_receivers, token_unique_participants, top_miners, top_tx_senders, top_tx_receivers, top_token_transfers, top_token_senders, top_token_receivers, top_token_participants, txs_by_type", metric)
	}

	log.Printf("🔗 [0G-STATS] Fetching %s for last %d days", displayName, days)

	params := url.Values{}
	params.Set("minTimestamp", fmt.Sprintf("%d", minTimestamp))
	params.Set("maxTimestamp", fmt.Sprintf("%d", maxTimestamp))
	params.Set("sort", "DESC")
	params.Set("limit", "500")

	if metric == "tps" {
		params.Set("intervalType", "day")
	}

	requestURL := fmt.Sprintf("%s%s?%s", zerogChainscanStats, endpoint, params.Encode())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(requestURL)
	if err != nil {
		return "", fmt.Errorf("failed to reach 0G Chainscan statistics API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("0G Chainscan statistics API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Statistics endpoints use the same Etherscan envelope: {status, message, result}
	var apiResp chainscanResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse statistics API response: %w", err)
	}

	if apiResp.Status != "1" {
		return fmt.Sprintf("No %s data available. API message: %s", displayName, apiResp.Message), nil
	}

	// The result field contains {total, list, intervalType}
	var statsResp statResponse
	if err := json.Unmarshal(apiResp.Result, &statsResp); err != nil {
		// Some endpoints may return a different result format — return raw
		var prettyJSON interface{}
		if err2 := json.Unmarshal(apiResp.Result, &prettyJSON); err2 == nil {
			formatted, _ := json.MarshalIndent(prettyJSON, "", "  ")
			return fmt.Sprintf("0G Network — %s (last %d days):\n\n%s", displayName, days, string(formatted)), nil
		}
		return "", fmt.Errorf("failed to parse statistics result: %w", err)
	}

	// Parse the list entries
	var entries []map[string]interface{}
	if err := json.Unmarshal(statsResp.List, &entries); err != nil {
		// Return raw list if parsing fails
		return fmt.Sprintf("0G Network — %s (last %d days, %d records):\n\n%s", displayName, days, statsResp.Total, string(statsResp.List)), nil
	}

	if len(entries) == 0 {
		return fmt.Sprintf("No %s data available for the 0G network in the last %d days. The 0G network data may not go back this far.", displayName, days), nil
	}

	// Determine the full date range of all data
	var fullNewest, fullOldest string
	if t, ok := entries[0]["statTime"]; ok {
		fullNewest = fmt.Sprintf("%v", t)
	}
	if t, ok := entries[len(entries)-1]["statTime"]; ok {
		fullOldest = fmt.Sprintf("%v", t)
	}

	// Paginate: slice entries into pages of pageSize
	totalEntries := len(entries)
	totalPages := (totalEntries + pageSize - 1) / pageSize
	if page > totalPages {
		page = totalPages
	}

	startIdx := (page - 1) * pageSize
	endIdx := startIdx + pageSize
	if endIdx > totalEntries {
		endIdx = totalEntries
	}
	pageEntries := entries[startIdx:endIdx]

	// Date range for this page
	var pageNewest, pageOldest string
	if t, ok := pageEntries[0]["statTime"]; ok {
		pageNewest = fmt.Sprintf("%v", t)
	}
	if t, ok := pageEntries[len(pageEntries)-1]["statTime"]; ok {
		pageOldest = fmt.Sprintf("%v", t)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("0G Network — %s (page %d of %d, %d entries):\n", displayName, page, totalPages, len(pageEntries)))
	sb.WriteString(fmt.Sprintf("This page: %s to %s\n", pageOldest, pageNewest))
	if fullOldest != "" && fullNewest != "" {
		sb.WriteString(fmt.Sprintf("Full data range: %s to %s (%d total entries)\n", fullOldest, fullNewest, totalEntries))
	}
	if page < totalPages {
		sb.WriteString(fmt.Sprintf("To see older data, call again with page=%d\n", page+1))
	}
	sb.WriteString("NOTE: If the date you're looking for is NOT in the full data range, the data is not available.\n")
	sb.WriteString("\n")

	for i, entry := range pageEntries {
		sb.WriteString(fmt.Sprintf("[%d] ", startIdx+i+1))
		if t, ok := entry["statTime"]; ok {
			sb.WriteString(fmt.Sprintf("Date: %v", t))
		}
		for key, val := range entry {
			if key == "statTime" {
				continue
			}
			sb.WriteString(fmt.Sprintf("  |  %s: %v", key, val))
		}
		sb.WriteString("\n")
	}

	log.Printf("✅ [0G-STATS] Retrieved page %d/%d (%d entries) for %s", page, totalPages, len(pageEntries), displayName)
	return sb.String(), nil
}

// ──────────────────────────────────────────────────────────────
// Tool 5: get_0g_balance
// ──────────────────────────────────────────────────────────────

// NewZeroGBalanceTool creates the get_0g_balance tool.
func NewZeroGBalanceTool() *Tool {
	return &Tool{
		Name:        "get_0g_balance",
		DisplayName: "0G Balance",
		Description: "Get the native $0G (A0GI) balance for one or more addresses on the 0G network (Chain ID: 16661). Supports querying up to 20 addresses at once. Returns balance in wei.",
		Icon:        "Wallet",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"address": map[string]interface{}{
					"type":        "string",
					"description": "A single 0x address or comma-separated list of up to 20 addresses",
				},
			},
			"required": []string{"address"},
		},
		Execute:  executeGetZeroGBalance,
		Source:   ToolSourceBuiltin,
		Category: "blockchain",
		Keywords: []string{"0g", "blockchain", "balance", "wallet", "web3", "crypto", "native", "a0gi"},
	}
}

func executeGetZeroGBalance(args map[string]interface{}) (string, error) {
	addressInput, ok := args["address"].(string)
	if !ok || addressInput == "" {
		return "", fmt.Errorf("address parameter is required")
	}

	// Split by comma for multi-address support
	addresses := strings.Split(addressInput, ",")
	for i := range addresses {
		addresses[i] = strings.TrimSpace(addresses[i])
	}

	if len(addresses) > 20 {
		return "", fmt.Errorf("maximum 20 addresses supported, got %d", len(addresses))
	}

	// Validate all addresses
	for _, addr := range addresses {
		if err := validateAddress(addr); err != nil {
			return "", fmt.Errorf("invalid address '%s': %w", addr, err)
		}
	}

	log.Printf("🔗 [0G-BALANCE] Fetching balance for %d address(es)", len(addresses))

	params := url.Values{}
	params.Set("module", "account")

	if len(addresses) == 1 {
		params.Set("action", "balance")
		params.Set("address", addresses[0])
	} else {
		params.Set("action", "balancemulti")
		params.Set("address", strings.Join(addresses, ","))
	}

	requestURL := fmt.Sprintf("%s?%s", zerogChainscanAPI, params.Encode())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(requestURL)
	if err != nil {
		return "", fmt.Errorf("failed to reach 0G Chainscan API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp chainscanResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse API response: %w", err)
	}

	if apiResp.Status != "1" {
		return fmt.Sprintf("Could not fetch balance. API message: %s", apiResp.Message), nil
	}

	if len(addresses) == 1 {
		// Single address returns a string balance
		var balance string
		if err := json.Unmarshal(apiResp.Result, &balance); err != nil {
			balance = string(apiResp.Result)
		}
		return fmt.Sprintf("Native $0G balance for %s: %s wei", addresses[0], balance), nil
	}

	// Multi-address returns [{account, balance}, ...]
	var balances []struct {
		Account string `json:"account"`
		Balance string `json:"balance"`
	}
	if err := json.Unmarshal(apiResp.Result, &balances); err != nil {
		return "", fmt.Errorf("failed to parse balance data: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Native $0G balances for %d addresses:\n\n", len(balances)))
	for i, b := range balances {
		sb.WriteString(fmt.Sprintf("[%d] %s: %s wei\n", i+1, b.Account, b.Balance))
	}
	return sb.String(), nil
}

// ──────────────────────────────────────────────────────────────
// Tool 6: get_0g_token_balance
// ──────────────────────────────────────────────────────────────

// NewZeroGTokenBalanceTool creates the get_0g_token_balance tool.
func NewZeroGTokenBalanceTool() *Tool {
	return &Tool{
		Name:        "get_0g_token_balance",
		DisplayName: "0G Token Balance",
		Description: "Get the ERC-20 token balance for a specific address and token contract on the 0G network (Chain ID: 16661). Returns the raw token balance (divide by 10^decimals for human-readable value).",
		Icon:        "Coins",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"address": map[string]interface{}{
					"type":        "string",
					"description": "The 0x wallet address to check balance for",
				},
				"contract_address": map[string]interface{}{
					"type":        "string",
					"description": "The 0x address of the ERC-20 token contract",
				},
			},
			"required": []string{"address", "contract_address"},
		},
		Execute:  executeGetZeroGTokenBalance,
		Source:   ToolSourceBuiltin,
		Category: "blockchain",
		Keywords: []string{"0g", "blockchain", "token", "balance", "erc20", "web3", "wallet"},
	}
}

func executeGetZeroGTokenBalance(args map[string]interface{}) (string, error) {
	address, ok := args["address"].(string)
	if !ok || address == "" {
		return "", fmt.Errorf("address parameter is required")
	}
	address = strings.TrimSpace(address)
	if err := validateAddress(address); err != nil {
		return "", fmt.Errorf("invalid address: %w", err)
	}

	contractAddress, ok := args["contract_address"].(string)
	if !ok || contractAddress == "" {
		return "", fmt.Errorf("contract_address parameter is required")
	}
	contractAddress = strings.TrimSpace(contractAddress)
	if err := validateAddress(contractAddress); err != nil {
		return "", fmt.Errorf("invalid contract_address: %w", err)
	}

	log.Printf("🔗 [0G-TOKEN-BAL] Fetching token balance for %s on contract %s", address, contractAddress)

	params := url.Values{}
	params.Set("module", "account")
	params.Set("action", "tokenbalance")
	params.Set("address", address)
	params.Set("contractaddress", contractAddress)

	requestURL := fmt.Sprintf("%s?%s", zerogChainscanAPI, params.Encode())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(requestURL)
	if err != nil {
		return "", fmt.Errorf("failed to reach 0G Chainscan API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp chainscanResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse API response: %w", err)
	}

	if apiResp.Status != "1" {
		return fmt.Sprintf("Could not fetch token balance. API message: %s", apiResp.Message), nil
	}

	var balance string
	if err := json.Unmarshal(apiResp.Result, &balance); err != nil {
		balance = string(apiResp.Result)
	}

	return fmt.Sprintf("Token balance for %s on contract %s: %s (raw units, divide by token decimals for human-readable value)", address, contractAddress, balance), nil
}

// ──────────────────────────────────────────────────────────────
// Tool 7: get_0g_contract_info
// ──────────────────────────────────────────────────────────────

// NewZeroGContractInfoTool creates the get_0g_contract_info tool.
func NewZeroGContractInfoTool() *Tool {
	return &Tool{
		Name:        "get_0g_contract_info",
		DisplayName: "0G Contract Info",
		Description: "Get detailed information about a smart contract on the 0G network including source code, compiler version, optimization settings, and constructor arguments. Also supports looking up contract creation details (creator address, creation tx hash).",
		Icon:        "FileSearch",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"contract_address": map[string]interface{}{
					"type":        "string",
					"description": "The 0x address of the smart contract",
				},
				"info_type": map[string]interface{}{
					"type":        "string",
					"description": "Type of info to retrieve: 'source' for source code and compiler details, 'creation' for creator and creation tx",
					"enum":        []string{"source", "creation"},
					"default":     "source",
				},
			},
			"required": []string{"contract_address"},
		},
		Execute:  executeGetZeroGContractInfo,
		Source:   ToolSourceBuiltin,
		Category: "blockchain",
		Keywords: []string{"0g", "blockchain", "contract", "source code", "creator", "creation", "compiler", "solidity", "verified"},
	}
}

func executeGetZeroGContractInfo(args map[string]interface{}) (string, error) {
	contractAddress, ok := args["contract_address"].(string)
	if !ok || contractAddress == "" {
		return "", fmt.Errorf("contract_address parameter is required")
	}
	contractAddress = strings.TrimSpace(contractAddress)
	if err := validateAddress(contractAddress); err != nil {
		return "", fmt.Errorf("invalid contract_address: %w", err)
	}

	infoType := "source"
	if t, ok := args["info_type"].(string); ok && t != "" {
		infoType = t
	}

	params := url.Values{}
	params.Set("module", "contract")

	switch infoType {
	case "source":
		params.Set("action", "getsourcecode")
	case "creation":
		params.Set("action", "getcontractcreation")
	default:
		return "", fmt.Errorf("invalid info_type '%s', must be 'source' or 'creation'", infoType)
	}

	params.Set("contractaddresses", contractAddress)
	// getsourcecode uses 'address', getcontractcreation uses 'contractaddresses'
	if infoType == "source" {
		params.Set("address", contractAddress)
	}

	log.Printf("🔗 [0G-CONTRACT-INFO] Fetching %s info for contract %s", infoType, contractAddress)

	requestURL := fmt.Sprintf("%s?%s", zerogChainscanAPI, params.Encode())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(requestURL)
	if err != nil {
		return "", fmt.Errorf("failed to reach 0G Chainscan API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp chainscanResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse API response: %w", err)
	}

	if apiResp.Status != "1" {
		return fmt.Sprintf("Could not retrieve contract info. API message: %s", apiResp.Message), nil
	}

	if infoType == "source" {
		var sources []struct {
			SourceCode           string `json:"SourceCode"`
			ABI                  string `json:"ABI"`
			ContractName         string `json:"ContractName"`
			CompilerVersion      string `json:"CompilerVersion"`
			OptimizationUsed     string `json:"OptimizationUsed"`
			Runs                 string `json:"Runs"`
			ConstructorArguments string `json:"ConstructorArguments"`
			EVMVersion           string `json:"EVMVersion"`
			Library              string `json:"Library"`
			LicenseType          string `json:"LicenseType"`
			Proxy                string `json:"Proxy"`
			Implementation       string `json:"Implementation"`
		}
		if err := json.Unmarshal(apiResp.Result, &sources); err != nil {
			return fmt.Sprintf("Contract source info for %s:\n%s", contractAddress, string(apiResp.Result)), nil
		}
		if len(sources) == 0 {
			return fmt.Sprintf("No source code information found for contract %s", contractAddress), nil
		}
		src := sources[0]
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Contract Info for %s on 0G Network:\n\n", contractAddress))
		sb.WriteString(fmt.Sprintf("  Name:              %s\n", src.ContractName))
		sb.WriteString(fmt.Sprintf("  Compiler:          %s\n", src.CompilerVersion))
		sb.WriteString(fmt.Sprintf("  EVM Version:       %s\n", src.EVMVersion))
		sb.WriteString(fmt.Sprintf("  Optimization:      %s (runs: %s)\n", src.OptimizationUsed, src.Runs))
		sb.WriteString(fmt.Sprintf("  License:           %s\n", src.LicenseType))
		if src.Proxy != "0" && src.Proxy != "" {
			sb.WriteString(fmt.Sprintf("  Proxy:             Yes (impl: %s)\n", src.Implementation))
		}
		if src.ConstructorArguments != "" {
			sb.WriteString(fmt.Sprintf("  Constructor Args:  %s\n", src.ConstructorArguments))
		}
		// Truncate source code if very long
		sourceCode := src.SourceCode
		if len(sourceCode) > 5000 {
			sourceCode = sourceCode[:5000] + "\n... (truncated, full source is " + fmt.Sprintf("%d", len(src.SourceCode)) + " chars)"
		}
		sb.WriteString(fmt.Sprintf("\nSource Code:\n%s\n", sourceCode))
		return sb.String(), nil
	}

	// Creation info
	var creations []struct {
		ContractAddress string `json:"contractAddress"`
		ContractCreator string `json:"contractCreator"`
		TxHash          string `json:"txHash"`
	}
	if err := json.Unmarshal(apiResp.Result, &creations); err != nil {
		return fmt.Sprintf("Contract creation info for %s:\n%s", contractAddress, string(apiResp.Result)), nil
	}
	if len(creations) == 0 {
		return fmt.Sprintf("No creation information found for contract %s", contractAddress), nil
	}
	c := creations[0]
	return fmt.Sprintf("Contract Creation Info for %s on 0G Network:\n\n  Creator: %s\n  Tx Hash: %s", c.ContractAddress, c.ContractCreator, c.TxHash), nil
}

// ──────────────────────────────────────────────────────────────
// Tool 8: get_0g_tx_status
// ──────────────────────────────────────────────────────────────

// NewZeroGTxStatusTool creates the get_0g_tx_status tool.
func NewZeroGTxStatusTool() *Tool {
	return &Tool{
		Name:        "get_0g_tx_status",
		DisplayName: "0G Transaction Status",
		Description: "Check the execution status and receipt of a transaction on the 0G network (Chain ID: 16661). Returns whether the transaction succeeded or failed, with error messages if applicable.",
		Icon:        "CheckCircle",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"txhash": map[string]interface{}{
					"type":        "string",
					"description": "The transaction hash to check (0x-prefixed)",
				},
			},
			"required": []string{"txhash"},
		},
		Execute:  executeGetZeroGTxStatus,
		Source:   ToolSourceBuiltin,
		Category: "blockchain",
		Keywords: []string{"0g", "blockchain", "transaction", "status", "receipt", "success", "failed", "hash"},
	}
}

func executeGetZeroGTxStatus(args map[string]interface{}) (string, error) {
	txhash, ok := args["txhash"].(string)
	if !ok || txhash == "" {
		return "", fmt.Errorf("txhash parameter is required")
	}
	txhash = strings.TrimSpace(txhash)
	if !strings.HasPrefix(txhash, "0x") && !strings.HasPrefix(txhash, "0X") {
		return "", fmt.Errorf("transaction hash must start with 0x")
	}

	log.Printf("🔗 [0G-TX-STATUS] Checking status for tx: %s", txhash)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Transaction Status for %s on 0G Network:\n\n", txhash))

	// Fetch execution status (getstatus)
	params := url.Values{}
	params.Set("module", "transaction")
	params.Set("action", "getstatus")
	params.Set("txhash", txhash)

	client := &http.Client{Timeout: 30 * time.Second}

	requestURL := fmt.Sprintf("%s?%s", zerogChainscanAPI, params.Encode())
	resp, err := client.Get(requestURL)
	if err != nil {
		return "", fmt.Errorf("failed to reach 0G Chainscan API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp chainscanResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse API response: %w", err)
	}

	if apiResp.Status == "1" {
		var execStatus struct {
			IsError        string `json:"isError"`
			ErrDescription string `json:"errDescription"`
		}
		if err := json.Unmarshal(apiResp.Result, &execStatus); err == nil {
			if execStatus.IsError == "0" {
				sb.WriteString("  Execution: SUCCESS\n")
			} else {
				sb.WriteString(fmt.Sprintf("  Execution: FAILED — %s\n", execStatus.ErrDescription))
			}
		}
	} else {
		sb.WriteString(fmt.Sprintf("  Execution Status: %s\n", apiResp.Message))
	}

	// Fetch receipt status (gettxreceiptstatus)
	params2 := url.Values{}
	params2.Set("module", "transaction")
	params2.Set("action", "gettxreceiptstatus")
	params2.Set("txhash", txhash)

	requestURL2 := fmt.Sprintf("%s?%s", zerogChainscanAPI, params2.Encode())
	resp2, err := client.Get(requestURL2)
	if err != nil {
		sb.WriteString(fmt.Sprintf("  Receipt: Could not fetch (%v)\n", err))
		return sb.String(), nil
	}
	defer resp2.Body.Close()

	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		sb.WriteString("  Receipt: Could not read response\n")
		return sb.String(), nil
	}

	var apiResp2 chainscanResponse
	if err := json.Unmarshal(body2, &apiResp2); err == nil && apiResp2.Status == "1" {
		var receiptStatus struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(apiResp2.Result, &receiptStatus); err == nil {
			if receiptStatus.Status == "1" {
				sb.WriteString("  Receipt: CONFIRMED (status=1)\n")
			} else if receiptStatus.Status == "0" {
				sb.WriteString("  Receipt: FAILED (status=0)\n")
			} else {
				sb.WriteString(fmt.Sprintf("  Receipt: status=%s\n", receiptStatus.Status))
			}
		}
	}

	log.Printf("✅ [0G-TX-STATUS] Checked status for %s", txhash)
	return sb.String(), nil
}

// ──────────────────────────────────────────────────────────────
// Tool 9: get_0g_token_supply
// ──────────────────────────────────────────────────────────────

// NewZeroGTokenSupplyTool creates the get_0g_token_supply tool.
func NewZeroGTokenSupplyTool() *Tool {
	return &Tool{
		Name:        "get_0g_token_supply",
		DisplayName: "0G Token Supply",
		Description: "Get the total supply of an ERC-20 token on the 0G network (Chain ID: 16661). Returns the total supply in the token's smallest unit.",
		Icon:        "PieChart",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"contract_address": map[string]interface{}{
					"type":        "string",
					"description": "The 0x address of the ERC-20 token contract",
				},
			},
			"required": []string{"contract_address"},
		},
		Execute:  executeGetZeroGTokenSupply,
		Source:   ToolSourceBuiltin,
		Category: "blockchain",
		Keywords: []string{"0g", "blockchain", "token", "supply", "total supply", "erc20", "web3"},
	}
}

func executeGetZeroGTokenSupply(args map[string]interface{}) (string, error) {
	contractAddress, ok := args["contract_address"].(string)
	if !ok || contractAddress == "" {
		return "", fmt.Errorf("contract_address parameter is required")
	}
	contractAddress = strings.TrimSpace(contractAddress)
	if err := validateAddress(contractAddress); err != nil {
		return "", fmt.Errorf("invalid contract_address: %w", err)
	}

	log.Printf("🔗 [0G-TOKEN-SUPPLY] Fetching total supply for %s", contractAddress)

	params := url.Values{}
	params.Set("module", "stats")
	params.Set("action", "tokensupply")
	params.Set("contractaddress", contractAddress)

	requestURL := fmt.Sprintf("%s?%s", zerogChainscanAPI, params.Encode())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(requestURL)
	if err != nil {
		return "", fmt.Errorf("failed to reach 0G Chainscan API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp chainscanResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse API response: %w", err)
	}

	if apiResp.Status != "1" {
		return fmt.Sprintf("Could not fetch token supply. API message: %s", apiResp.Message), nil
	}

	var supply string
	if err := json.Unmarshal(apiResp.Result, &supply); err != nil {
		supply = string(apiResp.Result)
	}

	return fmt.Sprintf("Total supply for token contract %s on 0G Network: %s (raw units)", contractAddress, supply), nil
}

// ──────────────────────────────────────────────────────────────
// Tool 10: get_0g_nft_data
// ──────────────────────────────────────────────────────────────

// NewZeroGNFTDataTool creates the get_0g_nft_data tool.
func NewZeroGNFTDataTool() *Tool {
	return &Tool{
		Name:        "get_0g_nft_data",
		DisplayName: "0G NFT Data",
		Description: "Query NFT (ERC-721/ERC-1155) data on the 0G network: check balances, list tokens, view transfers, find owners, and search NFT collections. Also supports ERC-721 transfer history via Chainscan tokennfttx API.",
		Icon:        "Image",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"description": "NFT query action: 'balances' (NFTs owned by address), 'tokens' (tokens in a collection), 'transfers' (transfer history), 'owners' (owners of a token), 'tokennfttx' (ERC-721 transfer history from Chainscan API)",
					"enum":        []string{"balances", "tokens", "transfers", "owners", "tokennfttx"},
				},
				"address": map[string]interface{}{
					"type":        "string",
					"description": "Wallet address (required for 'balances' and 'tokennfttx' actions)",
				},
				"contract_address": map[string]interface{}{
					"type":        "string",
					"description": "NFT contract address (required for 'tokens', 'transfers', 'owners')",
				},
				"token_id": map[string]interface{}{
					"type":        "string",
					"description": "Specific token ID (optional, for 'owners' and 'transfers')",
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number (default: 1)",
					"default":     1,
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Results per page (default: 25, max: 100)",
					"default":     25,
				},
			},
			"required": []string{"action"},
		},
		Execute:  executeGetZeroGNFTData,
		Source:   ToolSourceBuiltin,
		Category: "blockchain",
		Keywords: []string{"0g", "blockchain", "nft", "erc721", "erc1155", "token", "collectible", "web3"},
	}
}

func executeGetZeroGNFTData(args map[string]interface{}) (string, error) {
	action, ok := args["action"].(string)
	if !ok || action == "" {
		return "", fmt.Errorf("action parameter is required")
	}

	address, _ := args["address"].(string)
	contractAddress, _ := args["contract_address"].(string)
	tokenID, _ := args["token_id"].(string)

	page := 1
	if p, ok := args["page"].(float64); ok && p > 0 {
		page = int(p)
	}
	limit := 25
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
		if limit > 100 {
			limit = 100
		}
	}

	// For tokennfttx, use Chainscan API (Etherscan-compatible)
	if action == "tokennfttx" {
		if address == "" {
			return "", fmt.Errorf("address is required for tokennfttx action")
		}
		if err := validateAddress(strings.TrimSpace(address)); err != nil {
			return "", fmt.Errorf("invalid address: %w", err)
		}
		return fetchNFTTransfersFromChainscan(strings.TrimSpace(address), strings.TrimSpace(contractAddress), page, limit)
	}

	// For other NFT actions, use the /open/api NFT endpoints
	const nftBaseURL = "https://chainscan.0g.ai/open/api"

	params := url.Values{}

	switch action {
	case "balances":
		if address == "" {
			return "", fmt.Errorf("address is required for 'balances' action")
		}
		if err := validateAddress(strings.TrimSpace(address)); err != nil {
			return "", fmt.Errorf("invalid address: %w", err)
		}
		params.Set("module", "nft")
		params.Set("action", "balances")
		params.Set("address", strings.TrimSpace(address))
	case "tokens":
		if contractAddress == "" {
			return "", fmt.Errorf("contract_address is required for 'tokens' action")
		}
		if err := validateAddress(strings.TrimSpace(contractAddress)); err != nil {
			return "", fmt.Errorf("invalid contract_address: %w", err)
		}
		params.Set("module", "nft")
		params.Set("action", "tokens")
		params.Set("contractaddress", strings.TrimSpace(contractAddress))
	case "transfers":
		if contractAddress == "" {
			return "", fmt.Errorf("contract_address is required for 'transfers' action")
		}
		if err := validateAddress(strings.TrimSpace(contractAddress)); err != nil {
			return "", fmt.Errorf("invalid contract_address: %w", err)
		}
		params.Set("module", "nft")
		params.Set("action", "transfers")
		params.Set("contractaddress", strings.TrimSpace(contractAddress))
		if tokenID != "" {
			params.Set("tokenid", tokenID)
		}
	case "owners":
		if contractAddress == "" {
			return "", fmt.Errorf("contract_address is required for 'owners' action")
		}
		if err := validateAddress(strings.TrimSpace(contractAddress)); err != nil {
			return "", fmt.Errorf("invalid contract_address: %w", err)
		}
		params.Set("module", "nft")
		params.Set("action", "owners")
		params.Set("contractaddress", strings.TrimSpace(contractAddress))
		if tokenID != "" {
			params.Set("tokenid", tokenID)
		}
	default:
		return "", fmt.Errorf("unknown action '%s'. Valid: balances, tokens, transfers, owners, tokennfttx", action)
	}

	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("offset", fmt.Sprintf("%d", limit))

	log.Printf("🔗 [0G-NFT] Fetching NFT %s data", action)

	requestURL := fmt.Sprintf("%s?%s", nftBaseURL, params.Encode())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(requestURL)
	if err != nil {
		return "", fmt.Errorf("failed to reach 0G Chainscan API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp chainscanResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse API response: %w", err)
	}

	if apiResp.Status != "1" {
		return fmt.Sprintf("No NFT data found. API message: %s", apiResp.Message), nil
	}

	// Pretty-print the result
	var result interface{}
	if err := json.Unmarshal(apiResp.Result, &result); err == nil {
		formatted, err := json.MarshalIndent(result, "", "  ")
		if err == nil {
			return fmt.Sprintf("0G Network NFT %s data (page %d):\n\n%s", action, page, string(formatted)), nil
		}
	}

	return fmt.Sprintf("0G Network NFT %s data (page %d):\n\n%s", action, page, string(apiResp.Result)), nil
}

func fetchNFTTransfersFromChainscan(address, contractAddress string, page, limit int) (string, error) {
	params := url.Values{}
	params.Set("module", "account")
	params.Set("action", "tokennfttx")
	params.Set("address", address)
	params.Set("sort", "desc")
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("offset", fmt.Sprintf("%d", limit))

	if contractAddress != "" {
		if err := validateAddress(contractAddress); err != nil {
			return "", fmt.Errorf("invalid contract_address: %w", err)
		}
		params.Set("contractaddress", contractAddress)
	}

	requestURL := fmt.Sprintf("%s?%s", zerogChainscanAPI, params.Encode())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(requestURL)
	if err != nil {
		return "", fmt.Errorf("failed to reach 0G Chainscan API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp chainscanResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse API response: %w", err)
	}

	if apiResp.Status != "1" {
		return fmt.Sprintf("No ERC-721 transfers found for %s. API message: %s", address, apiResp.Message), nil
	}

	var transfers []struct {
		BlockNumber     string `json:"blockNumber"`
		Hash            string `json:"hash"`
		TimeStamp       string `json:"timeStamp"`
		From            string `json:"from"`
		To              string `json:"to"`
		ContractAddress string `json:"contractAddress"`
		TokenID         string `json:"tokenID"`
		TokenName       string `json:"tokenName"`
		TokenSymbol     string `json:"tokenSymbol"`
	}
	if err := json.Unmarshal(apiResp.Result, &transfers); err != nil {
		return fmt.Sprintf("ERC-721 transfers for %s:\n%s", address, string(apiResp.Result)), nil
	}

	if len(transfers) == 0 {
		return fmt.Sprintf("No ERC-721 transfers found for %s on 0G network.", address), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d ERC-721 transfer(s) for %s on 0G Network (page %d):\n\n", len(transfers), address, page))

	for i, tx := range transfers {
		sb.WriteString(fmt.Sprintf("[%d] %s (%s) — Token #%s\n", i+1, tx.TokenName, tx.TokenSymbol, tx.TokenID))
		sb.WriteString(fmt.Sprintf("    Hash:     %s\n", tx.Hash))
		sb.WriteString(fmt.Sprintf("    From:     %s\n", tx.From))
		sb.WriteString(fmt.Sprintf("    To:       %s\n", tx.To))
		sb.WriteString(fmt.Sprintf("    Contract: %s\n", tx.ContractAddress))
		sb.WriteString(fmt.Sprintf("    Block:    %s\n", tx.BlockNumber))
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// ──────────────────────────────────────────────────────────────
// Tool 11: get_0g_decode_method
// ──────────────────────────────────────────────────────────────

// NewZeroGDecodeMethodTool creates the get_0g_decode_method tool.
func NewZeroGDecodeMethodTool() *Tool {
	return &Tool{
		Name:        "get_0g_decode_method",
		DisplayName: "0G Decode Method",
		Description: "Decode a method call from transaction input data on the 0G network. Given a 4-byte method selector (e.g., '0xa9059cbb'), returns the human-readable function signature. Optionally decode full calldata with raw mode.",
		Icon:        "Code",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"data": map[string]interface{}{
					"type":        "string",
					"description": "The 0x-prefixed method selector (4 bytes, e.g., '0xa9059cbb') or full calldata to decode",
				},
				"raw": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, decode the full raw calldata (default: false, decode method selector only)",
					"default":     false,
				},
			},
			"required": []string{"data"},
		},
		Execute:  executeGetZeroGDecodeMethod,
		Source:   ToolSourceBuiltin,
		Category: "blockchain",
		Keywords: []string{"0g", "blockchain", "decode", "method", "selector", "calldata", "abi", "transaction", "input"},
	}
}

func executeGetZeroGDecodeMethod(args map[string]interface{}) (string, error) {
	data, ok := args["data"].(string)
	if !ok || data == "" {
		return "", fmt.Errorf("data parameter is required")
	}
	data = strings.TrimSpace(data)

	raw := false
	if r, ok := args["raw"].(bool); ok {
		raw = r
	}

	log.Printf("🔗 [0G-DECODE] Decoding method: %s (raw: %v)", data, raw)

	var endpoint string
	if raw {
		endpoint = "/open/api?module=decode&action=method&raw=true"
	} else {
		endpoint = "/open/api?module=decode&action=method"
	}

	requestURL := fmt.Sprintf("https://chainscan.0g.ai%s&data=%s", endpoint, url.QueryEscape(data))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(requestURL)
	if err != nil {
		return "", fmt.Errorf("failed to reach 0G Chainscan API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp chainscanResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse API response: %w", err)
	}

	if apiResp.Status != "1" {
		return fmt.Sprintf("Could not decode method. API message: %s", apiResp.Message), nil
	}

	// Pretty-print the result
	var result interface{}
	if err := json.Unmarshal(apiResp.Result, &result); err == nil {
		formatted, err := json.MarshalIndent(result, "", "  ")
		if err == nil {
			return fmt.Sprintf("Decoded method for data %s:\n\n%s", data, string(formatted)), nil
		}
	}

	var resultStr string
	if err := json.Unmarshal(apiResp.Result, &resultStr); err == nil {
		return fmt.Sprintf("Decoded method for data %s: %s", data, resultStr), nil
	}

	return fmt.Sprintf("Decoded method for data %s:\n%s", data, string(apiResp.Result)), nil
}

// ──────────────────────────────────────────────────────────────
// Tool 12: get_0g_block_by_time
// ──────────────────────────────────────────────────────────────

// NewZeroGBlockByTimeTool creates the get_0g_block_by_time tool.
func NewZeroGBlockByTimeTool() *Tool {
	return &Tool{
		Name:        "get_0g_block_by_time",
		DisplayName: "0G Block by Time",
		Description: "Find the block number closest to a given Unix timestamp on the 0G network. Useful for time-range queries that need block numbers.",
		Icon:        "Clock",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"timestamp": map[string]interface{}{
					"type":        "integer",
					"description": "Unix timestamp (seconds) to find the nearest block for",
				},
				"closest": map[string]interface{}{
					"type":        "string",
					"description": "Find the block 'before' or 'after' the timestamp (default: before)",
					"enum":        []string{"before", "after"},
					"default":     "before",
				},
			},
			"required": []string{"timestamp"},
		},
		Execute:  executeGetZeroGBlockByTime,
		Source:   ToolSourceBuiltin,
		Category: "blockchain",
		Keywords: []string{"0g", "blockchain", "block", "timestamp", "time", "blockno"},
	}
}

func executeGetZeroGBlockByTime(args map[string]interface{}) (string, error) {
	timestamp, ok := args["timestamp"].(float64)
	if !ok || timestamp <= 0 {
		return "", fmt.Errorf("timestamp parameter is required and must be a positive integer")
	}

	closest := "before"
	if c, ok := args["closest"].(string); ok && c != "" {
		closest = c
	}

	log.Printf("🔗 [0G-BLOCK-TIME] Finding block for timestamp %d (closest: %s)", int64(timestamp), closest)

	params := url.Values{}
	params.Set("module", "block")
	params.Set("action", "getblocknobytime")
	params.Set("timestamp", fmt.Sprintf("%d", int64(timestamp)))
	params.Set("closest", closest)

	requestURL := fmt.Sprintf("%s?%s", zerogChainscanAPI, params.Encode())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(requestURL)
	if err != nil {
		return "", fmt.Errorf("failed to reach 0G Chainscan API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp chainscanResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse API response: %w", err)
	}

	if apiResp.Status != "1" {
		return fmt.Sprintf("Could not find block for timestamp. API message: %s", apiResp.Message), nil
	}

	var blockNo string
	if err := json.Unmarshal(apiResp.Result, &blockNo); err != nil {
		blockNo = string(apiResp.Result)
	}

	result := fmt.Sprintf("Block number closest to timestamp %d (%s) on 0G Network: %s", int64(timestamp), closest, blockNo)

	// Warn if result is block 1 — likely means the timestamp predates the chain
	if blockNo == "1" || blockNo == "0" {
		result += "\n\nWARNING: Block 1 (genesis) returned — this timestamp likely predates the 0G network launch. The 0G network may not have been active at this time."
	}

	return result, nil
}
