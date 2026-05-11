package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// UnipileClient is a shared HTTP client for the Unipile API.
// It handles authentication and base URL construction for both
// WhatsApp and LinkedIn messaging endpoints.
type UnipileClient struct {
	baseURL     string
	accessToken string
	httpClient  *http.Client
}

// NewUnipileClientFromArgs creates a UnipileClient from credential data in the tool args.
func NewUnipileClientFromArgs(args map[string]interface{}) (*UnipileClient, error) {
	credData, err := GetCredentialData(args, "unipile")
	if err != nil {
		return nil, fmt.Errorf("failed to get Unipile credentials: %w", err)
	}

	dsn, _ := credData["dsn"].(string)
	accessToken, _ := credData["access_token"].(string)

	if dsn == "" {
		return nil, fmt.Errorf("Unipile DSN is required")
	}
	if accessToken == "" {
		return nil, fmt.Errorf("Unipile access token is required")
	}

	// Normalize DSN to base URL
	baseURL := dsn
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	return &UnipileClient{
		baseURL:     baseURL + "/api/v1",
		accessToken: accessToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Get performs a GET request to the Unipile API.
func (c *UnipileClient) Get(path string, queryParams url.Values) ([]byte, int, error) {
	fullURL := c.baseURL + path
	if len(queryParams) > 0 {
		fullURL += "?" + queryParams.Encode()
	}

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-API-KEY", c.accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}

	return body, resp.StatusCode, nil
}

// PostMultipart performs a POST request with multipart/form-data (used for sending messages).
func (c *UnipileClient) PostMultipart(path string, fields map[string]string) ([]byte, int, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	for key, val := range fields {
		if err := writer.WriteField(key, val); err != nil {
			return nil, 0, fmt.Errorf("failed to write field %s: %w", key, err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, 0, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	fullURL := c.baseURL + path
	req, err := http.NewRequest("POST", fullURL, &buf)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-API-KEY", c.accessToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}

	return body, resp.StatusCode, nil
}

// PostMultipartWithAttachment performs a POST with multipart/form-data fields and an optional file
// attachment downloaded from a URL. The file is sent as the "attachments" field per Unipile's API.
// If attachmentURL is empty, this behaves identically to PostMultipart.
func (c *UnipileClient) PostMultipartWithAttachment(apiPath string, fields map[string]string, attachmentURL string) ([]byte, int, error) {
	if attachmentURL == "" {
		return c.PostMultipart(apiPath, fields)
	}

	// Download the attachment from the URL
	dlClient := &http.Client{Timeout: 60 * time.Second}
	dlResp, err := dlClient.Get(attachmentURL)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to download attachment from %s: %w", attachmentURL, err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode < 200 || dlResp.StatusCode >= 300 {
		return nil, 0, fmt.Errorf("failed to download attachment: HTTP %d from %s", dlResp.StatusCode, attachmentURL)
	}

	// Determine filename from URL path or Content-Disposition header
	filename := "attachment"
	if cd := dlResp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			if fn, ok := params["filename"]; ok && fn != "" {
				filename = fn
			}
		}
	}
	if filename == "attachment" {
		// Fall back to URL path basename
		parsed, err := url.Parse(attachmentURL)
		if err == nil {
			base := path.Base(parsed.Path)
			if base != "" && base != "." && base != "/" {
				filename = base
			}
		}
	}

	fileData, err := io.ReadAll(dlResp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read attachment data: %w", err)
	}

	// Build multipart form with text fields + file attachment
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	for key, val := range fields {
		if err := writer.WriteField(key, val); err != nil {
			return nil, 0, fmt.Errorf("failed to write field %s: %w", key, err)
		}
	}

	filePart, err := writer.CreateFormFile("attachments", filename)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create attachment field: %w", err)
	}
	if _, err := filePart.Write(fileData); err != nil {
		return nil, 0, fmt.Errorf("failed to write attachment data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, 0, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	fullURL := c.baseURL + apiPath
	req, err := http.NewRequest("POST", fullURL, &buf)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-API-KEY", c.accessToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}

	return body, resp.StatusCode, nil
}

// PostJSON performs a POST request with JSON body.
func (c *UnipileClient) PostJSON(path string, payload interface{}) ([]byte, int, error) {
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to serialize payload: %w", err)
	}

	fullURL := c.baseURL + path
	req, err := http.NewRequest("POST", fullURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-API-KEY", c.accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}

	return body, resp.StatusCode, nil
}
