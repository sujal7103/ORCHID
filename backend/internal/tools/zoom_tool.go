package tools

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Zoom OAuth token cache to avoid generating new tokens on every request
var (
	zoomTokenCache   = make(map[string]*zoomCachedToken)
	zoomTokenCacheMu sync.RWMutex
)

type zoomCachedToken struct {
	AccessToken string
	ExpiresAt   time.Time
}

// NewZoomTool creates a Zoom meeting and webinar management tool
func NewZoomTool() *Tool {
	return &Tool{
		Name:        "zoom_meeting",
		DisplayName: "Zoom Meeting & Webinar",
		Description: `Manage Zoom meetings and webinars - create, list, get details, and create registrations.

Features:
- Create instant or scheduled meetings
- Create webinars (requires webinar add-on license)
- List user's meetings or webinars
- Get meeting/webinar details
- Create meeting/webinar registrations (for registration-required events)

Authentication is handled automatically via configured Zoom Server-to-Server OAuth credentials.
Do NOT ask users for credentials - they configure them once in the Credentials page.

IMPORTANT: For registration, the meeting/webinar must have registration enabled when created.
NOTE: Webinar features require a Zoom Webinar add-on license.`,
		Icon:     "Video",
		Source:   ToolSourceBuiltin,
		Category: "integration",
		Keywords: []string{"zoom", "meeting", "webinar", "video", "conference", "call", "schedule", "registration", "broadcast"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"create", "list", "get", "register", "create_webinar", "list_webinars", "get_webinar", "register_webinar"},
					"description": "Action: 'create' (meeting), 'list' (meetings), 'get' (meeting details), 'register' (meeting registrant), 'create_webinar', 'list_webinars', 'get_webinar', 'register_webinar'",
				},
				"meeting_id": map[string]interface{}{
					"type":        "string",
					"description": "Meeting ID (required for 'get', 'register' actions)",
				},
				"webinar_id": map[string]interface{}{
					"type":        "string",
					"description": "Webinar ID (required for 'get_webinar', 'register_webinar' actions)",
				},
				"topic": map[string]interface{}{
					"type":        "string",
					"description": "Meeting/Webinar topic/title (required for 'create' and 'create_webinar')",
				},
				"type": map[string]interface{}{
					"type":        "number",
					"description": "Meeting type: 1=instant, 2=scheduled (default), 3=recurring no fixed time, 8=recurring fixed time. Webinar type: 5=webinar, 6=recurring no fixed time, 9=recurring fixed time",
				},
				"start_time": map[string]interface{}{
					"type":        "string",
					"description": "Start time in ISO 8601 format (e.g., '2024-01-15T10:00:00Z'). Required for scheduled meetings/webinars.",
				},
				"duration": map[string]interface{}{
					"type":        "number",
					"description": "Duration in minutes (default: 60)",
				},
				"timezone": map[string]interface{}{
					"type":        "string",
					"description": "Timezone (e.g., 'America/New_York', 'UTC'). Default: UTC",
				},
				"agenda": map[string]interface{}{
					"type":        "string",
					"description": "Meeting/Webinar agenda/description",
				},
				"password": map[string]interface{}{
					"type":        "string",
					"description": "Password (auto-generated if not provided)",
				},
				"registration_required": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether registration is required to join",
				},
				"registrant_email": map[string]interface{}{
					"type":        "string",
					"description": "Email of the person to register (required for 'register' and 'register_webinar')",
				},
				"registrant_first_name": map[string]interface{}{
					"type":        "string",
					"description": "First name of the registrant (required for 'register' and 'register_webinar')",
				},
				"registrant_last_name": map[string]interface{}{
					"type":        "string",
					"description": "Last name of the registrant (optional)",
				},
				"user_id": map[string]interface{}{
					"type":        "string",
					"description": "Zoom user ID or email (default: 'me' for the authenticated user)",
				},
			},
			"required": []string{"action"},
		},
		Execute: executeZoomMeeting,
	}
}

func executeZoomMeeting(args map[string]interface{}) (string, error) {
	// Get credential data for Zoom
	credData, err := GetCredentialData(args, "zoom")
	if err != nil {
		return "", fmt.Errorf("failed to get Zoom credentials: %w. Please configure Zoom credentials first.", err)
	}

	// Extract required credentials
	accountID, _ := credData["account_id"].(string)
	clientID, _ := credData["client_id"].(string)
	clientSecret, _ := credData["client_secret"].(string)

	if accountID == "" || clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("Zoom credentials incomplete: account_id, client_id, and client_secret are required")
	}

	// Get OAuth access token
	accessToken, err := getZoomAccessToken(accountID, clientID, clientSecret)
	if err != nil {
		return "", fmt.Errorf("failed to get Zoom access token: %w", err)
	}

	// Get action
	action, ok := args["action"].(string)
	if !ok || action == "" {
		return "", fmt.Errorf("'action' is required (create, list, get, register, create_webinar, list_webinars, get_webinar, register_webinar)")
	}

	// Execute based on action
	switch action {
	case "create":
		return createZoomMeeting(accessToken, args)
	case "list":
		return listZoomMeetings(accessToken, args)
	case "get":
		return getZoomMeeting(accessToken, args)
	case "register":
		return registerZoomMeeting(accessToken, args)
	case "create_webinar":
		return createZoomWebinar(accessToken, args)
	case "list_webinars":
		return listZoomWebinars(accessToken, args)
	case "get_webinar":
		return getZoomWebinar(accessToken, args)
	case "register_webinar":
		return registerZoomWebinar(accessToken, args)
	default:
		return "", fmt.Errorf("unknown action: %s. Valid actions: create, list, get, register, create_webinar, list_webinars, get_webinar, register_webinar", action)
	}
}

// getZoomAccessToken gets an OAuth access token using Server-to-Server OAuth
func getZoomAccessToken(accountID, clientID, clientSecret string) (string, error) {
	cacheKey := accountID + ":" + clientID

	// Check cache first
	zoomTokenCacheMu.RLock()
	cached, exists := zoomTokenCache[cacheKey]
	zoomTokenCacheMu.RUnlock()

	if exists && time.Now().Before(cached.ExpiresAt) {
		return cached.AccessToken, nil
	}

	// Request new token
	tokenURL := "https://zoom.us/oauth/token"
	data := url.Values{}
	data.Set("grant_type", "account_credentials")
	data.Set("account_id", accountID)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	// Basic auth with client_id:client_secret
	auth := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Zoom OAuth failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	// Cache the token (expire 5 minutes early to be safe)
	zoomTokenCacheMu.Lock()
	zoomTokenCache[cacheKey] = &zoomCachedToken{
		AccessToken: tokenResp.AccessToken,
		ExpiresAt:   time.Now().Add(time.Duration(tokenResp.ExpiresIn-300) * time.Second),
	}
	zoomTokenCacheMu.Unlock()

	return tokenResp.AccessToken, nil
}

// createZoomMeeting creates a new Zoom meeting
func createZoomMeeting(accessToken string, args map[string]interface{}) (string, error) {
	topic, _ := args["topic"].(string)
	if topic == "" {
		return "", fmt.Errorf("'topic' is required for creating a meeting")
	}

	userID := "me"
	if uid, ok := args["user_id"].(string); ok && uid != "" {
		userID = uid
	}

	// Build meeting request
	meeting := map[string]interface{}{
		"topic": topic,
		"type":  2, // Default to scheduled meeting
	}

	if meetingType, ok := args["type"].(float64); ok {
		meeting["type"] = int(meetingType)
	}

	if startTime, ok := args["start_time"].(string); ok && startTime != "" {
		meeting["start_time"] = startTime
	}

	if duration, ok := args["duration"].(float64); ok {
		meeting["duration"] = int(duration)
	} else {
		meeting["duration"] = 60 // Default 60 minutes
	}

	if timezone, ok := args["timezone"].(string); ok && timezone != "" {
		meeting["timezone"] = timezone
	}

	if agenda, ok := args["agenda"].(string); ok && agenda != "" {
		meeting["agenda"] = agenda
	}

	if password, ok := args["password"].(string); ok && password != "" {
		meeting["password"] = password
	}

	// Meeting settings
	settings := map[string]interface{}{
		"join_before_host": true,
		"mute_upon_entry":  true,
		"waiting_room":     false,
	}

	if regRequired, ok := args["registration_required"].(bool); ok && regRequired {
		settings["approval_type"] = 0 // Auto-approve
		meeting["type"] = 2           // Must be scheduled for registration
	}

	meeting["settings"] = settings

	// Make API request
	apiURL := fmt.Sprintf("https://api.zoom.us/v2/users/%s/meetings", userID)
	jsonBody, _ := json.Marshal(meeting)

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Zoom API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	json.Unmarshal(body, &result)

	// Format the response nicely
	output := map[string]interface{}{
		"success":    true,
		"meeting_id": result["id"],
		"topic":      result["topic"],
		"join_url":   result["join_url"],
		"start_url":  result["start_url"],
		"password":   result["password"],
		"start_time": result["start_time"],
		"duration":   result["duration"],
		"timezone":   result["timezone"],
		"host_email": result["host_email"],
	}

	jsonResult, _ := json.MarshalIndent(output, "", "  ")
	return string(jsonResult), nil
}

// listZoomMeetings lists meetings for a user
func listZoomMeetings(accessToken string, args map[string]interface{}) (string, error) {
	userID := "me"
	if uid, ok := args["user_id"].(string); ok && uid != "" {
		userID = uid
	}

	apiURL := fmt.Sprintf("https://api.zoom.us/v2/users/%s/meetings?type=upcoming&page_size=30", userID)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Zoom API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	json.Unmarshal(body, &result)

	output := map[string]interface{}{
		"success":       true,
		"total_records": result["total_records"],
		"meetings":      result["meetings"],
	}

	jsonResult, _ := json.MarshalIndent(output, "", "  ")
	return string(jsonResult), nil
}

// getZoomMeeting gets details of a specific meeting
func getZoomMeeting(accessToken string, args map[string]interface{}) (string, error) {
	meetingID, ok := args["meeting_id"].(string)
	if !ok || meetingID == "" {
		return "", fmt.Errorf("'meeting_id' is required for getting meeting details")
	}

	apiURL := fmt.Sprintf("https://api.zoom.us/v2/meetings/%s", meetingID)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Zoom API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	json.Unmarshal(body, &result)

	output := map[string]interface{}{
		"success":    true,
		"meeting_id": result["id"],
		"topic":      result["topic"],
		"type":       result["type"],
		"status":     result["status"],
		"start_time": result["start_time"],
		"duration":   result["duration"],
		"timezone":   result["timezone"],
		"join_url":   result["join_url"],
		"password":   result["password"],
		"host_id":    result["host_id"],
		"host_email": result["host_email"],
		"settings":   result["settings"],
	}

	jsonResult, _ := json.MarshalIndent(output, "", "  ")
	return string(jsonResult), nil
}

// registerZoomMeeting registers a participant for a meeting
func registerZoomMeeting(accessToken string, args map[string]interface{}) (string, error) {
	meetingID, ok := args["meeting_id"].(string)
	if !ok || meetingID == "" {
		return "", fmt.Errorf("'meeting_id' is required for registration")
	}

	email, ok := args["registrant_email"].(string)
	if !ok || email == "" {
		return "", fmt.Errorf("'registrant_email' is required for registration")
	}

	firstName, ok := args["registrant_first_name"].(string)
	if !ok || firstName == "" {
		return "", fmt.Errorf("'registrant_first_name' is required for registration")
	}

	registrant := map[string]interface{}{
		"email":      email,
		"first_name": firstName,
	}

	if lastName, ok := args["registrant_last_name"].(string); ok && lastName != "" {
		registrant["last_name"] = lastName
	}

	apiURL := fmt.Sprintf("https://api.zoom.us/v2/meetings/%s/registrants", meetingID)
	jsonBody, _ := json.Marshal(registrant)

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Zoom API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	json.Unmarshal(body, &result)

	output := map[string]interface{}{
		"success":       true,
		"registrant_id": result["registrant_id"],
		"meeting_id":    meetingID,
		"email":         email,
		"first_name":    firstName,
		"join_url":      result["join_url"],
		"topic":         result["topic"],
		"start_time":    result["start_time"],
	}

	jsonResult, _ := json.MarshalIndent(output, "", "  ")
	return string(jsonResult), nil
}

// createZoomWebinar creates a new Zoom webinar
func createZoomWebinar(accessToken string, args map[string]interface{}) (string, error) {
	topic, _ := args["topic"].(string)
	if topic == "" {
		return "", fmt.Errorf("'topic' is required for creating a webinar")
	}

	userID := "me"
	if uid, ok := args["user_id"].(string); ok && uid != "" {
		userID = uid
	}

	// Build webinar request
	webinar := map[string]interface{}{
		"topic": topic,
		"type":  5, // Default to webinar
	}

	if webinarType, ok := args["type"].(float64); ok {
		webinar["type"] = int(webinarType)
	}

	if startTime, ok := args["start_time"].(string); ok && startTime != "" {
		webinar["start_time"] = startTime
	}

	if duration, ok := args["duration"].(float64); ok {
		webinar["duration"] = int(duration)
	} else {
		webinar["duration"] = 60
	}

	if timezone, ok := args["timezone"].(string); ok && timezone != "" {
		webinar["timezone"] = timezone
	}

	if agenda, ok := args["agenda"].(string); ok && agenda != "" {
		webinar["agenda"] = agenda
	}

	if password, ok := args["password"].(string); ok && password != "" {
		webinar["password"] = password
	}

	// Webinar settings
	settings := map[string]interface{}{
		"approval_type":     0, // Auto-approve registrants
		"registration_type": 1, // Register once and attend any occurrence
		"attendees_and_panelists_reminder_email_notification": map[string]interface{}{
			"enable": true,
		},
	}

	if regRequired, ok := args["registration_required"].(bool); ok && regRequired {
		settings["approval_type"] = 0
	}

	webinar["settings"] = settings

	// Make API request
	apiURL := fmt.Sprintf("https://api.zoom.us/v2/users/%s/webinars", userID)
	jsonBody, _ := json.Marshal(webinar)

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Zoom API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	json.Unmarshal(body, &result)

	output := map[string]interface{}{
		"success":    true,
		"webinar_id": result["id"],
		"topic":      result["topic"],
		"join_url":   result["join_url"],
		"start_url":  result["start_url"],
		"password":   result["password"],
		"start_time": result["start_time"],
		"duration":   result["duration"],
		"timezone":   result["timezone"],
		"host_email": result["host_email"],
		"type":       "webinar",
	}

	jsonResult, _ := json.MarshalIndent(output, "", "  ")
	return string(jsonResult), nil
}

// listZoomWebinars lists webinars for a user
func listZoomWebinars(accessToken string, args map[string]interface{}) (string, error) {
	userID := "me"
	if uid, ok := args["user_id"].(string); ok && uid != "" {
		userID = uid
	}

	apiURL := fmt.Sprintf("https://api.zoom.us/v2/users/%s/webinars?page_size=30", userID)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Zoom API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	json.Unmarshal(body, &result)

	output := map[string]interface{}{
		"success":       true,
		"total_records": result["total_records"],
		"webinars":      result["webinars"],
		"type":          "webinar_list",
	}

	jsonResult, _ := json.MarshalIndent(output, "", "  ")
	return string(jsonResult), nil
}

// getZoomWebinar gets details of a specific webinar
func getZoomWebinar(accessToken string, args map[string]interface{}) (string, error) {
	webinarID, ok := args["webinar_id"].(string)
	if !ok || webinarID == "" {
		return "", fmt.Errorf("'webinar_id' is required for getting webinar details")
	}

	apiURL := fmt.Sprintf("https://api.zoom.us/v2/webinars/%s", webinarID)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Zoom API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	json.Unmarshal(body, &result)

	output := map[string]interface{}{
		"success":    true,
		"webinar_id": result["id"],
		"topic":      result["topic"],
		"type":       result["type"],
		"start_time": result["start_time"],
		"duration":   result["duration"],
		"timezone":   result["timezone"],
		"join_url":   result["join_url"],
		"password":   result["password"],
		"host_id":    result["host_id"],
		"host_email": result["host_email"],
		"settings":   result["settings"],
		"event_type": "webinar",
	}

	jsonResult, _ := json.MarshalIndent(output, "", "  ")
	return string(jsonResult), nil
}

// registerZoomWebinar registers a participant for a webinar
func registerZoomWebinar(accessToken string, args map[string]interface{}) (string, error) {
	webinarID, ok := args["webinar_id"].(string)
	if !ok || webinarID == "" {
		return "", fmt.Errorf("'webinar_id' is required for webinar registration")
	}

	email, ok := args["registrant_email"].(string)
	if !ok || email == "" {
		return "", fmt.Errorf("'registrant_email' is required for registration")
	}

	firstName, ok := args["registrant_first_name"].(string)
	if !ok || firstName == "" {
		return "", fmt.Errorf("'registrant_first_name' is required for registration")
	}

	registrant := map[string]interface{}{
		"email":      email,
		"first_name": firstName,
	}

	if lastName, ok := args["registrant_last_name"].(string); ok && lastName != "" {
		registrant["last_name"] = lastName
	}

	apiURL := fmt.Sprintf("https://api.zoom.us/v2/webinars/%s/registrants", webinarID)
	jsonBody, _ := json.Marshal(registrant)

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Zoom API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	json.Unmarshal(body, &result)

	output := map[string]interface{}{
		"success":       true,
		"registrant_id": result["registrant_id"],
		"webinar_id":    webinarID,
		"email":         email,
		"first_name":    firstName,
		"join_url":      result["join_url"],
		"topic":         result["topic"],
		"start_time":    result["start_time"],
		"type":          "webinar_registration",
	}

	jsonResult, _ := json.MarshalIndent(output, "", "  ")
	return string(jsonResult), nil
}
