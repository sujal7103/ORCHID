package services

import (
	"clara-agents/internal/database"
	"clara-agents/internal/models"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

const (
	// APIKeyPrefix is the prefix for all API keys
	APIKeyPrefix = "clv_"
	// APIKeyLength is the length of the random part of the key (32 bytes = 64 hex chars)
	APIKeyLength = 32
	// APIKeyPrefixLength is how many chars to show as prefix (including "clv_")
	APIKeyPrefixLength = 12
)

// APIKeyService manages API keys
type APIKeyService struct {
	mongoDB     *database.MongoDB
	tierService *TierService
}

// NewAPIKeyService creates a new API key service
func NewAPIKeyService(mongoDB *database.MongoDB, tierService *TierService) *APIKeyService {
	return &APIKeyService{
		mongoDB:     mongoDB,
		tierService: tierService,
	}
}

// collection returns the api_keys collection
func (s *APIKeyService) collection() *mongo.Collection {
	return s.mongoDB.Database().Collection("api_keys")
}

// GenerateKey generates a new API key
func (s *APIKeyService) GenerateKey() (string, error) {
	bytes := make([]byte, APIKeyLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return APIKeyPrefix + hex.EncodeToString(bytes), nil
}

// HashKey hashes an API key for storage
func (s *APIKeyService) HashKey(key string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash key: %w", err)
	}
	return string(hash), nil
}

// VerifyKey verifies an API key against a hash
func (s *APIKeyService) VerifyKey(key, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(key))
	return err == nil
}

// Create creates a new API key
func (s *APIKeyService) Create(ctx context.Context, userID string, req *models.CreateAPIKeyRequest) (*models.CreateAPIKeyResponse, error) {
	// Check API key limit
	if s.tierService != nil {
		count, err := s.CountByUser(ctx, userID)
		if err != nil {
			return nil, err
		}
		if !s.tierService.CheckAPIKeyLimit(ctx, userID, count) {
			limits := s.tierService.GetLimits(ctx, userID)
			return nil, fmt.Errorf("API key limit reached (%d/%d)", count, limits.MaxAPIKeys)
		}
	}

	// Validate scopes
	for _, scope := range req.Scopes {
		if !models.IsValidScope(scope) {
			return nil, fmt.Errorf("invalid scope: %s", scope)
		}
	}

	// Generate key
	key, err := s.GenerateKey()
	if err != nil {
		return nil, err
	}

	// Hash key for storage
	hash, err := s.HashKey(key)
	if err != nil {
		return nil, err
	}

	// Calculate expiration if specified
	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		exp := time.Now().Add(time.Duration(req.ExpiresIn) * 24 * time.Hour)
		expiresAt = &exp
	}

	now := time.Now()
	apiKey := &models.APIKey{
		UserID:      userID,
		KeyPrefix:   key[:APIKeyPrefixLength],
		KeyHash:     hash,
		PlainKey:    key, // TEMPORARY: Store plain key for early platform phase
		Name:        req.Name,
		Description: req.Description,
		Scopes:      req.Scopes,
		RateLimit:   req.RateLimit,
		ExpiresAt:   expiresAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	result, err := s.collection().InsertOne(ctx, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	apiKey.ID = result.InsertedID.(primitive.ObjectID)

	log.Printf("🔑 [APIKEY] Created API key %s for user %s (prefix: %s)",
		apiKey.ID.Hex(), userID, apiKey.KeyPrefix)

	return &models.CreateAPIKeyResponse{
		ID:        apiKey.ID.Hex(),
		Key:       key, // Full key - only returned once!
		KeyPrefix: apiKey.KeyPrefix,
		Name:      apiKey.Name,
		Scopes:    apiKey.Scopes,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}, nil
}

// ValidateKey validates an API key and returns the key record
func (s *APIKeyService) ValidateKey(ctx context.Context, key string) (*models.APIKey, error) {
	if len(key) < APIKeyPrefixLength {
		return nil, fmt.Errorf("invalid API key format")
	}

	// Extract prefix for lookup
	prefix := key[:APIKeyPrefixLength]

	// Find by prefix (there could be multiple with same prefix, but unlikely)
	cursor, err := s.collection().Find(ctx, bson.M{
		"keyPrefix": prefix,
		"revokedAt": bson.M{"$exists": false}, // Not revoked
	})
	if err != nil {
		return nil, fmt.Errorf("failed to lookup API key: %w", err)
	}
	defer cursor.Close(ctx)

	// Check each matching key (usually just one)
	for cursor.Next(ctx) {
		var apiKey models.APIKey
		if err := cursor.Decode(&apiKey); err != nil {
			continue
		}

		// Verify the hash
		if s.VerifyKey(key, apiKey.KeyHash) {
			// Check expiration
			if apiKey.IsExpired() {
				return nil, fmt.Errorf("API key has expired")
			}

			// Update last used
			go s.updateLastUsed(context.Background(), apiKey.ID)

			return &apiKey, nil
		}
	}

	return nil, fmt.Errorf("invalid API key")
}

// updateLastUsed updates the last used timestamp
func (s *APIKeyService) updateLastUsed(ctx context.Context, keyID primitive.ObjectID) {
	_, err := s.collection().UpdateByID(ctx, keyID, bson.M{
		"$set": bson.M{
			"lastUsedAt": time.Now(),
		},
	})
	if err != nil {
		log.Printf("⚠️ [APIKEY] Failed to update last used: %v", err)
	}
}

// ListByUser returns all API keys for a user (without hashes)
func (s *APIKeyService) ListByUser(ctx context.Context, userID string) ([]*models.APIKeyListItem, error) {
	cursor, err := s.collection().Find(ctx, bson.M{
		"userId": userID,
	}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}
	defer cursor.Close(ctx)

	var keys []*models.APIKeyListItem
	for cursor.Next(ctx) {
		var key models.APIKey
		if err := cursor.Decode(&key); err != nil {
			continue
		}
		keys = append(keys, key.ToListItem())
	}

	return keys, nil
}

// GetByID retrieves an API key by ID
func (s *APIKeyService) GetByID(ctx context.Context, keyID primitive.ObjectID) (*models.APIKey, error) {
	var key models.APIKey
	err := s.collection().FindOne(ctx, bson.M{"_id": keyID}).Decode(&key)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("API key not found")
		}
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}
	return &key, nil
}

// GetByIDAndUser retrieves an API key by ID ensuring user ownership
func (s *APIKeyService) GetByIDAndUser(ctx context.Context, keyID primitive.ObjectID, userID string) (*models.APIKey, error) {
	var key models.APIKey
	err := s.collection().FindOne(ctx, bson.M{
		"_id":    keyID,
		"userId": userID,
	}).Decode(&key)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("API key not found")
		}
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}
	return &key, nil
}

// Revoke revokes an API key (soft delete)
func (s *APIKeyService) Revoke(ctx context.Context, keyID primitive.ObjectID, userID string) error {
	result, err := s.collection().UpdateOne(ctx, bson.M{
		"_id":    keyID,
		"userId": userID,
	}, bson.M{
		"$set": bson.M{
			"revokedAt": time.Now(),
			"updatedAt": time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("API key not found")
	}

	log.Printf("🔒 [APIKEY] Revoked API key %s for user %s", keyID.Hex(), userID)
	return nil
}

// Delete permanently deletes an API key
func (s *APIKeyService) Delete(ctx context.Context, keyID primitive.ObjectID, userID string) error {
	result, err := s.collection().DeleteOne(ctx, bson.M{
		"_id":    keyID,
		"userId": userID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("API key not found")
	}

	log.Printf("🗑️ [APIKEY] Deleted API key %s for user %s", keyID.Hex(), userID)
	return nil
}

// DeleteAllByUser deletes all API keys for a user (GDPR compliance)
func (s *APIKeyService) DeleteAllByUser(ctx context.Context, userID string) (int64, error) {
	if userID == "" {
		return 0, fmt.Errorf("user ID is required")
	}

	result, err := s.collection().DeleteMany(ctx, bson.M{"userId": userID})
	if err != nil {
		return 0, fmt.Errorf("failed to delete user API keys: %w", err)
	}

	log.Printf("🗑️ [GDPR] Deleted %d API keys for user %s", result.DeletedCount, userID)
	return result.DeletedCount, nil
}

// CountByUser counts API keys for a user (non-revoked)
func (s *APIKeyService) CountByUser(ctx context.Context, userID string) (int64, error) {
	count, err := s.collection().CountDocuments(ctx, bson.M{
		"userId":    userID,
		"revokedAt": bson.M{"$exists": false},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to count API keys: %w", err)
	}
	return count, nil
}

// EnsureIndexes creates the necessary indexes for the api_keys collection
func (s *APIKeyService) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		// User ID for listing
		{
			Keys: bson.D{{Key: "userId", Value: 1}},
		},
		// Key prefix for lookup during validation
		{
			Keys: bson.D{{Key: "keyPrefix", Value: 1}},
		},
		// Compound index for revoked check
		{
			Keys: bson.D{
				{Key: "keyPrefix", Value: 1},
				{Key: "revokedAt", Value: 1},
			},
		},
	}

	_, err := s.collection().Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create API key indexes: %w", err)
	}

	log.Println("✅ [APIKEY] Ensured indexes for api_keys collection")
	return nil
}
