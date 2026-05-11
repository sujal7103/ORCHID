package tools

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const xAPIBase = "https://api.x.com/2"

// NewXSearchPostsTool creates a tool for searching X posts
func NewXSearchPostsTool() *Tool {
	return &Tool{
		Name:        "x_search_posts",
		DisplayName: "Search X Posts",
		Description: "Search for posts on X (Twitter) using the v2 API. Supports advanced query operators. Authentication is handled automatically via configured credentials.",
		Icon:        "Search",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"x", "twitter", "search", "posts", "tweets", "social"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query. Supports operators like from:, to:, #hashtag, @mention, has:media, is:retweet, lang:, etc.",
				},
				"max_results": map[string]interface{}{
					"type":        "number",
					"description": "Maximum number of results (10-100, default 10)",
				},
				"sort_order": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"recency", "relevancy"},
					"description": "Sort order (default: recency)",
				},
			},
			"required": []string{"query"},
		},
		Execute: executeXSearchPosts,
	}
}

// NewXPostTweetTool creates a tool for posting tweets
func NewXPostTweetTool() *Tool {
	return &Tool{
		Name:        "x_post_tweet",
		DisplayName: "Post to X",
		Description: "Post a new tweet to X (Twitter). Requires OAuth 1.0a credentials (API Key, API Secret, Access Token, Access Token Secret). Authentication is handled automatically.",
		Icon:        "Send",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"x", "twitter", "post", "tweet", "publish", "social"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"text": map[string]interface{}{
					"type":        "string",
					"description": "The text content of the tweet (max 280 characters)",
				},
				"reply_to": map[string]interface{}{
					"type":        "string",
					"description": "Tweet ID to reply to (optional)",
				},
				"quote_tweet_id": map[string]interface{}{
					"type":        "string",
					"description": "Tweet ID to quote (optional)",
				},
			},
			"required": []string{"text"},
		},
		Execute: executeXPostTweet,
	}
}

// NewXGetUserTool creates a tool for getting user info
func NewXGetUserTool() *Tool {
	return &Tool{
		Name:        "x_get_user",
		DisplayName: "Get X User",
		Description: "Get information about an X (Twitter) user by username. Authentication is handled automatically.",
		Icon:        "User",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"x", "twitter", "user", "profile", "account"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"username": map[string]interface{}{
					"type":        "string",
					"description": "X username (without @)",
				},
			},
			"required": []string{"username"},
		},
		Execute: executeXGetUser,
	}
}

// NewXGetUserPostsTool creates a tool for getting a user's posts
func NewXGetUserPostsTool() *Tool {
	return &Tool{
		Name:        "x_get_user_posts",
		DisplayName: "Get User's X Posts",
		Description: "Get recent posts from an X (Twitter) user. Authentication is handled automatically.",
		Icon:        "FileText",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"x", "twitter", "user", "posts", "tweets", "timeline"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"user_id": map[string]interface{}{
					"type":        "string",
					"description": "X user ID (numeric)",
				},
				"username": map[string]interface{}{
					"type":        "string",
					"description": "X username (alternative to user_id, will be resolved)",
				},
				"max_results": map[string]interface{}{
					"type":        "number",
					"description": "Maximum number of results (5-100, default 10)",
				},
			},
			"required": []string{},
		},
		Execute: executeXGetUserPosts,
	}
}

type xCredentials struct {
	BearerToken       string
	APIKey            string
	APISecret         string
	AccessToken       string
	AccessTokenSecret string
}

func getXCredentials(args map[string]interface{}) (*xCredentials, error) {
	credData, err := GetCredentialData(args, "x_twitter")
	if err != nil {
		return nil, fmt.Errorf("failed to get X credentials: %w", err)
	}

	creds := &xCredentials{
		BearerToken:       credData["bearer_token"].(string),
		APIKey:            "",
		APISecret:         "",
		AccessToken:       "",
		AccessTokenSecret: "",
	}

	if apiKey, ok := credData["api_key"].(string); ok {
		creds.APIKey = apiKey
	}
	if apiSecret, ok := credData["api_secret"].(string); ok {
		creds.APISecret = apiSecret
	}
	if accessToken, ok := credData["access_token"].(string); ok {
		creds.AccessToken = accessToken
	}
	if accessTokenSecret, ok := credData["access_token_secret"].(string); ok {
		creds.AccessTokenSecret = accessTokenSecret
	}

	if creds.BearerToken == "" {
		return nil, fmt.Errorf("bearer_token is required")
	}

	return creds, nil
}

func xBearerRequest(method, endpoint, bearerToken string, body interface{}) (map[string]interface{}, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, xAPIBase+endpoint, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.StatusCode >= 400 {
		errMsg := "X API error"
		if errors, ok := result["errors"].([]interface{}); ok && len(errors) > 0 {
			if errObj, ok := errors[0].(map[string]interface{}); ok {
				if msg, ok := errObj["message"].(string); ok {
					errMsg = msg
				}
			}
		}
		if detail, ok := result["detail"].(string); ok {
			errMsg = detail
		}
		return nil, fmt.Errorf("%s (status %d)", errMsg, resp.StatusCode)
	}

	return result, nil
}

// OAuth 1.0a signing for posting tweets
func generateOAuthSignature(method, urlStr string, params map[string]string, consumerSecret, tokenSecret string) string {
	// Sort parameters
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Create parameter string
	var paramPairs []string
	for _, k := range keys {
		paramPairs = append(paramPairs, fmt.Sprintf("%s=%s", url.QueryEscape(k), url.QueryEscape(params[k])))
	}
	paramString := strings.Join(paramPairs, "&")

	// Create signature base string
	signatureBase := fmt.Sprintf("%s&%s&%s",
		strings.ToUpper(method),
		url.QueryEscape(urlStr),
		url.QueryEscape(paramString),
	)

	// Create signing key
	signingKey := fmt.Sprintf("%s&%s", url.QueryEscape(consumerSecret), url.QueryEscape(tokenSecret))

	// Generate HMAC-SHA1
	h := hmac.New(sha1.New, []byte(signingKey))
	h.Write([]byte(signatureBase))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return signature
}

func generateNonce() string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func xOAuthRequest(method, endpoint string, creds *xCredentials, body interface{}) (map[string]interface{}, error) {
	urlStr := xAPIBase + endpoint

	// OAuth parameters
	oauthParams := map[string]string{
		"oauth_consumer_key":     creds.APIKey,
		"oauth_nonce":            generateNonce(),
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        strconv.FormatInt(time.Now().Unix(), 10),
		"oauth_token":            creds.AccessToken,
		"oauth_version":          "1.0",
	}

	// Generate signature
	signature := generateOAuthSignature(method, urlStr, oauthParams, creds.APISecret, creds.AccessTokenSecret)
	oauthParams["oauth_signature"] = signature

	// Build Authorization header
	var authParts []string
	for k, v := range oauthParams {
		authParts = append(authParts, fmt.Sprintf(`%s="%s"`, k, url.QueryEscape(v)))
	}
	sort.Strings(authParts)
	authHeader := "OAuth " + strings.Join(authParts, ", ")

	// Create request
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, urlStr, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.StatusCode >= 400 {
		errMsg := "X API error"
		if detail, ok := result["detail"].(string); ok {
			errMsg = detail
		}
		if title, ok := result["title"].(string); ok {
			errMsg = title
		}
		return nil, fmt.Errorf("%s (status %d)", errMsg, resp.StatusCode)
	}

	return result, nil
}

func executeXSearchPosts(args map[string]interface{}) (string, error) {
	creds, err := getXCredentials(args)
	if err != nil {
		return "", err
	}

	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	maxResults := 10
	if mr, ok := args["max_results"].(float64); ok && mr > 0 {
		maxResults = int(mr)
		if maxResults < 10 {
			maxResults = 10
		}
		if maxResults > 100 {
			maxResults = 100
		}
	}

	// Build endpoint with query params
	params := url.Values{}
	params.Set("query", query)
	params.Set("max_results", strconv.Itoa(maxResults))
	params.Set("tweet.fields", "created_at,public_metrics,author_id,conversation_id")
	params.Set("expansions", "author_id")
	params.Set("user.fields", "name,username,verified")

	if sortOrder, ok := args["sort_order"].(string); ok && sortOrder != "" {
		params.Set("sort_order", sortOrder)
	}

	endpoint := "/tweets/search/recent?" + params.Encode()

	result, err := xBearerRequest("GET", endpoint, creds.BearerToken, nil)
	if err != nil {
		return "", err
	}

	// Process results
	posts := []map[string]interface{}{}
	if data, ok := result["data"].([]interface{}); ok {
		for _, p := range data {
			if post, ok := p.(map[string]interface{}); ok {
				posts = append(posts, post)
			}
		}
	}

	// Get user info
	users := map[string]map[string]interface{}{}
	if includes, ok := result["includes"].(map[string]interface{}); ok {
		if usersData, ok := includes["users"].([]interface{}); ok {
			for _, u := range usersData {
				if user, ok := u.(map[string]interface{}); ok {
					if id, ok := user["id"].(string); ok {
						users[id] = user
					}
				}
			}
		}
	}

	// Enrich posts with user info
	for i, post := range posts {
		if authorID, ok := post["author_id"].(string); ok {
			if user, exists := users[authorID]; exists {
				posts[i]["author"] = user
			}
		}
	}

	response := map[string]interface{}{
		"success": true,
		"count":   len(posts),
		"posts":   posts,
		"meta":    result["meta"],
	}

	jsonResult, _ := json.MarshalIndent(response, "", "  ")
	return string(jsonResult), nil
}

func executeXPostTweet(args map[string]interface{}) (string, error) {
	creds, err := getXCredentials(args)
	if err != nil {
		return "", err
	}

	// Check for OAuth 1.0a credentials
	if creds.APIKey == "" || creds.APISecret == "" || creds.AccessToken == "" || creds.AccessTokenSecret == "" {
		return "", fmt.Errorf("posting tweets requires OAuth 1.0a credentials (api_key, api_secret, access_token, access_token_secret)")
	}

	text, _ := args["text"].(string)
	if text == "" {
		return "", fmt.Errorf("text is required")
	}

	if len(text) > 280 {
		return "", fmt.Errorf("tweet text exceeds 280 characters")
	}

	body := map[string]interface{}{
		"text": text,
	}

	if replyTo, ok := args["reply_to"].(string); ok && replyTo != "" {
		body["reply"] = map[string]interface{}{
			"in_reply_to_tweet_id": replyTo,
		}
	}

	if quoteTweetID, ok := args["quote_tweet_id"].(string); ok && quoteTweetID != "" {
		body["quote_tweet_id"] = quoteTweetID
	}

	result, err := xOAuthRequest("POST", "/tweets", creds, body)
	if err != nil {
		return "", err
	}

	data, _ := result["data"].(map[string]interface{})

	response := map[string]interface{}{
		"success":  true,
		"message":  "Tweet posted successfully",
		"tweet_id": data["id"],
		"text":     data["text"],
	}

	jsonResult, _ := json.MarshalIndent(response, "", "  ")
	return string(jsonResult), nil
}

func executeXGetUser(args map[string]interface{}) (string, error) {
	creds, err := getXCredentials(args)
	if err != nil {
		return "", err
	}

	username, _ := args["username"].(string)
	if username == "" {
		return "", fmt.Errorf("username is required")
	}

	// Remove @ if present
	username = strings.TrimPrefix(username, "@")

	params := url.Values{}
	params.Set("user.fields", "created_at,description,public_metrics,verified,profile_image_url,location,url")

	endpoint := fmt.Sprintf("/users/by/username/%s?%s", username, params.Encode())

	result, err := xBearerRequest("GET", endpoint, creds.BearerToken, nil)
	if err != nil {
		return "", err
	}

	data, _ := result["data"].(map[string]interface{})

	response := map[string]interface{}{
		"success": true,
		"user":    data,
	}

	jsonResult, _ := json.MarshalIndent(response, "", "  ")
	return string(jsonResult), nil
}

func executeXGetUserPosts(args map[string]interface{}) (string, error) {
	creds, err := getXCredentials(args)
	if err != nil {
		return "", err
	}

	userID, _ := args["user_id"].(string)
	username, _ := args["username"].(string)

	// If username provided, resolve to user ID first
	if userID == "" && username != "" {
		username = strings.TrimPrefix(username, "@")
		userResult, err := xBearerRequest("GET", fmt.Sprintf("/users/by/username/%s", username), creds.BearerToken, nil)
		if err != nil {
			return "", fmt.Errorf("failed to resolve username: %w", err)
		}
		if data, ok := userResult["data"].(map[string]interface{}); ok {
			userID, _ = data["id"].(string)
		}
	}

	if userID == "" {
		return "", fmt.Errorf("either user_id or username is required")
	}

	maxResults := 10
	if mr, ok := args["max_results"].(float64); ok && mr > 0 {
		maxResults = int(mr)
		if maxResults < 5 {
			maxResults = 5
		}
		if maxResults > 100 {
			maxResults = 100
		}
	}

	params := url.Values{}
	params.Set("max_results", strconv.Itoa(maxResults))
	params.Set("tweet.fields", "created_at,public_metrics,conversation_id")

	endpoint := fmt.Sprintf("/users/%s/tweets?%s", userID, params.Encode())

	result, err := xBearerRequest("GET", endpoint, creds.BearerToken, nil)
	if err != nil {
		return "", err
	}

	posts := []map[string]interface{}{}
	if data, ok := result["data"].([]interface{}); ok {
		for _, p := range data {
			if post, ok := p.(map[string]interface{}); ok {
				posts = append(posts, post)
			}
		}
	}

	response := map[string]interface{}{
		"success": true,
		"user_id": userID,
		"count":   len(posts),
		"posts":   posts,
		"meta":    result["meta"],
	}

	jsonResult, _ := json.MarshalIndent(response, "", "  ")
	return string(jsonResult), nil
}

