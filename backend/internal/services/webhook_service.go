package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"clara-agents/internal/database"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Webhook represents a registered webhook trigger for an agent
type Webhook struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	AgentID   string             `json:"agentId" bson:"agentId"`
	UserID    string             `json:"userId" bson:"userId"`
	Path      string             `json:"path" bson:"path"`         // unique slug, e.g. "a1b2c3d4"
	Method    string             `json:"method" bson:"method"`     // POST (default), GET, PUT
	Secret    string             `json:"secret,omitempty" bson:"secret,omitempty"` // optional HMAC-SHA256 secret
	Enabled   bool               `json:"enabled" bson:"enabled"`
	CreatedAt time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt" bson:"updatedAt"`
}

// WebhookResponse is the JSON response for webhook info
type WebhookResponse struct {
	ID         string `json:"id"`
	AgentID    string `json:"agentId"`
	Path       string `json:"path"`
	WebhookURL string `json:"webhookUrl"`
	Method     string `json:"method"`
	HasSecret  bool   `json:"hasSecret"`
	Enabled    bool   `json:"enabled"`
	CreatedAt  string `json:"createdAt"`
}

// ToResponse converts a Webhook to its API response representation
func (w *Webhook) ToResponse(baseURL string) WebhookResponse {
	return WebhookResponse{
		ID:         w.ID.Hex(),
		AgentID:    w.AgentID,
		Path:       w.Path,
		WebhookURL: baseURL + "/api/wh/" + w.Path,
		Method:     w.Method,
		HasSecret:  w.Secret != "",
		Enabled:    w.Enabled,
		CreatedAt:  w.CreatedAt.Format(time.RFC3339),
	}
}

// WebhookService manages webhook registrations in MongoDB
type WebhookService struct {
	mongoDB *database.MongoDB
}

// NewWebhookService creates a new webhook service
func NewWebhookService(mongoDB *database.MongoDB) *WebhookService {
	return &WebhookService{mongoDB: mongoDB}
}

func (s *WebhookService) collection() *mongo.Collection {
	return s.mongoDB.Database().Collection("webhooks")
}

// EnsureIndexes creates necessary indexes for the webhooks collection
func (s *WebhookService) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "path", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "agentId", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "userId", Value: 1}},
		},
	}

	_, err := s.collection().Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create webhook indexes: %w", err)
	}

	log.Println("✅ Webhook indexes created")
	return nil
}

// CreateWebhook creates a new webhook for an agent
func (s *WebhookService) CreateWebhook(ctx context.Context, agentID, userID, method string) (*Webhook, error) {
	// Check if agent already has a webhook
	existing, _ := s.GetByAgentID(ctx, agentID)
	if existing != nil {
		return existing, nil // idempotent — return existing
	}

	path, err := generateWebhookSlug()
	if err != nil {
		return nil, fmt.Errorf("failed to generate webhook path: %w", err)
	}

	if method == "" {
		method = "POST"
	}

	now := time.Now()
	webhook := &Webhook{
		ID:        primitive.NewObjectID(),
		AgentID:   agentID,
		UserID:    userID,
		Path:      path,
		Method:    method,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if _, err := s.collection().InsertOne(ctx, webhook); err != nil {
		return nil, fmt.Errorf("failed to create webhook: %w", err)
	}

	log.Printf("🔗 [WEBHOOK] Created webhook for agent %s: path=%s", agentID, path)
	return webhook, nil
}

// GetByPath retrieves a webhook by its path slug (hot path for incoming requests)
func (s *WebhookService) GetByPath(ctx context.Context, path string) (*Webhook, error) {
	var webhook Webhook
	err := s.collection().FindOne(ctx, bson.M{"path": path}).Decode(&webhook)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get webhook: %w", err)
	}
	return &webhook, nil
}

// GetByAgentID retrieves a webhook by agent ID
func (s *WebhookService) GetByAgentID(ctx context.Context, agentID string) (*Webhook, error) {
	var webhook Webhook
	err := s.collection().FindOne(ctx, bson.M{"agentId": agentID}).Decode(&webhook)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get webhook: %w", err)
	}
	return &webhook, nil
}

// DeleteByAgentID deletes the webhook for an agent
func (s *WebhookService) DeleteByAgentID(ctx context.Context, agentID string) error {
	result, err := s.collection().DeleteOne(ctx, bson.M{"agentId": agentID})
	if err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}
	if result.DeletedCount > 0 {
		log.Printf("🗑️ [WEBHOOK] Deleted webhook for agent %s", agentID)
	}
	return nil
}

// SetEnabled enables or disables a webhook
func (s *WebhookService) SetEnabled(ctx context.Context, agentID string, enabled bool) error {
	_, err := s.collection().UpdateOne(ctx,
		bson.M{"agentId": agentID},
		bson.M{"$set": bson.M{"enabled": enabled, "updatedAt": time.Now()}},
	)
	return err
}

// generateWebhookSlug generates a random 8-character hex slug
func generateWebhookSlug() (string, error) {
	bytes := make([]byte, 4) // 4 bytes = 8 hex chars
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
