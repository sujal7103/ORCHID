package health

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// --- Chat Health Check Strategy ---

// ChatHealthCheck tests a chat provider with a minimal completion request
type ChatHealthCheck struct{}

func (c *ChatHealthCheck) Capability() CapabilityType { return CapabilityChat }

func (c *ChatHealthCheck) Check(entry *ProviderHealth, getter ProviderGetter) (int, error) {
	provider, err := getter(entry.ProviderID)
	if err != nil {
		return 0, fmt.Errorf("provider lookup failed: %w", err)
	}
	if !provider.Enabled {
		return 0, fmt.Errorf("provider %s is disabled", provider.Name)
	}

	modelName := entry.ModelName
	if modelName == "" {
		return 0, fmt.Errorf("no model specified for chat health check")
	}

	isOpenAI := strings.Contains(strings.ToLower(provider.BaseURL), "openai.com")
	requestBody := map[string]interface{}{
		"model": modelName,
		"messages": []map[string]interface{}{
			{"role": "user", "content": "hi"},
		},
	}
	if isOpenAI {
		requestBody["max_completion_tokens"] = 1
	} else {
		requestBody["max_tokens"] = 1
	}

	return doHealthCheckRequest(provider, requestBody)
}

// --- Vision Health Check Strategy ---

// VisionHealthCheck tests a vision provider with a minimal 1x1 PNG image
type VisionHealthCheck struct{}

func (v *VisionHealthCheck) Capability() CapabilityType { return CapabilityVision }

func (v *VisionHealthCheck) Check(entry *ProviderHealth, getter ProviderGetter) (int, error) {
	provider, err := getter(entry.ProviderID)
	if err != nil {
		return 0, fmt.Errorf("provider lookup failed: %w", err)
	}
	if !provider.Enabled {
		return 0, fmt.Errorf("provider %s is disabled", provider.Name)
	}

	imgBytes := generateTestImage()
	base64Image := base64.StdEncoding.EncodeToString(imgBytes)
	dataURL := fmt.Sprintf("data:image/png;base64,%s", base64Image)

	isOpenAI := strings.Contains(strings.ToLower(provider.BaseURL), "openai.com")
	requestBody := map[string]interface{}{
		"model": entry.ModelName,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "text", "text": "What color is this image? Reply in one word."},
					{"type": "image_url", "image_url": map[string]interface{}{
						"url":    dataURL,
						"detail": "low",
					}},
				},
			},
		},
	}
	if isOpenAI {
		requestBody["max_completion_tokens"] = 10
	} else {
		requestBody["max_tokens"] = 10
	}

	return doHealthCheckRequest(provider, requestBody)
}

// generateTestImage creates a minimal 1x1 pixel red PNG for health checks
func generateTestImage() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x00, 0x02, 0x00, 0x01, 0xe2, 0x21, 0xbc,
		0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}
}

// --- Image Generation Health Check Strategy ---

// ImageGenHealthCheck tests image gen providers with a lightweight API probe
type ImageGenHealthCheck struct{}

func (i *ImageGenHealthCheck) Capability() CapabilityType { return CapabilityImageGen }

func (i *ImageGenHealthCheck) Check(entry *ProviderHealth, getter ProviderGetter) (int, error) {
	provider, err := getter(entry.ProviderID)
	if err != nil {
		return 0, fmt.Errorf("provider lookup failed: %w", err)
	}
	if !provider.Enabled {
		return 0, fmt.Errorf("provider %s is disabled", provider.Name)
	}

	// Lightweight connectivity check - try /v1/models or just HEAD the base URL
	return doConnectivityCheck(provider)
}

// --- Audio Health Check Strategy ---

// AudioHealthCheck tests audio providers with a lightweight API probe
type AudioHealthCheck struct{}

func (a *AudioHealthCheck) Capability() CapabilityType { return CapabilityAudio }

func (a *AudioHealthCheck) Check(entry *ProviderHealth, getter ProviderGetter) (int, error) {
	provider, err := getter(entry.ProviderID)
	if err != nil {
		return 0, fmt.Errorf("provider lookup failed: %w", err)
	}
	if !provider.Enabled {
		return 0, fmt.Errorf("provider %s is disabled", provider.Name)
	}

	return doConnectivityCheck(provider)
}

// --- Shared helpers ---

// doHealthCheckRequest makes a chat completion request for health checking
func doHealthCheckRequest(provider *ProviderInfo, requestBody map[string]interface{}) (int, error) {
	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal health check request: %w", err)
	}

	apiURL := fmt.Sprintf("%s/chat/completions", strings.TrimSuffix(provider.BaseURL, "/"))
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(requestJSON))
	if err != nil {
		return 0, fmt.Errorf("failed to create health check request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", provider.APIKey))

	client := &http.Client{Timeout: 30 * time.Second}
	startTime := time.Now()

	resp, err := client.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	latencyMs := int(time.Since(startTime).Milliseconds())

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return latencyMs, fmt.Errorf("failed to read health check response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		bodyStr := string(body)
		if IsQuotaError(resp.StatusCode, bodyStr) {
			return latencyMs, fmt.Errorf("quota exceeded: %s", bodyStr)
		}
		return latencyMs, fmt.Errorf("health check API error %d: %s", resp.StatusCode, bodyStr)
	}

	return latencyMs, nil
}

// doConnectivityCheck performs a lightweight check by hitting the /models endpoint
func doConnectivityCheck(provider *ProviderInfo) (int, error) {
	modelsURL := fmt.Sprintf("%s/models", strings.TrimSuffix(provider.BaseURL, "/"))
	httpReq, err := http.NewRequest("GET", modelsURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create connectivity check request: %w", err)
	}

	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", provider.APIKey))

	client := &http.Client{Timeout: 15 * time.Second}
	startTime := time.Now()

	resp, err := client.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("connectivity check failed: %w", err)
	}
	defer resp.Body.Close()

	latencyMs := int(time.Since(startTime).Milliseconds())

	if resp.StatusCode == http.StatusUnauthorized {
		return latencyMs, fmt.Errorf("authentication failed (invalid API key)")
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		body, _ := io.ReadAll(resp.Body)
		return latencyMs, fmt.Errorf("quota exceeded: %s", string(body))
	}

	// Any 2xx or even 404 (endpoint doesn't exist but server is up) is considered "connected"
	if resp.StatusCode >= 500 {
		body, _ := io.ReadAll(resp.Body)
		return latencyMs, fmt.Errorf("server error %d: %s", resp.StatusCode, string(body))
	}

	return latencyMs, nil
}
