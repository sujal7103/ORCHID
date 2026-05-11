package vision

import (
	"bytes"
	"clara-agents/internal/health"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Provider represents a minimal provider interface for vision
type Provider struct {
	ID      int
	Name    string
	BaseURL string
	APIKey  string
	Enabled bool
}

// ModelAlias represents a model alias with vision support info
type ModelAlias struct {
	DisplayName    string
	ActualModel    string
	SupportsVision *bool
}

// ProviderGetter is a function type to get provider by ID
type ProviderGetter func(id int) (*Provider, error)

// VisionModelFinder is a function type to find vision-capable models
type VisionModelFinder func() (providerID int, modelName string, err error)

// Service handles image analysis using vision-capable models
type Service struct {
	httpClient        *http.Client
	providerGetter    ProviderGetter
	visionModelFinder VisionModelFinder
	healthService     *health.Service
	mu                sync.RWMutex
}

var (
	instance *Service
	once     sync.Once
)

// GetService returns the singleton vision service
// Note: Must call InitService first to set up dependencies
func GetService() *Service {
	return instance
}

// InitService initializes the vision service with dependencies.
// healthSvc is optional - if nil, the service uses the legacy single-provider behavior.
func InitService(providerGetter ProviderGetter, visionModelFinder VisionModelFinder, healthSvc *health.Service) *Service {
	once.Do(func() {
		instance = &Service{
			httpClient: &http.Client{
				Timeout: 60 * time.Second,
			},
			providerGetter:    providerGetter,
			visionModelFinder: visionModelFinder,
			healthService:     healthSvc,
		}
	})
	return instance
}

// DescribeImageRequest contains parameters for image description
type DescribeImageRequest struct {
	ImageData []byte
	MimeType  string
	Question  string // Optional question about the image
	Detail    string // "brief" or "detailed"
}

// DescribeImageResponse contains the result of image description
type DescribeImageResponse struct {
	Description string `json:"description"`
	Model       string `json:"model"`
	Provider    string `json:"provider"`
}

// DescribeImage analyzes an image and returns a text description.
// When a health service is available, it tries all healthy providers with automatic failover.
// Otherwise, it falls back to the legacy single-provider behavior.
func (s *Service) DescribeImage(req *DescribeImageRequest) (*DescribeImageResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.providerGetter == nil {
		return nil, fmt.Errorf("vision service not properly initialized")
	}

	log.Printf("[VISION] Analyzing image (%d bytes, %s)", len(req.ImageData), req.MimeType)

	// Convert to base64
	base64Image := base64.StdEncoding.EncodeToString(req.ImageData)
	dataURL := fmt.Sprintf("data:%s;base64,%s", req.MimeType, base64Image)

	// Build the prompt
	prompt := "Describe this image in detail."
	if req.Question != "" {
		prompt = req.Question
	} else if req.Detail == "brief" {
		prompt = "Briefly describe this image in 1-2 sentences."
	}

	// If health service is available, use failover across all healthy providers
	if s.healthService != nil {
		return s.describeImageWithFailover(dataURL, prompt)
	}

	// Legacy fallback: single provider from visionModelFinder
	return s.describeImageLegacy(dataURL, prompt)
}

// describeImageWithFailover tries all healthy vision providers in priority order
func (s *Service) describeImageWithFailover(dataURL string, prompt string) (*DescribeImageResponse, error) {
	providers := s.healthService.GetHealthyProviders(health.CapabilityVision)
	if len(providers) == 0 {
		log.Printf("[VISION] No healthy providers found, falling back to legacy finder")
		return s.describeImageLegacy(dataURL, prompt)
	}

	log.Printf("[VISION] %d healthy vision provider(s) available for failover", len(providers))

	var lastErr error
	for i, h := range providers {
		if s.healthService.IsInCooldown(health.CapabilityVision, h.ProviderID, h.ModelName) {
			log.Printf("[VISION] Skipping %s/%s (in cooldown)", h.ProviderName, h.ModelName)
			continue
		}

		provider, err := s.providerGetter(h.ProviderID)
		if err != nil {
			log.Printf("[VISION] Failed to get provider %d: %v", h.ProviderID, err)
			continue
		}

		if !provider.Enabled {
			continue
		}

		if i > 0 {
			log.Printf("[VISION] Failing over to %s/%s", provider.Name, h.ModelName)
		}

		result, err := s.callVisionAPI(provider, h.ModelName, dataURL, prompt)
		if err == nil {
			s.healthService.MarkHealthy(health.CapabilityVision, h.ProviderID, h.ModelName)
			return result, nil
		}

		// Report failure to health service
		bodyStr := err.Error()
		if health.IsQuotaError(0, bodyStr) {
			cooldown := health.ParseCooldownDuration(0, bodyStr)
			s.healthService.SetCooldown(health.CapabilityVision, h.ProviderID, h.ModelName, cooldown)
			log.Printf("[VISION] Provider %s/%s quota exceeded, cooldown %v",
				provider.Name, h.ModelName, cooldown)
		} else {
			s.healthService.MarkUnhealthy(health.CapabilityVision, h.ProviderID, h.ModelName, bodyStr, 0)
		}
		lastErr = err
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all vision providers failed, last error: %w", lastErr)
	}
	return nil, fmt.Errorf("no vision providers available")
}

// describeImageLegacy uses the original single-provider approach
func (s *Service) describeImageLegacy(dataURL string, prompt string) (*DescribeImageResponse, error) {
	if s.visionModelFinder == nil {
		return nil, fmt.Errorf("no vision model finder configured")
	}

	providerID, modelName, err := s.visionModelFinder()
	if err != nil {
		return nil, fmt.Errorf("no vision-capable model available: %w", err)
	}

	provider, err := s.providerGetter(providerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	return s.callVisionAPI(provider, modelName, dataURL, prompt)
}

// callVisionAPI makes the actual API call to a single provider
func (s *Service) callVisionAPI(provider *Provider, modelName string, dataURL string, prompt string) (*DescribeImageResponse, error) {
	messages := []map[string]interface{}{
		{
			"role": "user",
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": prompt,
				},
				{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url":    dataURL,
						"detail": "auto",
					},
				},
			},
		},
	}

	isOpenAI := strings.Contains(strings.ToLower(provider.BaseURL), "openai.com")
	requestBody := map[string]interface{}{
		"model":    modelName,
		"messages": messages,
	}

	if isOpenAI {
		requestBody["max_completion_tokens"] = 1000
	} else {
		requestBody["max_tokens"] = 1000
	}

	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := fmt.Sprintf("%s/chat/completions", strings.TrimSuffix(provider.BaseURL, "/"))
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(requestJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", provider.APIKey))

	log.Printf("[VISION] Calling %s with model %s", provider.Name, modelName)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		bodyStr := string(body)
		log.Printf("[VISION] API error from %s: %d - %s", provider.Name, resp.StatusCode, bodyStr)

		if health.IsQuotaError(resp.StatusCode, bodyStr) {
			return nil, fmt.Errorf("quota exceeded: %s", bodyStr)
		}
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, bodyStr)
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from vision model")
	}

	description := apiResp.Choices[0].Message.Content
	log.Printf("[VISION] Image described successfully via %s/%s (%d chars)",
		provider.Name, modelName, len(description))

	return &DescribeImageResponse{
		Description: description,
		Model:       modelName,
		Provider:    provider.Name,
	}, nil
}
