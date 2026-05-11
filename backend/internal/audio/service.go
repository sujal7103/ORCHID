package audio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Provider represents a minimal provider interface for audio
type Provider struct {
	ID      int
	Name    string
	BaseURL string
	APIKey  string
	Enabled bool
}

// ProviderGetter is a function type to get a provider
type ProviderGetter func() (*Provider, error)

// HealthReporter provides health awareness without importing the health package
type HealthReporter struct {
	IsHealthy   func(providerID int) bool
	MarkHealthy func(providerID int)
	MarkFailed  func(providerID int, errMsg string, statusCode int)
}

// Service handles audio transcription using Whisper API (Groq or OpenAI)
type Service struct {
	httpClient           *http.Client
	groqProviderGetter   ProviderGetter
	openaiProviderGetter ProviderGetter
	healthReporter       *HealthReporter
	mu                   sync.RWMutex
}

var (
	instance *Service
	once     sync.Once
)

// GetService returns the singleton audio service
func GetService() *Service {
	return instance
}

// InitService initializes the audio service with dependencies
// Priority: Groq (cheaper) -> OpenAI (fallback)
func InitService(groqProviderGetter, openaiProviderGetter ProviderGetter) *Service {
	once.Do(func() {
		instance = &Service{
			httpClient: &http.Client{
				Timeout: 120 * time.Second, // Whisper can take a while for long audio
			},
			groqProviderGetter:   groqProviderGetter,
			openaiProviderGetter: openaiProviderGetter,
		}
	})
	return instance
}

// SetHealthReporter sets the optional health reporter for provider health tracking
func (s *Service) SetHealthReporter(hr *HealthReporter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.healthReporter = hr
}

// TranscribeRequest contains parameters for audio transcription
type TranscribeRequest struct {
	AudioPath          string
	Language           string // Optional language code (e.g., "en", "es", "fr")
	Prompt             string // Optional prompt to guide transcription
	TranslateToEnglish bool   // If true, translates non-English audio to English
}

// TranscribeResponse contains the result of transcription
type TranscribeResponse struct {
	Text     string  `json:"text"`
	Language string  `json:"language,omitempty"`
	Duration float64 `json:"duration,omitempty"`
	Provider string  `json:"provider,omitempty"` // Which provider was used
}

// Transcribe transcribes audio to text using Whisper API
// Tries Groq first (cheaper), falls back to OpenAI
// If TranslateToEnglish is true, uses the translation endpoint to output English
func (s *Service) Transcribe(req *TranscribeRequest) (*TranscribeResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	action := "Transcribing"
	if req.TranslateToEnglish {
		action = "Translating to English"
	}
	log.Printf("🎵 [AUDIO] %s audio: %s", action, req.AudioPath)

	// Try Groq first (much cheaper: $0.04/hour vs OpenAI $0.36/hour)
	// Note: Groq supports transcription but translation support may be limited
	if s.groqProviderGetter != nil && !req.TranslateToEnglish {
		provider, err := s.groqProviderGetter()
		if err == nil && provider != nil && provider.APIKey != "" {
			// Check health before trying
			if s.healthReporter != nil && s.healthReporter.IsHealthy != nil && !s.healthReporter.IsHealthy(provider.ID) {
				log.Printf("[AUDIO] Groq (ID:%d) is unhealthy, skipping to OpenAI", provider.ID)
			} else {
				log.Printf("[AUDIO] Using Groq Whisper (whisper-large-v3)")
				resp, err := s.transcribeWithGroq(req, provider)
				if err == nil {
					if s.healthReporter != nil && s.healthReporter.MarkHealthy != nil {
						s.healthReporter.MarkHealthy(provider.ID)
					}
					return resp, nil
				}
				log.Printf("[AUDIO] Groq transcription failed, trying OpenAI: %v", err)
				if s.healthReporter != nil && s.healthReporter.MarkFailed != nil {
					s.healthReporter.MarkFailed(provider.ID, err.Error(), 0)
				}
			}
		}
	}

	// Use OpenAI for translation or as fallback for transcription
	if s.openaiProviderGetter != nil {
		provider, err := s.openaiProviderGetter()
		if err == nil && provider != nil && provider.APIKey != "" {
			// Check health before trying
			if s.healthReporter != nil && s.healthReporter.IsHealthy != nil && !s.healthReporter.IsHealthy(provider.ID) {
				log.Printf("[AUDIO] OpenAI (ID:%d) is unhealthy, skipping", provider.ID)
			} else {
				if req.TranslateToEnglish {
					log.Printf("[AUDIO] Using OpenAI Whisper Translation (whisper-1)")
					resp, err := s.translateWithOpenAI(req, provider)
					if err == nil {
						if s.healthReporter != nil && s.healthReporter.MarkHealthy != nil {
							s.healthReporter.MarkHealthy(provider.ID)
						}
						return resp, nil
					}
					if s.healthReporter != nil && s.healthReporter.MarkFailed != nil {
						s.healthReporter.MarkFailed(provider.ID, err.Error(), 0)
					}
					return nil, err
				}
				log.Printf("[AUDIO] Using OpenAI Whisper (whisper-1)")
				resp, err := s.transcribeWithOpenAI(req, provider)
				if err == nil {
					if s.healthReporter != nil && s.healthReporter.MarkHealthy != nil {
						s.healthReporter.MarkHealthy(provider.ID)
					}
					return resp, nil
				}
				if s.healthReporter != nil && s.healthReporter.MarkFailed != nil {
					s.healthReporter.MarkFailed(provider.ID, err.Error(), 0)
				}
				return nil, err
			}
		}
	}

	return nil, fmt.Errorf("no audio provider configured or all providers are unhealthy")
}

// transcribeWithGroq uses Groq's Whisper API (whisper-large-v3)
func (s *Service) transcribeWithGroq(req *TranscribeRequest, provider *Provider) (*TranscribeResponse, error) {
	return s.transcribeWithProvider(req, provider, "https://api.groq.com/openai/v1/audio/transcriptions", "whisper-large-v3", "Groq")
}

// transcribeWithOpenAI uses OpenAI's Whisper API (whisper-1)
func (s *Service) transcribeWithOpenAI(req *TranscribeRequest, provider *Provider) (*TranscribeResponse, error) {
	return s.transcribeWithProvider(req, provider, "https://api.openai.com/v1/audio/transcriptions", "whisper-1", "OpenAI")
}

// translateWithOpenAI uses OpenAI's Whisper Translation API to translate audio to English
func (s *Service) translateWithOpenAI(req *TranscribeRequest, provider *Provider) (*TranscribeResponse, error) {
	return s.transcribeWithProvider(req, provider, "https://api.openai.com/v1/audio/translations", "whisper-1", "OpenAI-Translation")
}

// transcribeWithProvider is the common transcription logic for any Whisper-compatible API
func (s *Service) transcribeWithProvider(req *TranscribeRequest, provider *Provider, apiURL, model, providerName string) (*TranscribeResponse, error) {
	// Open audio file
	audioFile, err := os.Open(req.AudioPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open audio file: %w", err)
	}
	defer audioFile.Close()

	// Get file info
	fileInfo, err := audioFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat audio file: %w", err)
	}

	log.Printf("🔄 [AUDIO] Sending audio to %s Whisper API (%d bytes, model: %s)", providerName, fileInfo.Size(), model)

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file field
	filename := filepath.Base(req.AudioPath)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, audioFile); err != nil {
		return nil, fmt.Errorf("failed to copy audio data: %w", err)
	}

	// Add model field
	if err := writer.WriteField("model", model); err != nil {
		return nil, fmt.Errorf("failed to write model field: %w", err)
	}

	// Add optional language
	if req.Language != "" {
		if err := writer.WriteField("language", req.Language); err != nil {
			return nil, fmt.Errorf("failed to write language field: %w", err)
		}
	}

	// Add optional prompt
	if req.Prompt != "" {
		if err := writer.WriteField("prompt", req.Prompt); err != nil {
			return nil, fmt.Errorf("failed to write prompt field: %w", err)
		}
	}

	// Add response format
	if err := writer.WriteField("response_format", "verbose_json"); err != nil {
		return nil, fmt.Errorf("failed to write response_format field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create request
	httpReq, err := http.NewRequest("POST", apiURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", provider.APIKey))

	// Make request
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ [AUDIO] %s Whisper API error: %d - %s", providerName, resp.StatusCode, string(respBody))

		// Try to parse error message
		var errorResp struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		if err := json.Unmarshal(respBody, &errorResp); err == nil && errorResp.Error.Message != "" {
			return nil, fmt.Errorf("%s Whisper API error: %s", providerName, errorResp.Error.Message)
		}

		return nil, fmt.Errorf("%s Whisper API error: %d", providerName, resp.StatusCode)
	}

	// Parse response
	var apiResp struct {
		Text     string  `json:"text"`
		Language string  `json:"language"`
		Duration float64 `json:"duration"`
	}

	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	log.Printf("✅ [AUDIO] %s transcription successful (%d chars, %.1fs duration)", providerName, len(apiResp.Text), apiResp.Duration)

	return &TranscribeResponse{
		Text:     apiResp.Text,
		Language: apiResp.Language,
		Duration: apiResp.Duration,
		Provider: providerName,
	}, nil
}

// GetSupportedFormats returns the list of supported audio formats
func GetSupportedFormats() []string {
	return []string{
		"mp3", "mp4", "mpeg", "mpga", "m4a", "wav", "webm", "ogg", "flac",
	}
}

// IsSupportedFormat checks if a MIME type is supported for transcription
func IsSupportedFormat(mimeType string) bool {
	supportedTypes := map[string]bool{
		"audio/mpeg":  true,
		"audio/mp3":   true,
		"audio/mp4":   true,
		"audio/x-m4a": true,
		"audio/wav":   true,
		"audio/x-wav": true,
		"audio/wave":  true,
		"audio/webm":  true,
		"audio/ogg":   true,
		"audio/flac":  true,
	}
	return supportedTypes[mimeType]
}
