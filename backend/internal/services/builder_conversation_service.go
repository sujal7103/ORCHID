package services

import (
	"clara-agents/internal/crypto"
	"clara-agents/internal/database"
	"clara-agents/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// BuilderConversationService handles builder conversation operations with MongoDB
type BuilderConversationService struct {
	db         *database.MongoDB
	collection *mongo.Collection
	encryption *crypto.EncryptionService
}

// NewBuilderConversationService creates a new builder conversation service
func NewBuilderConversationService(db *database.MongoDB, encryption *crypto.EncryptionService) *BuilderConversationService {
	return &BuilderConversationService{
		db:         db,
		collection: db.Collection(database.CollectionBuilderConversations),
		encryption: encryption,
	}
}

// CreateConversation creates a new builder conversation for an agent
func (s *BuilderConversationService) CreateConversation(ctx context.Context, agentID, userID, modelID string) (*models.ConversationResponse, error) {
	now := time.Now()

	// Create encrypted conversation with string-based IDs
	// AgentID is a timestamp-based string (e.g., "1765018813035-yplenlye1")
	// UserID is a Supabase UUID string
	conv := &models.EncryptedBuilderConversation{
		AgentID:           agentID,
		UserID:            userID,
		EncryptedMessages: "", // Empty at creation
		ModelID:           modelID,
		MessageCount:      0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	result, err := s.collection.InsertOne(ctx, conv)
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	conv.ID = result.InsertedID.(primitive.ObjectID)

	return &models.ConversationResponse{
		ID:        conv.ID.Hex(),
		AgentID:   agentID,
		ModelID:   modelID,
		Messages:  []models.BuilderMessage{},
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// GetConversation retrieves a conversation by ID and decrypts messages
func (s *BuilderConversationService) GetConversation(ctx context.Context, conversationID, userID string) (*models.ConversationResponse, error) {
	convOID, err := primitive.ObjectIDFromHex(conversationID)
	if err != nil {
		return nil, fmt.Errorf("invalid conversation ID: %w", err)
	}

	var encrypted models.EncryptedBuilderConversation
	err = s.collection.FindOne(ctx, bson.M{"_id": convOID}).Decode(&encrypted)
	if err == mongo.ErrNoDocuments {
		return nil, fmt.Errorf("conversation not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	// Decrypt messages
	var messages []models.BuilderMessage
	if encrypted.EncryptedMessages != "" {
		decrypted, err := s.encryption.Decrypt(userID, encrypted.EncryptedMessages)
		if err != nil {
			log.Printf("⚠️ Failed to decrypt conversation %s: %v", conversationID, err)
			// Return empty messages on decryption failure
			messages = []models.BuilderMessage{}
		} else {
			if err := json.Unmarshal(decrypted, &messages); err != nil {
				log.Printf("⚠️ Failed to unmarshal decrypted messages: %v", err)
				messages = []models.BuilderMessage{}
			}
		}
	}

	return &models.ConversationResponse{
		ID:        conversationID,
		AgentID:   encrypted.AgentID, // AgentID is already a string
		ModelID:   encrypted.ModelID,
		Messages:  messages,
		CreatedAt: encrypted.CreatedAt,
		UpdatedAt: encrypted.UpdatedAt,
	}, nil
}

// GetConversationsByAgent retrieves all conversations for an agent
func (s *BuilderConversationService) GetConversationsByAgent(ctx context.Context, agentID, userID string) ([]models.ConversationListItem, error) {
	opts := options.Find().
		SetSort(bson.M{"updatedAt": -1}).
		SetLimit(50)

	// AgentID is a string (timestamp-based), not an ObjectID
	cursor, err := s.collection.Find(ctx, bson.M{"agentId": agentID}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list conversations: %w", err)
	}
	defer cursor.Close(ctx)

	var conversations []models.ConversationListItem
	for cursor.Next(ctx) {
		var encrypted models.EncryptedBuilderConversation
		if err := cursor.Decode(&encrypted); err != nil {
			log.Printf("⚠️ Failed to decode conversation: %v", err)
			continue
		}
		conversations = append(conversations, encrypted.ToListItem())
	}

	return conversations, nil
}

// AddMessage adds a message to a conversation
func (s *BuilderConversationService) AddMessage(ctx context.Context, conversationID, userID string, req *models.AddMessageRequest) (*models.BuilderMessage, error) {
	// Get existing conversation
	conv, err := s.GetConversation(ctx, conversationID, userID)
	if err != nil {
		return nil, err
	}

	// Create new message
	message := models.BuilderMessage{
		ID:               fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		Role:             req.Role,
		Content:          req.Content,
		Timestamp:        time.Now(),
		WorkflowSnapshot: req.WorkflowSnapshot,
	}

	// Add message to list
	messages := append(conv.Messages, message)

	// Serialize and encrypt messages
	messagesJSON, err := json.Marshal(messages)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize messages: %w", err)
	}

	encryptedMessages, err := s.encryption.Encrypt(userID, messagesJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt messages: %w", err)
	}

	// Update conversation
	convOID, _ := primitive.ObjectIDFromHex(conversationID)
	_, err = s.collection.UpdateOne(ctx,
		bson.M{"_id": convOID},
		bson.M{
			"$set": bson.M{
				"encryptedMessages": encryptedMessages,
				"messageCount":      len(messages),
				"updatedAt":         time.Now(),
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update conversation: %w", err)
	}

	return &message, nil
}

// DeleteConversation deletes a conversation
func (s *BuilderConversationService) DeleteConversation(ctx context.Context, conversationID, userID string) error {
	convOID, err := primitive.ObjectIDFromHex(conversationID)
	if err != nil {
		return fmt.Errorf("invalid conversation ID: %w", err)
	}

	result, err := s.collection.DeleteOne(ctx, bson.M{"_id": convOID})
	if err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("conversation not found")
	}

	log.Printf("✅ Deleted builder conversation %s for user %s", conversationID, userID)
	return nil
}

// DeleteConversationsByAgent deletes all conversations for an agent
func (s *BuilderConversationService) DeleteConversationsByAgent(ctx context.Context, agentID string) error {
	// AgentID is a string (timestamp-based), not an ObjectID
	result, err := s.collection.DeleteMany(ctx, bson.M{"agentId": agentID})
	if err != nil {
		return fmt.Errorf("failed to delete conversations: %w", err)
	}

	log.Printf("✅ Deleted %d builder conversations for agent %s", result.DeletedCount, agentID)
	return nil
}

// DeleteConversationsByUser deletes all conversations for a user (GDPR)
func (s *BuilderConversationService) DeleteConversationsByUser(ctx context.Context, userID string) error {
	// UserID is a Supabase UUID string, not an ObjectID
	result, err := s.collection.DeleteMany(ctx, bson.M{"userId": userID})
	if err != nil {
		return fmt.Errorf("failed to delete user conversations: %w", err)
	}

	log.Printf("✅ [GDPR] Deleted %d builder conversations for user %s", result.DeletedCount, userID)
	return nil
}

// GetOrCreateConversation gets the most recent conversation for an agent, or creates one if none exists
func (s *BuilderConversationService) GetOrCreateConversation(ctx context.Context, agentID, userID, modelID string) (*models.ConversationResponse, error) {
	// Try to find existing conversation
	// AgentID is a string (timestamp-based), not an ObjectID
	opts := options.FindOne().SetSort(bson.M{"updatedAt": -1})

	var encrypted models.EncryptedBuilderConversation
	err := s.collection.FindOne(ctx, bson.M{"agentId": agentID}, opts).Decode(&encrypted)

	if err == mongo.ErrNoDocuments {
		// Create new conversation
		return s.CreateConversation(ctx, agentID, userID, modelID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find conversation: %w", err)
	}

	// Return existing conversation
	return s.GetConversation(ctx, encrypted.ID.Hex(), userID)
}
