package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// NewShopifyProductsTool creates a Shopify products listing tool
func NewShopifyProductsTool() *Tool {
	return &Tool{
		Name:        "shopify_products",
		DisplayName: "Shopify Products",
		Description: `List products from a Shopify store.

Returns product details including title, description, variants, and inventory.
Authentication is handled automatically via configured Shopify credentials.`,
		Icon:     "ShoppingCart",
		Source:   ToolSourceBuiltin,
		Category: "integration",
		Keywords: []string{"shopify", "products", "ecommerce", "inventory", "store"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system.",
				},
				"limit": map[string]interface{}{
					"type":        "number",
					"description": "Number of products to return (max 250, default 50)",
				},
				"product_type": map[string]interface{}{
					"type":        "string",
					"description": "Filter by product type",
				},
				"vendor": map[string]interface{}{
					"type":        "string",
					"description": "Filter by vendor",
				},
				"status": map[string]interface{}{
					"type":        "string",
					"description": "Filter by status: active, archived, or draft",
				},
			},
			"required": []string{},
		},
		Execute: executeShopifyProducts,
	}
}

// NewShopifyOrdersTool creates a Shopify orders listing tool
func NewShopifyOrdersTool() *Tool {
	return &Tool{
		Name:        "shopify_orders",
		DisplayName: "Shopify Orders",
		Description: `List orders from a Shopify store.

Returns order details including line items, customer info, and fulfillment status.
Authentication is handled automatically via configured Shopify credentials.`,
		Icon:     "Package",
		Source:   ToolSourceBuiltin,
		Category: "integration",
		Keywords: []string{"shopify", "orders", "ecommerce", "sales", "fulfillment"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system.",
				},
				"limit": map[string]interface{}{
					"type":        "number",
					"description": "Number of orders to return (max 250, default 50)",
				},
				"status": map[string]interface{}{
					"type":        "string",
					"description": "Filter by status: open, closed, cancelled, any",
				},
				"financial_status": map[string]interface{}{
					"type":        "string",
					"description": "Filter: pending, authorized, partially_paid, paid, refunded, voided",
				},
				"fulfillment_status": map[string]interface{}{
					"type":        "string",
					"description": "Filter: shipped, partial, unshipped, any",
				},
			},
			"required": []string{},
		},
		Execute: executeShopifyOrders,
	}
}

// NewShopifyCustomersTool creates a Shopify customers listing tool
func NewShopifyCustomersTool() *Tool {
	return &Tool{
		Name:        "shopify_customers",
		DisplayName: "Shopify Customers",
		Description: `List customers from a Shopify store.

Returns customer details including name, email, orders count, and total spent.
Authentication is handled automatically via configured Shopify credentials.`,
		Icon:     "Users",
		Source:   ToolSourceBuiltin,
		Category: "integration",
		Keywords: []string{"shopify", "customers", "ecommerce", "crm"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system.",
				},
				"limit": map[string]interface{}{
					"type":        "number",
					"description": "Number of customers to return (max 250, default 50)",
				},
			},
			"required": []string{},
		},
		Execute: executeShopifyCustomers,
	}
}

func buildShopifyRequest(storeURL, accessToken, endpoint string, queryParams url.Values) (*http.Request, error) {
	storeURL = strings.TrimPrefix(storeURL, "https://")
	storeURL = strings.TrimPrefix(storeURL, "http://")
	storeURL = strings.TrimSuffix(storeURL, "/")

	apiURL := fmt.Sprintf("https://%s/admin/api/2025-01/%s.json", storeURL, endpoint)
	if len(queryParams) > 0 {
		apiURL += "?" + queryParams.Encode()
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Shopify-Access-Token", accessToken)
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func executeShopifyProducts(args map[string]interface{}) (string, error) {
	credData, err := GetCredentialData(args, "shopify")
	if err != nil {
		return "", fmt.Errorf("failed to get Shopify credentials: %w", err)
	}

	storeURL, _ := credData["store_url"].(string)
	accessToken, _ := credData["access_token"].(string)
	if storeURL == "" || accessToken == "" {
		return "", fmt.Errorf("Shopify credentials incomplete: store_url and access_token are required")
	}

	queryParams := url.Values{}
	if limit, ok := args["limit"].(float64); ok && limit > 0 {
		queryParams.Set("limit", fmt.Sprintf("%d", int(limit)))
	} else {
		queryParams.Set("limit", "50")
	}
	if pt, ok := args["product_type"].(string); ok && pt != "" {
		queryParams.Set("product_type", pt)
	}
	if v, ok := args["vendor"].(string); ok && v != "" {
		queryParams.Set("vendor", v)
	}
	if s, ok := args["status"].(string); ok && s != "" {
		queryParams.Set("status", s)
	}

	req, err := buildShopifyRequest(storeURL, accessToken, "products", queryParams)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if resp.StatusCode >= 400 {
		errMsg := "unknown error"
		if e, ok := result["errors"].(string); ok {
			errMsg = e
		}
		return "", fmt.Errorf("Shopify API error: %s", errMsg)
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult), nil
}

func executeShopifyOrders(args map[string]interface{}) (string, error) {
	credData, err := GetCredentialData(args, "shopify")
	if err != nil {
		return "", fmt.Errorf("failed to get Shopify credentials: %w", err)
	}

	storeURL, _ := credData["store_url"].(string)
	accessToken, _ := credData["access_token"].(string)
	if storeURL == "" || accessToken == "" {
		return "", fmt.Errorf("Shopify credentials incomplete: store_url and access_token are required")
	}

	queryParams := url.Values{}
	if limit, ok := args["limit"].(float64); ok && limit > 0 {
		queryParams.Set("limit", fmt.Sprintf("%d", int(limit)))
	} else {
		queryParams.Set("limit", "50")
	}
	if s, ok := args["status"].(string); ok && s != "" {
		queryParams.Set("status", s)
	}
	if fs, ok := args["financial_status"].(string); ok && fs != "" {
		queryParams.Set("financial_status", fs)
	}
	if ffs, ok := args["fulfillment_status"].(string); ok && ffs != "" {
		queryParams.Set("fulfillment_status", ffs)
	}

	req, err := buildShopifyRequest(storeURL, accessToken, "orders", queryParams)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if resp.StatusCode >= 400 {
		errMsg := "unknown error"
		if e, ok := result["errors"].(string); ok {
			errMsg = e
		}
		return "", fmt.Errorf("Shopify API error: %s", errMsg)
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult), nil
}

func executeShopifyCustomers(args map[string]interface{}) (string, error) {
	credData, err := GetCredentialData(args, "shopify")
	if err != nil {
		return "", fmt.Errorf("failed to get Shopify credentials: %w", err)
	}

	storeURL, _ := credData["store_url"].(string)
	accessToken, _ := credData["access_token"].(string)
	if storeURL == "" || accessToken == "" {
		return "", fmt.Errorf("Shopify credentials incomplete: store_url and access_token are required")
	}

	queryParams := url.Values{}
	if limit, ok := args["limit"].(float64); ok && limit > 0 {
		queryParams.Set("limit", fmt.Sprintf("%d", int(limit)))
	} else {
		queryParams.Set("limit", "50")
	}

	req, err := buildShopifyRequest(storeURL, accessToken, "customers", queryParams)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if resp.StatusCode >= 400 {
		errMsg := "unknown error"
		if e, ok := result["errors"].(string); ok {
			errMsg = e
		}
		return "", fmt.Errorf("Shopify API error: %s", errMsg)
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult), nil
}
