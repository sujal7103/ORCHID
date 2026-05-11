package tools

import (
	"clara-agents/internal/audio"
	"clara-agents/internal/filecache"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// NewTranscribeAudioTool creates the transcribe_audio tool for speech-to-text
func NewTranscribeAudioTool() *Tool {
	return &Tool{
		Name:        "transcribe_audio",
		DisplayName: "Transcribe Audio",
		Description: "Transcribes speech from audio files to text using OpenAI Whisper. Supports MP3, WAV, M4A, OGG, FLAC, and WebM formats. Can translate non-English audio to English.",
		Icon:        "Mic",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_id": map[string]interface{}{
					"type":        "string",
					"description": "The file ID of the uploaded audio file (from the upload response)",
				},
				"language": map[string]interface{}{
					"type":        "string",
					"description": "Optional language code (e.g., 'en' for English, 'es' for Spanish, 'fr' for French). Auto-detected if not specified.",
				},
				"prompt": map[string]interface{}{
					"type":        "string",
					"description": "Optional prompt to guide the transcription style or provide context (e.g., technical terms, names)",
				},
				"translate_to_english": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, translates the audio to English regardless of the source language. Useful for non-English audio files.",
					"default":     false,
				},
			},
			"required": []string{"file_id"},
		},
		Execute:  executeTranscribeAudio,
		Source:   ToolSourceBuiltin,
		Category: "data_sources",
		Keywords: []string{"audio", "transcribe", "speech", "voice", "whisper", "mp3", "wav", "speech-to-text", "stt", "translate"},
	}
}

func executeTranscribeAudio(args map[string]interface{}) (string, error) {
	// Extract file_id parameter
	fileID, ok := args["file_id"].(string)
	if !ok || fileID == "" {
		return "", fmt.Errorf("file_id parameter is required and must be a string")
	}

	// Extract optional language parameter
	language := ""
	if l, ok := args["language"].(string); ok {
		language = l
	}

	// Extract optional prompt parameter
	prompt := ""
	if p, ok := args["prompt"].(string); ok {
		prompt = p
	}

	// Extract translate_to_english parameter
	translateToEnglish := false
	if t, ok := args["translate_to_english"].(bool); ok {
		translateToEnglish = t
	}

	// Extract user context (injected by tool executor)
	userID, _ := args["__user_id__"].(string)

	// Clean up internal parameters
	delete(args, "__user_id__")
	delete(args, "__conversation_id__")

	action := "Transcribing"
	if translateToEnglish {
		action = "Translating to English"
	}
	log.Printf("🎵 [TRANSCRIBE-AUDIO] %s audio file_id=%s language=%s (user=%s)", action, fileID, language, userID)

	// Get file cache service to validate file exists and user has access
	fileCacheService := filecache.GetService()

	var file *filecache.CachedFile
	if userID != "" {
		var err error
		file, err = fileCacheService.GetByUser(fileID, userID)
		if err != nil {
			// Try without user validation for workflow context
			file, _ = fileCacheService.Get(fileID)
			if file != nil && file.UserID != userID {
				log.Printf("🚫 [TRANSCRIBE-AUDIO] Access denied: file %s belongs to different user", fileID)
				return "", fmt.Errorf("access denied: you don't have permission to access this file")
			}
		}
	} else {
		file, _ = fileCacheService.Get(fileID)
	}

	if file == nil {
		log.Printf("❌ [TRANSCRIBE-AUDIO] File not found: %s", fileID)
		return "", fmt.Errorf("audio file not found or has expired. Files are only available for 30 minutes after upload")
	}

	// Validate it's an audio file
	if !strings.HasPrefix(file.MimeType, "audio/") {
		log.Printf("⚠️ [TRANSCRIBE-AUDIO] File is not audio: %s (%s)", fileID, file.MimeType)
		return "", fmt.Errorf("file is not an audio file (type: %s). Supported formats: MP3, WAV, M4A, OGG, FLAC, WebM", file.MimeType)
	}

	// Check if format is supported
	if !audio.IsSupportedFormat(file.MimeType) {
		return "", fmt.Errorf("audio format not supported: %s. Supported formats: %s", file.MimeType, strings.Join(audio.GetSupportedFormats(), ", "))
	}

	// Verify file path exists
	if file.FilePath == "" {
		return "", fmt.Errorf("audio file path not available")
	}

	// Get the audio service
	audioService := audio.GetService()
	if audioService == nil {
		return "", fmt.Errorf("audio service not available. Please configure your OpenAI API key")
	}

	// Build the request
	req := &audio.TranscribeRequest{
		AudioPath:          file.FilePath,
		Language:           language,
		Prompt:             prompt,
		TranslateToEnglish: translateToEnglish,
	}

	// Call audio service
	result, err := audioService.Transcribe(req)
	if err != nil {
		log.Printf("❌ [TRANSCRIBE-AUDIO] Transcription failed: %v", err)
		return "", fmt.Errorf("failed to transcribe audio: %v", err)
	}

	// Build response
	response := map[string]interface{}{
		"success":  true,
		"file_id":  fileID,
		"filename": file.Filename,
		"text":     result.Text,
	}

	if result.Language != "" {
		response["detected_language"] = result.Language
	}
	if result.Duration > 0 {
		response["duration_seconds"] = result.Duration
	}
	if language != "" {
		response["requested_language"] = language
	}
	if translateToEnglish {
		response["translated_to_english"] = true
	}

	// Add word count
	words := strings.Fields(result.Text)
	response["word_count"] = len(words)

	responseJSON, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	log.Printf("✅ [TRANSCRIBE-AUDIO] Successfully transcribed %s (%d words, %.1fs)", file.Filename, len(words), result.Duration)

	return string(responseJSON), nil
}
