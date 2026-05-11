package services

import (
	"context"
	"fmt"
	"time"

	"clara-agents/internal/database"
	"clara-agents/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EngramService manages Cortex's central knowledge store (Engram)
type EngramService struct {
	collection *mongo.Collection
}

// NewEngramService creates a new engram service
func NewEngramService(mongodb *database.MongoDB) *EngramService {
	return &EngramService{
		collection: mongodb.Collection(database.CollectionNexusEngrams),
	}
}

// Write inserts a new engram entry
func (s *EngramService) Write(ctx context.Context, entry *models.EngramEntry) error {
	entry.CreatedAt = time.Now()
	if entry.ID.IsZero() {
		entry.ID = primitive.NewObjectID()
	}

	_, err := s.collection.InsertOne(ctx, entry)
	if err != nil {
		return fmt.Errorf("failed to write engram: %w", err)
	}
	return nil
}

// FindByKey returns an engram entry matching a specific key for a user, or nil if not found.
func (s *EngramService) FindByKey(ctx context.Context, userID string, key string) (*models.EngramEntry, error) {
	var entry models.EngramEntry
	err := s.collection.FindOne(ctx, bson.M{
		"userId": userID,
		"key":    key,
	}).Decode(&entry)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find engram by key: %w", err)
	}
	return &entry, nil
}

// GetRecent returns the most recent engram entries for a user
func (s *EngramService) GetRecent(ctx context.Context, userID string, limit int64) ([]models.EngramEntry, error) {
	if limit <= 0 {
		limit = 10
	}

	cursor, err := s.collection.Find(ctx, bson.M{
		"userId": userID,
	}, options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("failed to get recent engrams: %w", err)
	}
	defer cursor.Close(ctx)

	var entries []models.EngramEntry
	if err := cursor.All(ctx, &entries); err != nil {
		return nil, fmt.Errorf("failed to decode engrams: %w", err)
	}
	return entries, nil
}

// GetByType returns engram entries of a specific type
func (s *EngramService) GetByType(ctx context.Context, userID string, entryType string) ([]models.EngramEntry, error) {
	cursor, err := s.collection.Find(ctx, bson.M{
		"userId": userID,
		"type":   entryType,
	}, options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetLimit(50))
	if err != nil {
		return nil, fmt.Errorf("failed to get engrams by type: %w", err)
	}
	defer cursor.Close(ctx)

	var entries []models.EngramEntry
	if err := cursor.All(ctx, &entries); err != nil {
		return nil, fmt.Errorf("failed to decode engrams: %w", err)
	}
	return entries, nil
}

// GetBySession returns engram entries for a specific session
func (s *EngramService) GetBySession(ctx context.Context, userID string, sessionID primitive.ObjectID, limit int64) ([]models.EngramEntry, error) {
	if limit <= 0 {
		limit = 20
	}

	cursor, err := s.collection.Find(ctx, bson.M{
		"userId":    userID,
		"sessionId": sessionID,
	}, options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("failed to get session engrams: %w", err)
	}
	defer cursor.Close(ctx)

	var entries []models.EngramEntry
	if err := cursor.All(ctx, &entries); err != nil {
		return nil, fmt.Errorf("failed to decode engrams: %w", err)
	}
	return entries, nil
}

// GetDaemonStatuses returns recent daemon status entries for the active daemons view
func (s *EngramService) GetDaemonStatuses(ctx context.Context, userID string) ([]models.EngramEntry, error) {
	cursor, err := s.collection.Find(ctx, bson.M{
		"userId": userID,
		"type":   "daemon_output",
	}, options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetLimit(20))
	if err != nil {
		return nil, fmt.Errorf("failed to get daemon statuses: %w", err)
	}
	defer cursor.Close(ctx)

	var entries []models.EngramEntry
	if err := cursor.All(ctx, &entries); err != nil {
		return nil, fmt.Errorf("failed to decode daemon statuses: %w", err)
	}
	return entries, nil
}

// Search searches engram entries by key or summary containing the query
func (s *EngramService) Search(ctx context.Context, userID string, query string) ([]models.EngramEntry, error) {
	filter := bson.M{
		"userId": userID,
		"$or": []bson.M{
			{"key": bson.M{"$regex": query, "$options": "i"}},
			{"summary": bson.M{"$regex": query, "$options": "i"}},
		},
	}

	cursor, err := s.collection.Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetLimit(20))
	if err != nil {
		return nil, fmt.Errorf("failed to search engrams: %w", err)
	}
	defer cursor.Close(ctx)

	var entries []models.EngramEntry
	if err := cursor.All(ctx, &entries); err != nil {
		return nil, fmt.Errorf("failed to decode engrams: %w", err)
	}
	return entries, nil
}

// GetBySources returns engram entries matching any of the given source values for a user.
func (s *EngramService) GetBySources(ctx context.Context, userID string, sources []string, limit int64) ([]models.EngramEntry, error) {
	if limit <= 0 {
		limit = 50
	}

	cursor, err := s.collection.Find(ctx, bson.M{
		"userId": userID,
		"source": bson.M{"$in": sources},
	}, options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("failed to get engrams by sources: %w", err)
	}
	defer cursor.Close(ctx)

	var entries []models.EngramEntry
	if err := cursor.All(ctx, &entries); err != nil {
		return nil, fmt.Errorf("failed to decode engrams: %w", err)
	}
	return entries, nil
}

// CleanExpired removes expired entries (backup for TTL index)
func (s *EngramService) CleanExpired(ctx context.Context) error {
	result, err := s.collection.DeleteMany(ctx, bson.M{
		"expiresAt": bson.M{"$lt": time.Now()},
	})
	if err != nil {
		return fmt.Errorf("failed to clean expired engrams: %w", err)
	}
	if result.DeletedCount > 0 {
		fmt.Printf("Cleaned %d expired engram entries\n", result.DeletedCount)
	}
	return nil
}

// DeleteByUser removes all engram entries for a user
func (s *EngramService) DeleteByUser(ctx context.Context, userID string) error {
	_, err := s.collection.DeleteMany(ctx, bson.M{"userId": userID})
	if err != nil {
		return fmt.Errorf("failed to delete user engrams: %w", err)
	}
	return nil
}
