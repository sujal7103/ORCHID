package services

import (
	"bytes"
	"clara-agents/internal/crypto"
	"clara-agents/internal/database"
	"clara-agents/internal/models"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/leonid-shevtsov/telegold"
	"github.com/yuin/goldmark"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	// CollectionChannels is the MongoDB collection name for channels
	CollectionChannels = "channels"
	// CollectionChannelSessions is the MongoDB collection name for channel sessions
	CollectionChannelSessions = "channel_sessions"
)

// TelegramPoller handles long polling for a single bot
type TelegramPoller struct {
	channelID  primitive.ObjectID
	botToken   string
	lastOffset int64
	stopChan   chan struct{}
	running    bool
}

// ChannelService manages communication channels for users
type ChannelService struct {
	mongoDB        *database.MongoDB
	encryption     *crypto.EncryptionService
	httpClient     *http.Client
	pollingClient  *http.Client // Longer timeout for long polling
	pollers        map[string]*TelegramPoller
	pollersMux     sync.RWMutex
	messageHandler func(channel *models.Channel, session *models.ChannelSession, message *models.TelegramMessage)
}

// NewChannelService creates a new channel service
func NewChannelService(mongoDB *database.MongoDB, encryption *crypto.EncryptionService) *ChannelService {
	return &ChannelService{
		mongoDB:    mongoDB,
		encryption: encryption,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // Increased for API calls
		},
		pollingClient: &http.Client{
			Timeout: 35 * time.Second, // Long polling timeout
		},
		pollers: make(map[string]*TelegramPoller),
	}
}

// SetMessageHandler sets the callback for processing incoming messages
func (s *ChannelService) SetMessageHandler(handler func(channel *models.Channel, session *models.ChannelSession, message *models.TelegramMessage)) {
	s.messageHandler = handler
}

// collection returns the channels collection
func (s *ChannelService) collection() *mongo.Collection {
	return s.mongoDB.Database().Collection(CollectionChannels)
}

// sessionsCollection returns the channel sessions collection
func (s *ChannelService) sessionsCollection() *mongo.Collection {
	return s.mongoDB.Database().Collection(CollectionChannelSessions)
}

// generateWebhookSecret generates a secure random webhook secret
func generateWebhookSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Create creates a new channel with encrypted configuration
func (s *ChannelService) Create(ctx context.Context, userID string, req *models.CreateChannelRequest) (*models.ChannelResponse, error) {
	// Validate platform
	switch req.Platform {
	case models.ChannelPlatformTelegram:
		// Validate Telegram config
		botToken, ok := req.Config["bot_token"].(string)
		if !ok || botToken == "" {
			return nil, fmt.Errorf("bot_token is required for Telegram channel")
		}

		// Test the bot token first
		botInfo, err := s.getTelegramBotInfo(botToken)
		if err != nil {
			return nil, fmt.Errorf("invalid bot token: %w", err)
		}
		req.Config["bot_username"] = botInfo.Username
		req.Config["bot_name"] = botInfo.FirstName

	default:
		return nil, fmt.Errorf("unsupported platform: %s", req.Platform)
	}

	// Check if user already has a channel for this platform
	existing, _ := s.GetByUserAndPlatform(ctx, userID, req.Platform)
	if existing != nil {
		return nil, fmt.Errorf("you already have a %s channel configured", req.Platform)
	}

	// Serialize config to JSON
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize channel config: %w", err)
	}

	// Encrypt the config
	encryptedConfig, err := s.encryption.Encrypt(userID, configJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt channel config: %w", err)
	}

	// Generate webhook secret
	webhookSecret, err := generateWebhookSecret()
	if err != nil {
		return nil, fmt.Errorf("failed to generate webhook secret: %w", err)
	}

	// Set default name if not provided
	name := req.Name
	if name == "" {
		name = fmt.Sprintf("%s Channel", req.Platform)
	}

	// Extract bot info for storage
	botUsername, _ := req.Config["bot_username"].(string)
	botName, _ := req.Config["bot_name"].(string)

	now := time.Now()
	channel := &models.Channel{
		UserID:             userID,
		Platform:           req.Platform,
		Name:               name,
		Enabled:            true,
		EncryptedConfig:    encryptedConfig,
		WebhookSecret:      webhookSecret,
		BotUsername:        botUsername,
		BotName:            botName,
		MaxHistoryMessages: 20, // Default to 20 messages in context
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	result, err := s.collection().InsertOne(ctx, channel)
	if err != nil {
		return nil, fmt.Errorf("failed to create channel: %w", err)
	}

	channel.ID = result.InsertedID.(primitive.ObjectID)

	// Register webhook with Telegram
	if req.Platform == models.ChannelPlatformTelegram {
		botToken, _ := req.Config["bot_token"].(string)
		webhookURL := s.generateWebhookURL(channel)
		if err := s.setTelegramWebhook(botToken, webhookURL); err != nil {
			// Log warning but don't fail - user can manually set webhook
			log.Printf("⚠️ [CHANNEL] Failed to auto-register Telegram webhook: %v", err)
		} else {
			log.Printf("✅ [CHANNEL] Auto-registered Telegram webhook: %s", webhookURL)
		}
	}

	log.Printf("📡 [CHANNEL] Created %s channel %s for user %s", req.Platform, channel.ID.Hex(), userID)

	return s.toResponse(channel), nil
}

// GetByID retrieves a channel by ID
func (s *ChannelService) GetByID(ctx context.Context, channelID primitive.ObjectID) (*models.Channel, error) {
	var channel models.Channel
	err := s.collection().FindOne(ctx, bson.M{"_id": channelID}).Decode(&channel)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("channel not found")
		}
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}
	return &channel, nil
}

// GetByIDAndUser retrieves a channel ensuring user ownership
func (s *ChannelService) GetByIDAndUser(ctx context.Context, channelID primitive.ObjectID, userID string) (*models.Channel, error) {
	var channel models.Channel
	err := s.collection().FindOne(ctx, bson.M{
		"_id":    channelID,
		"userId": userID,
	}).Decode(&channel)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("channel not found")
		}
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}
	return &channel, nil
}

// GetByUserAndPlatform retrieves a user's channel for a specific platform
func (s *ChannelService) GetByUserAndPlatform(ctx context.Context, userID string, platform models.ChannelPlatform) (*models.Channel, error) {
	var channel models.Channel
	err := s.collection().FindOne(ctx, bson.M{
		"userId":   userID,
		"platform": platform,
	}).Decode(&channel)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // Not found is not an error in this case
		}
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}
	return &channel, nil
}

// GetByWebhookSecret retrieves a channel by its webhook secret (for incoming webhooks)
func (s *ChannelService) GetByWebhookSecret(ctx context.Context, secret string) (*models.Channel, error) {
	var channel models.Channel
	err := s.collection().FindOne(ctx, bson.M{
		"webhookSecret": secret,
	}).Decode(&channel)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("channel not found")
		}
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}
	return &channel, nil
}

// ListByUser lists all channels for a user
func (s *ChannelService) ListByUser(ctx context.Context, userID string) ([]*models.ChannelResponse, error) {
	cursor, err := s.collection().Find(ctx, bson.M{"userId": userID})
	if err != nil {
		return nil, fmt.Errorf("failed to list channels: %w", err)
	}
	defer cursor.Close(ctx)

	var channels []*models.Channel
	if err := cursor.All(ctx, &channels); err != nil {
		return nil, fmt.Errorf("failed to decode channels: %w", err)
	}

	responses := make([]*models.ChannelResponse, len(channels))
	for i, ch := range channels {
		responses[i] = s.toResponse(ch)
	}

	return responses, nil
}

// Update updates a channel
func (s *ChannelService) Update(ctx context.Context, channelID primitive.ObjectID, userID string, req *models.UpdateChannelRequest) (*models.ChannelResponse, error) {
	channel, err := s.GetByIDAndUser(ctx, channelID, userID)
	if err != nil {
		return nil, err
	}

	update := bson.M{"updatedAt": time.Now()}

	if req.Name != nil {
		update["name"] = *req.Name
	}
	if req.Enabled != nil {
		update["enabled"] = *req.Enabled
	}
	if req.DefaultModelID != nil {
		update["defaultModelId"] = *req.DefaultModelID
	}
	if req.DefaultSystemPrompt != nil {
		update["defaultSystemPrompt"] = *req.DefaultSystemPrompt
	}
	if req.MaxHistoryMessages != nil {
		update["maxHistoryMessages"] = *req.MaxHistoryMessages
	}
	if req.AllowedUsers != nil {
		update["allowedUsers"] = *req.AllowedUsers
	}

	_, err = s.collection().UpdateOne(ctx, bson.M{"_id": channelID}, bson.M{"$set": update})
	if err != nil {
		return nil, fmt.Errorf("failed to update channel: %w", err)
	}

	// Refresh channel data
	channel, err = s.GetByIDAndUser(ctx, channelID, userID)
	if err != nil {
		return nil, err
	}

	log.Printf("📡 [CHANNEL] Updated channel %s for user %s", channelID.Hex(), userID)

	return s.toResponse(channel), nil
}

// Delete deletes a channel and its sessions
func (s *ChannelService) Delete(ctx context.Context, channelID primitive.ObjectID, userID string) error {
	// Verify ownership and get channel
	channel, err := s.GetByIDAndUser(ctx, channelID, userID)
	if err != nil {
		return err
	}

	// Unregister webhook with Telegram
	if channel.Platform == models.ChannelPlatformTelegram {
		config, err := s.GetDecryptedConfig(ctx, channel)
		if err == nil {
			if botToken, ok := config["bot_token"].(string); ok && botToken != "" {
				if err := s.deleteTelegramWebhook(botToken); err != nil {
					log.Printf("⚠️ [CHANNEL] Failed to unregister Telegram webhook: %v", err)
				} else {
					log.Printf("✅ [CHANNEL] Unregistered Telegram webhook for channel %s", channelID.Hex())
				}
			}
		}
	}

	// Delete all sessions for this channel
	_, err = s.sessionsCollection().DeleteMany(ctx, bson.M{"channelId": channelID})
	if err != nil {
		log.Printf("⚠️ [CHANNEL] Failed to delete sessions for channel %s: %v", channelID.Hex(), err)
	}

	// Delete the channel
	_, err = s.collection().DeleteOne(ctx, bson.M{"_id": channelID})
	if err != nil {
		return fmt.Errorf("failed to delete channel: %w", err)
	}

	log.Printf("📡 [CHANNEL] Deleted channel %s for user %s", channelID.Hex(), userID)

	return nil
}

// GetDecryptedConfig retrieves and decrypts the channel configuration
func (s *ChannelService) GetDecryptedConfig(ctx context.Context, channel *models.Channel) (map[string]interface{}, error) {
	decryptedJSON, err := s.encryption.Decrypt(channel.UserID, channel.EncryptedConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt channel config: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(decryptedJSON, &config); err != nil {
		return nil, fmt.Errorf("failed to parse channel config: %w", err)
	}

	return config, nil
}

// TestChannel tests the channel configuration
func (s *ChannelService) TestChannel(ctx context.Context, channelID primitive.ObjectID, userID string) (*models.TestChannelResponse, error) {
	channel, err := s.GetByIDAndUser(ctx, channelID, userID)
	if err != nil {
		return nil, err
	}

	config, err := s.GetDecryptedConfig(ctx, channel)
	if err != nil {
		return &models.TestChannelResponse{
			Success: false,
			Message: "Failed to decrypt channel configuration",
		}, nil
	}

	switch channel.Platform {
	case models.ChannelPlatformTelegram:
		return s.testTelegramBot(ctx, channel, config)
	default:
		return &models.TestChannelResponse{
			Success: false,
			Message: "Testing not implemented for this platform",
		}, nil
	}
}

// testTelegramBot tests a Telegram bot token and updates bot info
func (s *ChannelService) testTelegramBot(ctx context.Context, channel *models.Channel, config map[string]interface{}) (*models.TestChannelResponse, error) {
	botToken, ok := config["bot_token"].(string)
	if !ok || botToken == "" {
		return &models.TestChannelResponse{
			Success: false,
			Message: "Bot token is required",
		}, nil
	}

	// Call Telegram getMe API
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", botToken)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return &models.TestChannelResponse{
			Success: false,
			Message: "Failed to connect to Telegram",
		}, nil
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if ok, _ := result["ok"].(bool); ok {
		var botUsername, botName string
		if botResult, ok := result["result"].(map[string]interface{}); ok {
			botUsername, _ = botResult["username"].(string)
			botName, _ = botResult["first_name"].(string)

			// Update channel with bot info
			s.collection().UpdateOne(ctx, bson.M{"_id": channel.ID}, bson.M{
				"$set": bson.M{
					"botUsername": botUsername,
					"botName":     botName,
					"updatedAt":   time.Now(),
				},
			})
		}

		return &models.TestChannelResponse{
			Success:     true,
			Message:     fmt.Sprintf("Bot verified: @%s (%s)", botUsername, botName),
			BotUsername: botUsername,
			BotName:     botName,
		}, nil
	}

	description, _ := result["description"].(string)
	return &models.TestChannelResponse{
		Success: false,
		Message: fmt.Sprintf("Invalid bot token: %s", description),
	}, nil
}

// getTelegramBotInfo retrieves bot information from Telegram API
func (s *ChannelService) getTelegramBotInfo(botToken string) (*struct {
	Username  string
	FirstName string
}, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", botToken)
	req, _ := http.NewRequest("GET", url, nil)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Telegram: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if ok, _ := result["ok"].(bool); !ok {
		description, _ := result["description"].(string)
		return nil, fmt.Errorf("Telegram API error: %s", description)
	}

	botResult, ok := result["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response from Telegram")
	}

	return &struct {
		Username  string
		FirstName string
	}{
		Username:  botResult["username"].(string),
		FirstName: botResult["first_name"].(string),
	}, nil
}

// setTelegramWebhook registers the webhook URL with Telegram
func (s *ChannelService) setTelegramWebhook(botToken, webhookURL string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/setWebhook", botToken)

	payload := map[string]interface{}{
		"url":             webhookURL,
		"allowed_updates": []string{"message"},
	}

	payloadBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to Telegram: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if ok, _ := result["ok"].(bool); !ok {
		description, _ := result["description"].(string)
		return fmt.Errorf("failed to set webhook: %s", description)
	}

	log.Printf("📡 [TELEGRAM] Webhook registered: %s", webhookURL)
	return nil
}

// deleteTelegramWebhook removes the webhook from Telegram
func (s *ChannelService) deleteTelegramWebhook(botToken string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/deleteWebhook", botToken)
	req, _ := http.NewRequest("POST", url, nil)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to Telegram: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if ok, _ := result["ok"].(bool); !ok {
		description, _ := result["description"].(string)
		return fmt.Errorf("failed to delete webhook: %s", description)
	}

	log.Printf("📡 [TELEGRAM] Webhook deleted")
	return nil
}

// IncrementMessageCount increments the message count for a channel
func (s *ChannelService) IncrementMessageCount(ctx context.Context, channelID primitive.ObjectID) error {
	now := time.Now()
	_, err := s.collection().UpdateOne(ctx, bson.M{"_id": channelID}, bson.M{
		"$inc": bson.M{"messageCount": 1},
		"$set": bson.M{"lastUsedAt": now, "updatedAt": now},
	})
	return err
}

// toResponse converts a Channel to a ChannelResponse (safe for frontend)
func (s *ChannelService) toResponse(channel *models.Channel) *models.ChannelResponse {
	return &models.ChannelResponse{
		ID:                  channel.ID.Hex(),
		Platform:            channel.Platform,
		Name:                channel.Name,
		Enabled:             channel.Enabled,
		WebhookURL:          s.generateWebhookURL(channel),
		BotUsername:         channel.BotUsername,
		BotName:             channel.BotName,
		DefaultModelID:      channel.DefaultModelID,
		DefaultSystemPrompt: channel.DefaultSystemPrompt,
		MaxHistoryMessages:  channel.MaxHistoryMessages,
		AllowedUsers:        channel.AllowedUsers,
		MessageCount:        channel.MessageCount,
		LastUsedAt:          channel.LastUsedAt,
		CreatedAt:           channel.CreatedAt,
	}
}

// IsUserAllowed checks if a Telegram user is allowed to use this channel
// Returns true if:
//   - allowedUsers is empty (no restrictions)
//   - userID matches an entry in allowedUsers
//   - username matches an entry in allowedUsers (case-insensitive)
func (s *ChannelService) IsUserAllowed(channel *models.Channel, userID string, username string) bool {
	// If no allowlist, allow everyone
	if len(channel.AllowedUsers) == 0 {
		return true
	}

	// Normalize username (remove @ if present)
	username = strings.TrimPrefix(strings.ToLower(username), "@")

	for _, allowed := range channel.AllowedUsers {
		// Normalize the allowed entry
		allowed = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(allowed)), "@")

		// Match by user ID or username
		if allowed == userID || allowed == username {
			return true
		}
	}

	return false
}

// generateWebhookURL generates the webhook URL for a channel
func (s *ChannelService) generateWebhookURL(channel *models.Channel) string {
	// Use environment variable for base URL, or construct from typical setup
	baseURL := getChannelBaseURL()
	return fmt.Sprintf("%s/api/channels/%s/webhook/%s", baseURL, channel.Platform, channel.WebhookSecret)
}

// getChannelBaseURL returns the base URL for channel webhooks
func getChannelBaseURL() string {
	// Check environment variable first
	if url := getEnv("API_BASE_URL", ""); url != "" {
		return url
	}
	if url := getEnv("WEBHOOK_BASE_URL", ""); url != "" {
		return url
	}
	// Default for development
	return "http://localhost:3001"
}

// ============================================================================
// Session Management
// ============================================================================

// GetOrCreateSession gets or creates a session for a channel user
func (s *ChannelService) GetOrCreateSession(ctx context.Context, channelID primitive.ObjectID, platformUserID, platformChatID string) (*models.ChannelSession, error) {
	var session models.ChannelSession
	err := s.sessionsCollection().FindOne(ctx, bson.M{
		"channelId":      channelID,
		"platformUserId": platformUserID,
		"platformChatId": platformChatID,
	}).Decode(&session)

	if err == nil {
		return &session, nil
	}

	if err != mongo.ErrNoDocuments {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Create new session
	now := time.Now()
	session = models.ChannelSession{
		ChannelID:      channelID,
		PlatformUserID: platformUserID,
		PlatformChatID: platformChatID,
		ConversationID: primitive.NewObjectID().Hex(), // Generate new conversation ID
		Messages:       []models.ChannelMessage{},
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	result, err := s.sessionsCollection().InsertOne(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	session.ID = result.InsertedID.(primitive.ObjectID)
	log.Printf("📡 [CHANNEL-SESSION] Created session %s for channel %s", session.ID.Hex(), channelID.Hex())

	return &session, nil
}

// AddMessageToSession adds a message to a session's history
func (s *ChannelService) AddMessageToSession(ctx context.Context, sessionID primitive.ObjectID, role, content string, maxMessages int) error {
	msg := models.ChannelMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}

	// Add message and trim to max size
	_, err := s.sessionsCollection().UpdateOne(ctx, bson.M{"_id": sessionID}, bson.M{
		"$push": bson.M{
			"messages": bson.M{
				"$each":     []models.ChannelMessage{msg},
				"$slice":    -maxMessages, // Keep only the last N messages
				"$position": 0,
			},
		},
		"$set": bson.M{"updatedAt": time.Now()},
	})

	// Actually, we want to append and then slice from the end
	// Let's do it differently - push then slice
	_, err = s.sessionsCollection().UpdateOne(ctx, bson.M{"_id": sessionID}, bson.M{
		"$push": bson.M{
			"messages": msg,
		},
		"$set": bson.M{"updatedAt": time.Now()},
	})
	if err != nil {
		return err
	}

	// Trim to max messages if needed
	var session models.ChannelSession
	err = s.sessionsCollection().FindOne(ctx, bson.M{"_id": sessionID}).Decode(&session)
	if err != nil {
		return err
	}

	if len(session.Messages) > maxMessages {
		// Keep only the last maxMessages
		trimmedMessages := session.Messages[len(session.Messages)-maxMessages:]
		_, err = s.sessionsCollection().UpdateOne(ctx, bson.M{"_id": sessionID}, bson.M{
			"$set": bson.M{"messages": trimmedMessages},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// GetSessionHistory gets the message history for a session
func (s *ChannelService) GetSessionHistory(ctx context.Context, sessionID primitive.ObjectID) ([]models.ChannelMessage, error) {
	var session models.ChannelSession
	err := s.sessionsCollection().FindOne(ctx, bson.M{"_id": sessionID}).Decode(&session)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	return session.Messages, nil
}

// ClearSession clears a session's message history (for /new command)
func (s *ChannelService) ClearSession(ctx context.Context, sessionID primitive.ObjectID) error {
	_, err := s.sessionsCollection().UpdateOne(ctx, bson.M{"_id": sessionID}, bson.M{
		"$set": bson.M{
			"messages":       []models.ChannelMessage{},
			"conversationId": primitive.NewObjectID().Hex(), // New conversation
			"updatedAt":      time.Now(),
		},
	})
	return err
}

// UpdateSessionModel updates the model for a session
func (s *ChannelService) UpdateSessionModel(ctx context.Context, sessionID primitive.ObjectID, modelID string) error {
	_, err := s.sessionsCollection().UpdateOne(ctx, bson.M{"_id": sessionID}, bson.M{
		"$set": bson.M{
			"modelId":   modelID,
			"updatedAt": time.Now(),
		},
	})
	return err
}

// ============================================================================
// Telegram API Helpers
// ============================================================================

// Telegram Markdown converter using telegold (goldmark with Telegram HTML renderer)
var telegramMarkdownConverter = goldmark.New(goldmark.WithRenderer(telegold.NewRenderer()))

// convertToTelegramHTML converts standard Markdown to Telegram-compatible HTML
// Uses the telegold library for reliable conversion
func convertToTelegramHTML(text string) string {
	var buf bytes.Buffer
	if err := telegramMarkdownConverter.Convert([]byte(text), &buf); err != nil {
		// If conversion fails, return original text
		log.Printf("⚠️ [TELEGRAM] Markdown conversion failed: %v", err)
		return text
	}
	return buf.String()
}

// SendTelegramMessage sends a message via Telegram Bot API
// Uses HTML format (more reliable than MarkdownV2), falls back to plain text if parsing fails
func (s *ChannelService) SendTelegramMessage(ctx context.Context, botToken string, chatID int64, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	// Convert standard Markdown to Telegram HTML using telegold
	telegramHTML := convertToTelegramHTML(text)

	// Try with HTML first (more reliable than MarkdownV2)
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       telegramHTML,
		"parse_mode": "HTML",
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return nil
	}

	// Check if it's an HTML parsing error
	bodyBytes, _ := io.ReadAll(resp.Body)
	errStr := string(bodyBytes)

	if strings.Contains(errStr, "can't parse entities") {
		// Retry with plain text
		log.Printf("⚠️ [TELEGRAM] HTML parsing failed, retrying without parse_mode")

		payload = map[string]interface{}{
			"chat_id": chatID,
			"text":    stripMarkdown(text),
		}
		body, _ = json.Marshal(payload)

		req, _ = http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		resp2, err := s.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send Telegram message (plain): %w", err)
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != 200 {
			bodyBytes2, _ := io.ReadAll(resp2.Body)
			return fmt.Errorf("Telegram API error (plain): %s", string(bodyBytes2))
		}
		return nil
	}

	return fmt.Errorf("Telegram API error: %s", errStr)
}

// stripMarkdown removes Markdown formatting for plain text fallback
func stripMarkdown(text string) string {
	// Remove bold markers
	text = strings.ReplaceAll(text, "**", "")
	text = strings.ReplaceAll(text, "__", "")
	// Remove code blocks - keep content
	codeBlockPattern := regexp.MustCompile("```[a-zA-Z]*\\n([\\s\\S]*?)```")
	text = codeBlockPattern.ReplaceAllString(text, "$1")
	// Remove inline code backticks
	text = strings.ReplaceAll(text, "`", "")
	// Remove strikethrough
	text = strings.ReplaceAll(text, "~~", "")
	// Convert headers to plain text
	headerPattern := regexp.MustCompile(`(?m)^#{1,6}\s+`)
	text = headerPattern.ReplaceAllString(text, "")
	// Convert links [text](url) to "text (url)"
	linkPattern := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	text = linkPattern.ReplaceAllString(text, "$1 ($2)")
	return text
}

// SendTelegramMessageChunked sends a long message by splitting it into chunks
// Telegram has a 4096 character limit per message
func (s *ChannelService) SendTelegramMessageChunked(ctx context.Context, botToken string, chatID int64, text string) error {
	const maxChunkSize = 4000 // Leave some margin for safety

	// If message is short enough, send directly
	if len(text) <= maxChunkSize {
		return s.SendTelegramMessage(ctx, botToken, chatID, text)
	}

	// Split into chunks
	chunks := splitMessageIntoChunks(text, maxChunkSize)
	totalChunks := len(chunks)

	log.Printf("📨 [TELEGRAM] Splitting message (%d chars) into %d chunks", len(text), totalChunks)

	for i, chunk := range chunks {
		// Add part indicator for multi-part messages (using Markdown that will be converted to HTML)
		if totalChunks > 1 {
			chunk = fmt.Sprintf("**[Part %d/%d]**\n\n%s", i+1, totalChunks, chunk)
		}

		if err := s.SendTelegramMessage(ctx, botToken, chatID, chunk); err != nil {
			return fmt.Errorf("failed to send chunk %d/%d: %w", i+1, totalChunks, err)
		}

		// Small delay between chunks to avoid rate limiting
		if i < totalChunks-1 {
			time.Sleep(300 * time.Millisecond)
		}
	}

	return nil
}

// splitMessageIntoChunks splits a message into chunks respecting boundaries
func splitMessageIntoChunks(text string, maxSize int) []string {
	if len(text) <= maxSize {
		return []string{text}
	}

	var chunks []string
	remaining := text

	for len(remaining) > 0 {
		if len(remaining) <= maxSize {
			chunks = append(chunks, remaining)
			break
		}

		// Find a good break point
		chunk := remaining[:maxSize]
		breakPoint := maxSize

		// Try to break at code block boundaries first (```)
		if idx := strings.LastIndex(chunk, "\n```"); idx > maxSize/2 {
			breakPoint = idx + 1 // Include the newline
		} else if idx := strings.LastIndex(chunk, "```\n"); idx > maxSize/2 {
			breakPoint = idx + 4 // Include ``` and newline
		} else if idx := strings.LastIndex(chunk, "\n\n"); idx > maxSize/2 {
			// Try to break at paragraph boundaries
			breakPoint = idx + 2
		} else if idx := strings.LastIndex(chunk, "\n"); idx > maxSize/2 {
			// Try to break at line boundaries
			breakPoint = idx + 1
		} else if idx := strings.LastIndex(chunk, ". "); idx > maxSize/2 {
			// Try to break at sentence boundaries
			breakPoint = idx + 2
		} else if idx := strings.LastIndex(chunk, " "); idx > maxSize/2 {
			// Try to break at word boundaries
			breakPoint = idx + 1
		}
		// If no good break point found, just break at maxSize

		chunks = append(chunks, strings.TrimSpace(remaining[:breakPoint]))
		remaining = strings.TrimSpace(remaining[breakPoint:])
	}

	return chunks
}

// SendTelegramTypingAction sends a "typing" action to indicate the bot is working
func (s *ChannelService) SendTelegramTypingAction(ctx context.Context, botToken string, chatID int64) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendChatAction", botToken)

	payload := map[string]interface{}{
		"chat_id": chatID,
		"action":  "typing",
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// SendContinuousTypingAction sends typing indicator every 4 seconds until context is cancelled
// Telegram's typing indicator only lasts ~5 seconds, so we refresh it continuously
func (s *ChannelService) SendContinuousTypingAction(ctx context.Context, botToken string, chatID int64) {
	// Send immediately
	s.SendTelegramTypingAction(ctx, botToken, chatID)

	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.SendTelegramTypingAction(ctx, botToken, chatID); err != nil {
				log.Printf("⚠️ [TELEGRAM] Failed to send typing action: %v", err)
				return
			}
		}
	}
}

// ============================================================================
// Telegram Media Handling
// ============================================================================

// GetTelegramFileURL gets the download URL for a Telegram file
func (s *ChannelService) GetTelegramFileURL(ctx context.Context, botToken, fileID string) (string, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getFile?file_id=%s", botToken, fileID)

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			FilePath string `json:"file_path"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.OK || result.Result.FilePath == "" {
		return "", fmt.Errorf("file not found")
	}

	// Construct download URL
	downloadURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", botToken, result.Result.FilePath)
	return downloadURL, nil
}

// DownloadTelegramFile downloads a file from Telegram and returns the bytes
func (s *ChannelService) DownloadTelegramFile(ctx context.Context, botToken, fileID string) ([]byte, string, error) {
	// Use a fresh context with longer timeout for file downloads
	downloadCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fileURL, err := s.GetTelegramFileURL(downloadCtx, botToken, fileID)
	if err != nil {
		return nil, "", err
	}

	// Create a client with no timeout (we control via context)
	downloadClient := &http.Client{
		Timeout: 60 * time.Second,
	}

	req, _ := http.NewRequestWithContext(downloadCtx, "GET", fileURL, nil)
	resp, err := downloadClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file: %w", err)
	}

	// Extract filename from URL
	parts := strings.Split(fileURL, "/")
	filename := parts[len(parts)-1]

	log.Printf("📥 [TELEGRAM] Downloaded file: %s (%d bytes)", filename, len(data))
	return data, filename, nil
}

// SendTelegramPhoto sends a photo to a Telegram chat
func (s *ChannelService) SendTelegramPhoto(ctx context.Context, botToken string, chatID int64, photoData []byte, caption string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendPhoto", botToken)

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add chat_id
	writer.WriteField("chat_id", fmt.Sprintf("%d", chatID))

	// Add caption if provided
	if caption != "" {
		// Truncate caption to 1024 chars (Telegram limit for photo captions)
		if len(caption) > 1024 {
			caption = caption[:1021] + "..."
		}
		writer.WriteField("caption", caption)
		writer.WriteField("parse_mode", "HTML")
	}

	// Add photo
	part, err := writer.CreateFormFile("photo", "image.png")
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	part.Write(photoData)
	writer.Close()

	req, _ := http.NewRequestWithContext(ctx, "POST", url, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send photo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Telegram API error: %s", string(body))
	}

	log.Printf("📸 [TELEGRAM] Sent photo to chat %d", chatID)
	return nil
}

// SendTelegramPhotoByURL sends a photo by URL to a Telegram chat
func (s *ChannelService) SendTelegramPhotoByURL(ctx context.Context, botToken string, chatID int64, photoURL, caption string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendPhoto", botToken)

	payload := map[string]interface{}{
		"chat_id": chatID,
		"photo":   photoURL,
	}

	if caption != "" {
		// Truncate caption to 1024 chars (Telegram limit)
		if len(caption) > 1024 {
			caption = caption[:1021] + "..."
		}
		payload["caption"] = caption
		payload["parse_mode"] = "HTML"
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send photo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Telegram API error: %s", string(respBody))
	}

	log.Printf("📸 [TELEGRAM] Sent photo (URL) to chat %d", chatID)
	return nil
}

// SendTelegramVoice sends a voice message to a Telegram chat
func (s *ChannelService) SendTelegramVoice(ctx context.Context, botToken string, chatID int64, audioData []byte, caption string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendVoice", botToken)

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add chat_id
	writer.WriteField("chat_id", fmt.Sprintf("%d", chatID))

	// Add caption if provided
	if caption != "" {
		if len(caption) > 1024 {
			caption = caption[:1021] + "..."
		}
		writer.WriteField("caption", caption)
	}

	// Add voice file (must be OGG with OPUS codec for Telegram)
	part, err := writer.CreateFormFile("voice", "voice.ogg")
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	part.Write(audioData)
	writer.Close()

	req, _ := http.NewRequestWithContext(ctx, "POST", url, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send voice: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Telegram API error: %s", string(body))
	}

	log.Printf("🎤 [TELEGRAM] Sent voice to chat %d", chatID)
	return nil
}

// SendTelegramDocument sends a document/file to a Telegram chat
func (s *ChannelService) SendTelegramDocument(ctx context.Context, botToken string, chatID int64, fileData []byte, filename, caption string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendDocument", botToken)

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add chat_id
	writer.WriteField("chat_id", fmt.Sprintf("%d", chatID))

	// Add caption if provided
	if caption != "" {
		if len(caption) > 1024 {
			caption = caption[:1021] + "..."
		}
		writer.WriteField("caption", caption)
		writer.WriteField("parse_mode", "HTML")
	}

	// Add document
	part, err := writer.CreateFormFile("document", filename)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	part.Write(fileData)
	writer.Close()

	req, _ := http.NewRequestWithContext(ctx, "POST", url, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Telegram API error: %s", string(body))
	}

	log.Printf("📄 [TELEGRAM] Sent document '%s' to chat %d", filename, chatID)
	return nil
}

// getEnv helper
func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// ============================================================================
// Long Polling Support (for localhost development)
// ============================================================================

// IsLocalhost checks if the webhook URL is localhost (can't receive webhooks)
func (s *ChannelService) IsLocalhost() bool {
	baseURL := getChannelBaseURL()
	return strings.Contains(baseURL, "localhost") || strings.Contains(baseURL, "127.0.0.1")
}

// StartPolling starts long polling for all enabled Telegram channels
func (s *ChannelService) StartPolling(ctx context.Context) {
	if !s.IsLocalhost() {
		log.Println("📡 [CHANNEL] Webhook mode enabled (public URL detected)")
		return
	}

	log.Println("📡 [CHANNEL] Long polling mode enabled (localhost detected)")

	// Find all enabled Telegram channels
	cursor, err := s.collection().Find(ctx, bson.M{
		"platform": models.ChannelPlatformTelegram,
		"enabled":  true,
	})
	if err != nil {
		log.Printf("⚠️ [CHANNEL] Failed to find channels for polling: %v", err)
		return
	}
	defer cursor.Close(ctx)

	var channels []models.Channel
	if err := cursor.All(ctx, &channels); err != nil {
		log.Printf("⚠️ [CHANNEL] Failed to decode channels: %v", err)
		return
	}

	for _, channel := range channels {
		config, err := s.GetDecryptedConfig(ctx, &channel)
		if err != nil {
			log.Printf("⚠️ [CHANNEL] Failed to decrypt config for channel %s: %v", channel.ID.Hex(), err)
			continue
		}

		botToken, _ := config["bot_token"].(string)
		if botToken != "" {
			s.StartPollerForChannel(channel.ID, botToken)
		}
	}
}

// StartPollerForChannel starts a long polling goroutine for a specific channel
func (s *ChannelService) StartPollerForChannel(channelID primitive.ObjectID, botToken string) {
	s.pollersMux.Lock()
	defer s.pollersMux.Unlock()

	key := channelID.Hex()
	if existing, ok := s.pollers[key]; ok && existing.running {
		log.Printf("📡 [POLLING] Poller already running for channel %s", key)
		return
	}

	// Delete any existing webhook first
	s.deleteTelegramWebhook(botToken)

	poller := &TelegramPoller{
		channelID:  channelID,
		botToken:   botToken,
		lastOffset: 0,
		stopChan:   make(chan struct{}),
		running:    true,
	}
	s.pollers[key] = poller

	go s.runPoller(poller)
	log.Printf("📡 [POLLING] Started poller for channel %s", key)
}

// StopPollerForChannel stops the long polling goroutine for a channel
func (s *ChannelService) StopPollerForChannel(channelID primitive.ObjectID) {
	s.pollersMux.Lock()
	defer s.pollersMux.Unlock()

	key := channelID.Hex()
	if poller, ok := s.pollers[key]; ok && poller.running {
		close(poller.stopChan)
		poller.running = false
		delete(s.pollers, key)
		log.Printf("📡 [POLLING] Stopped poller for channel %s", key)
	}
}

// StopAllPollers stops all running pollers
func (s *ChannelService) StopAllPollers() {
	s.pollersMux.Lock()
	defer s.pollersMux.Unlock()

	for key, poller := range s.pollers {
		if poller.running {
			close(poller.stopChan)
			poller.running = false
		}
		delete(s.pollers, key)
	}
	log.Println("📡 [POLLING] All pollers stopped")
}

// runPoller runs the long polling loop for a single channel
func (s *ChannelService) runPoller(poller *TelegramPoller) {
	log.Printf("📡 [POLLING] Polling loop started for channel %s", poller.channelID.Hex())

	for {
		select {
		case <-poller.stopChan:
			log.Printf("📡 [POLLING] Poller stopped for channel %s", poller.channelID.Hex())
			return
		default:
			updates, err := s.getTelegramUpdates(poller.botToken, poller.lastOffset)
			if err != nil {
				log.Printf("⚠️ [POLLING] Error getting updates: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			for _, update := range updates {
				// Update offset to acknowledge this update
				if update.UpdateID >= poller.lastOffset {
					poller.lastOffset = update.UpdateID + 1
				}

				// Process the message
				if update.Message != nil && s.messageHandler != nil {
					ctx := context.Background()

					// Get the channel
					channel, err := s.GetByID(ctx, poller.channelID)
					if err != nil {
						log.Printf("⚠️ [POLLING] Channel not found: %v", err)
						continue
					}

					// Get or create session
					platformUserID := fmt.Sprintf("%d", update.Message.From.ID)
					platformChatID := fmt.Sprintf("%d", update.Message.Chat.ID)
					session, err := s.GetOrCreateSession(ctx, poller.channelID, platformUserID, platformChatID)
					if err != nil {
						log.Printf("⚠️ [POLLING] Failed to get session: %v", err)
						continue
					}

					// Call the message handler
					s.messageHandler(channel, session, update.Message)
				}
			}
		}
	}
}

// getTelegramUpdates fetches updates using long polling
func (s *ChannelService) getTelegramUpdates(botToken string, offset int64) ([]*models.TelegramUpdate, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?timeout=30&allowed_updates=[\"message\"]", botToken)
	if offset > 0 {
		url += fmt.Sprintf("&offset=%d", offset)
	}

	req, _ := http.NewRequest("GET", url, nil)

	resp, err := s.pollingClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get updates: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool                     `json:"ok"`
		Result []*models.TelegramUpdate `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.OK {
		return nil, fmt.Errorf("Telegram API returned not OK")
	}

	return result.Result, nil
}
